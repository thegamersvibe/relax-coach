package score_test

import (
	"sort"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

// Adapted from the round-3 reviewer probe, through the real classifier/scorer.
// A acted earlier but died first; only living B appears in the owner list.
func TestHealerOwnerNameResolution(t *testing.T) {
	for _, mode := range []string{"unique", "empty", "empty_unique", "unknown", "two_ids", "nonhealer_collision", "same_id_twice"} {
		t.Run(mode, func(t *testing.T) {
			f := secondUnhealedFight()
			switch mode {
			case "empty":
				f.Players[0].Name, f.Players[1].Name = "", ""
			case "empty_unique":
				f.Players[1].Name = ""
			case "two_ids":
				f.Players[0].Name = f.Players[1].Name
			case "nonhealer_collision":
				f.Players[2].Name = f.Players[1].Name
			case "same_id_twice":
				f.Players = append(f.Players, f.Players[1])
			}
			for i := range f.Events {
				for _, p := range f.Players {
					for _, a := range []*model.Actor{&f.Events[i].Src, &f.Events[i].Dst, &f.Events[i].Caster} {
						if a.ID == p.ID {
							a.Name = p.Name
						}
					}
				}
			}
			a, b := f.Players[0], f.Players[1]
			f.Events = append(f.Events, mt.At(120000).Dst(a).Died(0).Build())
			// The fixture deliberately supplies a monotonic log, including the
			// earlier healer death; this is an identity test, not a clock probe.
			sort.SliceStable(f.Events, func(i, j int) bool { return f.Events[i].TimeMs < f.Events[j].TimeMs })
			c := config(t)
			ds, _, excluded, err := death.Review(f, f.EndMs, c)
			if err != nil || excluded != 0 {
				t.Fatalf("classifier error=%v excluded=%d", err, excluded)
			}
			found := false
			for i, d := range ds {
				if d.Player.ID != f.Players[2].ID {
					continue
				}
				found = true
				if d.Cause != model.CauseUnhealed || d.OwnerKind != model.OwnerHealers || len(d.Owners) != 1 || d.Owners[0] != b.Name {
					t.Fatalf("invalid selecting control: %+v", d)
				}
				if mode == "unknown" {
					ds[i].Owners = []string{"Unknown-Synthetic-Realm"}
				}
			}
			if !found {
				t.Fatal("victim death missing")
			}
			ns := score.Score([]model.Fight{f}, ds, nil, c)
			for _, h := range []model.Actor{a, b} {
				p := part(t, playerScore(t, ns, h), "output")
				charge, team, output := int64(0), int64(1000), int64(850)
				if h.ID == b.ID && (mode == "unique" || mode == "same_id_twice") {
					charge, team, output = 1, 0, 350
				}
				t.Logf("mode=%s healerA=%v charge=%d team=%d output=%v", mode, h.ID == a.ID, p.Evidence.Counts["unhealed_deaths"], p.Evidence.Counts["points_tenths.healer_team"], p.Tenths)
				if p.Evidence.Counts["unhealed_deaths"] != charge || p.Evidence.Counts["points_tenths.healer_team"] != team || p.Tenths != model.Some(output) {
					t.Errorf("owner must resolve to exactly one roster ID: want charge=%d team=%d output=%d", charge, team, output)
				}
			}
		})
	}
}
