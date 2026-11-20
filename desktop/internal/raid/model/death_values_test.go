package model_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestDeathValues(t *testing.T) {
	for _, tc := range []struct{ name, got, want string }{
		{"CauseAvoidable", model.CauseAvoidable, "avoidable ability"},
		{"CauseUnavoidable", model.CauseUnavoidable, "unavoidable or one-shot"},
		{"CauseUnhealed", model.CauseUnhealed, "unhealed over time"},
		{"CausePulledAggro", model.CausePulledAggro, "pulled aggro"},
		{"CauseLostBoss", model.CauseLostBoss, "tank lost the boss"},
		{"CauseAfterWipe", model.CauseAfterWipe, "after the wipe call"},
		{"CauseUnresolved", model.CauseUnresolved, "unresolved"},
		{"OwnerPlayer", model.OwnerPlayer, "player"},
		{"OwnerHealers", model.OwnerHealers, "healers"},
		{"OwnerNobody", model.OwnerNobody, "nobody"},
		{"OwnerUndecided", model.OwnerUndecided, "undecided"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}
