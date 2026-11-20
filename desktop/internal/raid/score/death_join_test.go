package score_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestDeathReviewUnhealedCapsHealerTeam(t *testing.T) {
	f := healingFixture()
	c := config(t)
	victim := f.Players[2]
	boss := model.Actor{ID: f.BossUnits[0]}
	f.Events = append(f.Events,
		model.Event{TimeMs: 124500, InfoUnit: victim.ID, HP: model.Some(1000), HPMax: model.Some(1000)},
		mt.At(125000).Src(boss).Dst(victim).Spell(81, "Synthetic damage").Damage(10, -1).Build(),
		model.Event{TimeMs: 126000, InfoUnit: victim.ID, HP: model.Some(500), HPMax: model.Some(1000)},
		mt.At(129000).Src(boss).Dst(victim).Spell(81, "Synthetic damage").Damage(10, -1).Build(),
		model.Event{TimeMs: 129500, InfoUnit: victim.ID, HP: model.Some(300), HPMax: model.Some(1000)},
		mt.At(130000).Dst(victim).Died(0).Build())
	deaths, _, excluded := death.Review(f, 160000, c)
	if excluded != 0 || len(deaths) != 1 || deaths[0].Cause != model.CauseUnhealed || deaths[0].OwnerKind != model.OwnerHealers {
		t.Fatalf("fixture did not produce an R6 unhealed death: %+v excluded=%d", deaths, excluded)
	}
	ns := score.Score([]model.Fight{f}, deaths, nil, c)
	for _, healer := range f.Players[:2] {
		p := part(t, playerScore(t, ns, healer), "output")
		if p.Evidence.Counts["unhealed_deaths"] != 1 || p.Evidence.Counts["points_tenths.healer_team"] != 0 || p.Tenths != model.Some(350) {
			t.Fatalf("R6 cause did not cap healer team: %+v", p)
		}
	}
}
