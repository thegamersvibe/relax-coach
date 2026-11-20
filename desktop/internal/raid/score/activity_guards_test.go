package score_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestCastStartExpiresAtFallbackCap(t *testing.T) {
	for _, tc := range []struct{ gap, want int64 }{{10000, 11500}, {10001, 1500}, {40000, 1500}} {
		f := fixture("dps", "dps")
		a := f.Players[0]
		f.Events = []model.Event{
			{TimeMs: 110000, Kind: model.CastStart, Src: a, SpellID: 81},
			{TimeMs: 110000 + tc.gap, Kind: model.CastSuccess, Src: a, SpellID: 81},
			{TimeMs: 130000, Kind: model.CastSuccess, Src: f.Players[1]},
		}
		p := part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a), "activity")
		if p.Evidence.Counts["active_ms"] != tc.want {
			t.Errorf("gap=%d active=%d want=%d", tc.gap, p.Evidence.Counts["active_ms"], tc.want)
		}
	}
}

func TestCastStartUsesConfiguredCap(t *testing.T) {
	for _, cap := range []int64{1250, 15000} {
		c := tunedScoring(t, func(m map[string]any) { m["activity_cast_cap_ms"] = cap })
		for _, gap := range []int64{cap - 1, cap, cap + 1} {
			f := fixture("dps", "dps")
			a := f.Players[0]
			f.Events = []model.Event{{TimeMs: 110000, Kind: model.CastStart, Src: a, SpellID: 81}, {TimeMs: 110000 + gap, Kind: model.CastSuccess, Src: a, SpellID: 81}, {TimeMs: 140000, Kind: model.CastSuccess, Src: f.Players[1]}}
			p := part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, c), a), "activity")
			want := int64(1500)
			if gap <= cap {
				want += gap
			}
			if p.Evidence.Counts["active_ms"] != want {
				t.Errorf("cap=%d gap=%d active=%d want=%d", cap, gap, p.Evidence.Counts["active_ms"], want)
			}
		}
	}
}

func TestSpellZeroSelfMissIsNotMelee(t *testing.T) {
	for _, tc := range []struct {
		self      bool
		spellName string
		want      int64
	}{
		{true, "", 1500}, {false, "Synthetic spell", 1500}, {false, "", 3000},
	} {
		f := fixture("dps", "dps")
		a := f.Players[0]
		dst := model.Actor{ID: f.BossUnits[0]}
		if tc.self {
			dst = a
		}
		f.Events = append(f.Events, model.Event{TimeMs: 120000, Kind: model.Missed, Src: a, Dst: dst, SpellName: tc.spellName, MissType: "ABSORB"})
		p := part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a), "activity")
		if p.Evidence.Counts["active_ms"] != tc.want {
			t.Errorf("self=%v name=%q active=%d want=%d", tc.self, tc.spellName, p.Evidence.Counts["active_ms"], tc.want)
		}
	}
}
