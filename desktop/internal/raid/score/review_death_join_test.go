package score_test

import (
	"sort"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func secondUnhealedFight() model.Fight {
	f := fixture("healer", "healer", "dps", "tank")
	v := f.Players[2]
	boss := model.Actor{ID: f.BossUnits[0]}
	for _, h := range f.Players[:2] {
		f.Events = append(f.Events, model.Event{TimeMs: 112000, InfoUnit: h.ID, HP: model.Some(10), HPMax: model.Some(100)}, mt.At(113000).Src(h).Dst(h).Heal(100, 0).Build())
	}
	f.Events = append(f.Events, model.Event{TimeMs: 124000, InfoUnit: v.ID, HP: model.Some(1000), HPMax: model.Some(1000)}, mt.At(125000).Src(boss).Dst(v).Spell(81, "Synthetic spell").Damage(10, -1).Build(), model.Event{TimeMs: 127000, InfoUnit: v.ID, HP: model.Some(500), HPMax: model.Some(1000)}, mt.At(129000).Src(boss).Dst(v).Spell(81, "Synthetic spell").Damage(10, -1).Build(), model.Event{TimeMs: 129500, InfoUnit: v.ID, HP: model.Some(300), HPMax: model.Some(1000)}, mt.At(130000).Dst(v).Died(0).Build())
	return f
}
func TestSecondDeathJoinSameMillisecond(t *testing.T) {
	c := config(t)
	for _, mode := range []string{"resurrect-after", "die-after"} {
		t.Run(mode, func(t *testing.T) {
			f := secondUnhealedFight()
			h := f.Players[0]
			if mode == "resurrect-after" {
				f.Events = append(f.Events, mt.At(120000).Dst(h).Died(0).Build(), model.Event{TimeMs: 130000, Kind: model.Resurrect, Dst: h})
			} else {
				f.Events = append(f.Events, mt.At(130000).Dst(h).Died(0).Build())
			}
			// Explicit synthetic log order: prior healer death before victim; tied
			// resurrection/death after victim. No production timestamp reordering.
			sort.SliceStable(f.Events, func(i, j int) bool { return f.Events[i].TimeMs < f.Events[j].TimeMs })
			ds, _, excluded, err := death.Review(f, f.EndMs, c)
			if err != nil {
				t.Fatal(err)
			}
			var victim model.Death
			for _, d := range ds {
				if d.Player.ID == f.Players[2].ID {
					victim = d
				}
			}
			if excluded != 0 || victim.Cause != model.CauseUnhealed || victim.OwnerKind != model.OwnerHealers {
				t.Fatalf("not a valid unhealed fixture: %+v", victim)
			}
			owner := false
			for _, s := range victim.Owners {
				if s == h.Name {
					owner = true
				}
			}
			if owner != (mode == "die-after") {
				t.Fatal("unexpected classifier ownership")
			}
			p := part(t, playerScore(t, score.Score([]model.Fight{f}, ds, nil, c), h), "output")
			t.Logf("mode=%s death.Review includes healer=%v score unhealed=%d team=%d output=%v", mode, owner, p.Evidence.Counts["unhealed_deaths"], p.Evidence.Counts["points_tenths.healer_team"], p.Tenths)
			wantTeam, wantOutput := int64(1000), int64(850)
			if owner {
				wantTeam, wantOutput = 0, 350
			}
			if p.Tenths != model.Some(wantOutput) || p.Evidence.Counts["points_tenths.healer_team"] != wantTeam {
				t.Error("joined owner charge produced wrong output")
			}
			if owner != (p.Evidence.Counts["unhealed_deaths"] == 1) {
				t.Error("score contradicts real death.Review owner at same millisecond")
			}
		})
	}
}
