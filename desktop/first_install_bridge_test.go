package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestFirstInstallBridgeLabelsAndState(t *testing.T) {
	for _, existing := range []bool{false, true} {
		game := freshClient(t)
		if existing {
			game = guildGame(t)
			// An existing Relax install: Journey is Relax Journey (see journey_replaced_test.go).
			addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: Relax Journey\n")
		}
		a := &App{}
		a.setGame(game)
		b := newNativeBridge(a, false, nativeServices{check: func(_ context.Context, current string) (AddonReleaseInfo, error) {
			return AddonReleaseInfo{Version: "1.5.0", Available: true}, nil
		}})
		report, err := b.Call("reportRefresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		r := report.(nativeReport)
		offer, err := b.Call("checkAddon", nil)
		if err != nil {
			t.Fatal(err)
		}
		o := offer.(nativeOffer)
		if o.Install == existing || r.Bundle.Install == existing || o.Bundle.Install == existing {
			t.Fatalf("existing=%v: offer.Install=%v report=%+v", existing, o.Install, r.Bundle)
		}
		if want := map[bool]string{false: "none", true: "installed"}[existing]; r.Bundle.State != want {
			t.Fatalf("existing=%v: bundle state %q, want %q", existing, r.Bundle.State, want)
		}
		j, _ := json.Marshal(o)
		if strings.Contains(string(j), "releases/download") {
			t.Fatal("download URL leaked to the UI")
		}
		b.Close()
	}
}
