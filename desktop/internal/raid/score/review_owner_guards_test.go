package score_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestHealerDeathChargeRequiresClassifierOwnership(t *testing.T) {
	f := healingFixture()
	a, b := f.Players[0], f.Players[1]
	for _, tc := range []struct {
		name, cause, kind, fight string
		owners                   []string
		charged                  bool
	}{
		{"owner", model.CauseUnhealed, model.OwnerHealers, f.ID, []string{a.Name}, true},
		{"other_owner", model.CauseUnhealed, model.OwnerHealers, f.ID, []string{b.Name}, false},
		{"empty_owners", model.CauseUnhealed, model.OwnerHealers, f.ID, nil, false},
		{"player_kind", model.CauseUnhealed, model.OwnerPlayer, f.ID, []string{a.Name}, false},
		{"undecided_kind", model.CauseUnhealed, model.OwnerUndecided, f.ID, []string{a.Name}, false},
		{"other_fight", model.CauseUnhealed, model.OwnerHealers, "other", []string{a.Name}, false},
		// M1: dropping the cause guard must not charge an adjacent non-unhealed death.
		{"unresolved", model.CauseUnresolved, model.OwnerHealers, f.ID, []string{a.Name, b.Name}, false},
		{"avoidable", model.CauseAvoidable, model.OwnerHealers, f.ID, []string{a.Name, b.Name}, false},
		{"after_wipe", model.CauseAfterWipe, model.OwnerHealers, f.ID, []string{a.Name, b.Name}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ds := []model.Death{{FightID: tc.fight, Player: f.Players[2], TimeMs: 130000, Cause: tc.cause, OwnerKind: tc.kind, Owners: tc.owners}}
			p := part(t, playerScore(t, score.Score([]model.Fight{f}, ds, nil, config(t)), a), "output")
			count, team := int64(0), int64(1000)
			if tc.charged {
				count, team = 1, 0
			}
			if p.Evidence.Counts["unhealed_deaths"] != count || p.Evidence.Counts["points_tenths.healer_team"] != team {
				t.Fatalf("unexpected charge=%v team=%v", p.Evidence.Counts["unhealed_deaths"], p.Evidence.Counts["points_tenths.healer_team"])
			}
		})
	}
}
