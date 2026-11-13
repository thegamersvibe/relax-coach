package main

import (
	"context"
	"reflect"
	"testing"
)

// The campfire media set is pinned per bundle version: 1.5.6+ requires the two
// songs and refuses the old single song; older bundles keep the single song.
func campfireApply(t *testing.T, version string, entries []addonZipEntry) (AddonUpdateResult, error) {
	t.Helper()
	game := guildGame(t)
	before := addonTreeHashes(t, game)
	u := guildUpdaterForVersion(t, funModeZip(t, entries, version), version)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil && !reflect.DeepEqual(before, addonTreeHashes(t, game)) {
		t.Fatal("refused bundle changed the game folder")
	}
	return result, err
}

func campfireEdit(entries []addonZipEntry, drop string, add ...string) []addonZipEntry {
	out := []addonZipEntry{}
	for _, e := range entries {
		if drop == "" || e.name != guildFixtureRoots[0]+"/"+drop {
			out = append(out, e)
		}
	}
	for _, name := range add {
		out = append(out, addonZipEntry{guildFixtureRoots[0] + "/" + name, "fixture " + name, 0644})
	}
	return out
}

func TestCampfireSongsArePinnedPerBundleVersion(t *testing.T) {
	for _, version := range []string{"1.5.6", "1.5.7", "1.6.0", "2.0.0"} {
		t.Run("ok/"+version, func(t *testing.T) {
			result, err := campfireApply(t, version, funModeEntries(version))
			if err != nil || result.Version != version {
				t.Fatalf("%s with two songs: %+v %v", version, result, err)
			}
		})
	}
	for _, version := range []string{"1.4.3", "1.5.5"} {
		t.Run("ok/"+version, func(t *testing.T) {
			result, err := campfireApply(t, version, funModeEntries(version))
			if err != nil || result.Version != version {
				t.Fatalf("%s with one song: %+v %v", version, result, err)
			}
		})
	}
	cases := []struct {
		name, version, drop string
		add                 []string
	}{
		{"1.5.6 with the old song instead", "1.5.6", "Media/Reactions/campfire_1.mp3", []string{"Media/Reactions/campfire.mp3"}},
		{"1.5.6 with the old song as well", "1.5.6", "", []string{"Media/Reactions/campfire.mp3"}},
		{"1.5.6 without song 2", "1.5.6", "Media/Reactions/campfire_2.mp3", nil},
		{"1.5.6 without song 1", "1.5.6", "Media/Reactions/campfire_1.mp3", nil},
		{"1.5.6 with a third song", "1.5.6", "", []string{"Media/Reactions/campfire_3.mp3"}},
		{"1.5.5 with a new song as well", "1.5.5", "", []string{"Media/Reactions/campfire_1.mp3"}},
		{"1.5.5 with the new songs instead", "1.5.5", "Media/Reactions/campfire.mp3", []string{"Media/Reactions/campfire_1.mp3", "Media/Reactions/campfire_2.mp3"}},
	}
	for _, tc := range cases {
		t.Run("refused/"+tc.name, func(t *testing.T) {
			result, err := campfireApply(t, tc.version, campfireEdit(funModeEntries(tc.version), tc.drop, tc.add...))
			if err == nil || result.Version != "" {
				t.Fatalf("expected a refusal, got %+v %v", result, err)
			}
		})
	}
}
