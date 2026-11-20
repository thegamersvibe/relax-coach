package score_test

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestPrepSameStampUsesSenderThenStableKey(t *testing.T) {
	for _, stamp := range []int64{90, 161} { // eligible and earliest-night fallback
		f := fixture("tank")
		a := stock(f.Players[0])
		a.Stamped, a.SenderClock = stamp, 10
		b := a
		b.SenderClock = 11
		b.BetterGear = []model.PrepGear{{Slot: 1}, {Slot: 2}, {Slot: 3}, {Slot: 4}}
		get := func(rs []model.PrepReport) model.Part {
			return part(t, score.Score([]model.Fight{f}, nil, rs, config(t))[0], "preparation")
		}
		for _, tiedClock := range []bool{false, true} {
			if tiedClock {
				a.SenderClock = b.SenderClock
			}
			x, y := get([]model.PrepReport{a, b}), get([]model.PrepReport{b, a})
			if !reflect.DeepEqual(x, y) {
				t.Fatalf("stamp=%d tiedClock=%v: order chose %v versus %v", stamp, tiedClock, x.Tenths, y.Tenths)
			}
			if !tiedClock && (x.Evidence.Counts["prep_sender_clock_s"] != 11 || x.Evidence.Counts["points_tenths.bag_gear"] != 0) {
				t.Fatalf("latest sender clock not chosen: %+v", x)
			}
		}
	}
}
