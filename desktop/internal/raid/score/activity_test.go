package score_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"testing"
)

func TestActiveIntervals(t *testing.T) {
	f := fixture("dps", "dps", "dps")
	a := f.Players[0]
	pet := mt.Pet("Hound", a)
	f.Owners = map[string]string{pet.ID: a.ID}
	f.Events = f.Events[:2]
	ev := func(at int64, kind model.Kind, spell int64) model.Event {
		return model.Event{TimeMs: at, Kind: kind, Src: a, SpellID: spell}
	}
	energy := ev(114000, model.CastSuccess, 2)
	energy.PowerType = model.Some(3)
	f.Events = append(f.Events, ev(110000, model.CastStart, 1), ev(112000, model.CastSuccess, 1), ev(112500, model.CastStart, 2), energy,
		ev(116000, model.CastStart, 3), ev(118000, model.CastFailed, 3), ev(120000, model.CastSuccess, 4),
		model.Event{TimeMs: 125000, Kind: model.Damage, Src: a, Stream: model.SwingLanded},
		model.Event{TimeMs: 126000, Kind: model.Damage, Src: a, Stream: model.SwingSource},
		ev(127000, model.Missed, 0), ev(130000, model.Damage, 75), ev(133000, model.Missed, 75),
		model.Event{TimeMs: 135000, Kind: model.CastSuccess, Src: pet}, ev(159500, model.CastSuccess, 5),
		model.Event{TimeMs: 120000, Kind: model.CastSuccess, Src: f.Players[1]})
	got := score.Score([]model.Fight{f}, nil, nil, config(t))
	p := part(t, playerScore(t, got, a), "activity")
	if p.Evidence.Counts["active_ms"] != 10000 || p.Evidence.Counts["alive_ms"] != 60000 || p.Coverage != 100 || p.Baseline != "role" {
		t.Fatalf("activity %+v", p)
	}
	// No own or owned events is unmeasured, never an idle-player zero.
	p = part(t, playerScore(t, got, f.Players[2]), "activity")
	if p.Tenths.Present || p.Evidence.Counts["active_ms"] != 0 {
		t.Fatalf("zero activity %+v", p)
	}
}
func TestEnergyAndFailedCasts(t *testing.T) {
	for _, tc := range []struct {
		power model.Opt
		want  int64
	}{{model.Some(3), 1000}, {model.Some(0), 1500}, {model.Opt{}, 1500}} {
		f := fixture("tank", "tank")
		a := f.Players[0]
		f.Events = []model.Event{{TimeMs: 110000, Kind: model.CastSuccess, Src: a, PowerType: tc.power}, {TimeMs: 120000, Kind: model.CastSuccess, Src: f.Players[1]}}
		p := part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a), "activity")
		if p.Evidence.Counts["active_ms"] != tc.want {
			t.Fatalf("power %+v: %+v", tc.power, p)
		}
	}
	f := fixture("dps", "dps")
	a := f.Players[0]
	f.Events = []model.Event{{TimeMs: 110000, Kind: model.CastStart, Src: a, SpellID: 1}, {TimeMs: 120000, Kind: model.CastFailed, Src: a, SpellID: 1}, {TimeMs: 125000, Kind: model.CastFailed, Src: a}, {TimeMs: 130000, Kind: model.CastSuccess, Src: f.Players[1]}}
	p := part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a), "activity")
	if p.Evidence.Counts["active_ms"] != 0 || p.Tenths != model.Some(0) {
		t.Fatalf("failed casts %+v", p)
	}
}
func TestActivityAliveClipping(t *testing.T) {
	f := fixture("dps", "dps")
	a := f.Players[0]
	f.Events = append(f.Events, model.Event{TimeMs: 129000, Kind: model.CastSuccess, Src: a}, mt.At(130000).Dst(a).Died(0).Build(), model.Event{TimeMs: 135000, Kind: model.CastSuccess, Src: a}, model.Event{TimeMs: 140000, Kind: model.Resurrect, Dst: a}, model.Event{TimeMs: 140000, Kind: model.CastSuccess, Src: a})
	p := part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a), "activity")
	if p.Evidence.Counts["active_ms"] != 4000 || p.Evidence.Counts["alive_ms"] != 50000 {
		t.Fatalf("alive clip %+v", p)
	}
}
