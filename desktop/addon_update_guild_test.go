package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func guildTestRelease(version string) map[string]any {
	r := addonTestRelease("v" + version)
	name := addonAssetName(version)
	r["assets"].([]map[string]any)[0] = map[string]any{"name": name, "browser_download_url": addonReleasePrefix + "download/v" + version + "/" + name}
	return r
}

func TestGuildReleaseSelection(t *testing.T) {
	for _, tc := range []struct {
		name, version, current, reason string
		available                      bool
		mutate                         func(map[string]any)
	}{
		{name: "guild required at floor", version: "1.4.2", current: "1.4.1", available: true},
		{name: "future guild", version: "1.4.9", current: "1.4.1", available: true},
		{name: "planner bundle", version: "1.5.0", current: "1.4.5", available: true},
		{name: "planner bundle under the old guild name is not offered", version: "1.5.0", current: "1.4.5", reason: "RelaxCoach-Bundle-1.5.0.zip", mutate: func(r map[string]any) {
			r["assets"].([]map[string]any)[0] = map[string]any{"name": "RelaxCoach-Guild-1.5.0.zip", "browser_download_url": addonReleasePrefix + "download/v1.5.0/RelaxCoach-Guild-1.5.0.zip"}
		}},
		{name: "coach only forbidden", version: "1.4.2", current: "1.4.1", reason: "RelaxCoach-Guild-1.4.2.zip", mutate: func(r map[string]any) { r["assets"] = addonTestRelease("v1.4.2")["assets"] }},
		{name: "app only", version: "1.4.2", current: "1.4.1", reason: "app-only", mutate: func(r map[string]any) { r["assets"] = appTestRelease("v1.4.2")["assets"] }},
		{name: "current tag app only", version: "1.4.2", current: "1.4.2", reason: "app-only", mutate: func(r map[string]any) { r["assets"] = appTestRelease("v1.4.2")["assets"] }},
		{name: "missing checksum", version: "1.4.2", current: "1.4.1", reason: "SHA256SUMS.txt", mutate: func(r map[string]any) { r["assets"] = r["assets"].([]map[string]any)[:1] }},
		{name: "current", version: "1.4.2", current: "1.4.2", reason: "not be downgraded"},
		{name: "newer installed", version: "1.4.2", current: "1.5.0", reason: "not be downgraded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := addonTestUpdater(t, func(w http.ResponseWriter, r *http.Request) {
				body := guildTestRelease(tc.version)
				if tc.mutate != nil {
					tc.mutate(body)
				}
				json.NewEncoder(w).Encode(body)
			})
			got, err := u.check(context.Background(), tc.current)
			if err != nil {
				t.Fatal(err)
			}
			if got.Available != tc.available {
				t.Fatalf("availability: %+v", got)
			}
			if tc.available {
				if got.AssetName != addonAssetName(tc.version) || got.Version != tc.version {
					t.Fatalf("wrong package: %+v", got)
				}
				if err := validateAddonRelease(got); err != nil {
					t.Fatal(err)
				}
			} else if got.Version != "" || got.AssetName != "" || !strings.Contains(got.Notes, tc.reason) {
				t.Fatalf("misleading unavailable offer: %+v", got)
			}
		})
	}
	legacy := AddonReleaseInfo{Version: "1.4.2", Available: true, AssetName: "RelaxCoach-1.4.2.zip", AssetURL: addonReleasePrefix + "download/v1.4.2/RelaxCoach-1.4.2.zip", ChecksumURL: addonReleasePrefix + "download/v1.4.2/SHA256SUMS.txt"}
	if validateAddonRelease(legacy) == nil {
		t.Fatal("direct apply accepted Coach-only new release")
	}
}
