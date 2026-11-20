package score_test

import (
	"slices"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestUncalibratedOutputExcluded(t *testing.T) {
	c := config(t)
	profile, ok := c.Profile("PALADIN.shockadin")
	if !ok || !profile.Uncalibrated {
		t.Fatal("shipped shockadin must be explicitly uncalibrated")
	}
	for _, role := range []string{"dps", "tank", "healer"} {
		t.Run(role, func(t *testing.T) {
			f := fixture(role, role, role)
			a := f.Players[0]
			f.Specs = map[string]string{a.ID: "PALADIN.shockadin"}
			ns := score.Score([]model.Fight{f}, nil, nil, c)
			n := playerScore(t, ns, a)
			p := part(t, n, "output")
			if p.Tenths.Present || p.Coverage != 0 || p.Baseline != "" || p.Provisional || n.Rank.Present {
				t.Fatalf("uncalibrated output measured/ranked: %+v", n)
			}
			if !slices.Contains(n.Fights[0].Flags, "uncalibratedSpec") || !part(t, n, "activity").Tenths.Present || !part(t, n, "mechanics").Tenths.Present {
				t.Fatal("flag missing or other parts excluded")
			}
			peer := part(t, playerScore(t, ns, f.Players[1]), "output")
			if !peer.Tenths.Present || peer.Evidence.Counts["baseline_players"] != 2 {
				t.Fatalf("unknown peers must retain role fallback without uncalibrated peer: %+v", peer)
			}
		})
	}
	f := fixture("dps", "dps")
	f.Specs = map[string]string{f.Players[0].ID: "PALADIN.shockadin"}
	for i := range c.Specs.Profiles {
		if c.Specs.Profiles[i].ID == "PALADIN.shockadin" {
			c.Specs.Profiles[i].Uncalibrated = false
		}
	}
	n := playerScore(t, score.Score([]model.Fight{f}, nil, nil, c), f.Players[0])
	if !part(t, n, "output").Tenths.Present || slices.Contains(n.Fights[0].Flags, "uncalibratedSpec") {
		t.Fatal("calibrated profile was excluded by name rather than flag")
	}
}
