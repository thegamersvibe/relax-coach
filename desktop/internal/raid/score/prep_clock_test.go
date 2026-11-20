package score_test

import (
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestPrepUTCClockSelection(t *testing.T) {
	for _, offset := range []int8{3, -4, 0} {
		t.Run(time.Duration(offset).String(), func(t *testing.T) {
			f := fixture("tank")
			// Keep UTC positive even when applying the east-of-UTC suffix.
			shift := int64(24 * time.Hour / time.Millisecond)
			f.StartMs += shift
			f.EndMs += shift
			for i := range f.Events {
				f.Events[i].TimeMs += shift
				f.Events[i].OffsetH = offset
			}
			// The first event is authoritative, not the last event in the tail.
			f.Events[len(f.Events)-1].OffsetH = offset + 1
			end := (f.EndMs - int64(offset)*int64(time.Hour/time.Millisecond)) / 1000
			before := stock(f.Players[0])
			before.Stamped = end
			before.SenderClock = 11
			after := stock(f.Players[0])
			after.Stamped = end + 3600
			after.SenderClock = 22
			after.BetterGear = []model.PrepGear{{Slot: 1}}
			early := stock(f.Players[0])
			early.Stamped = end - 20000
			early.SenderClock = 7
			got := part(t, score.Score([]model.Fight{f}, nil, []model.PrepReport{after, early, before}, config(t))[0], "preparation")
			counts := got.Evidence.Counts
			if counts["prep_stamped_s"] != end || counts["prep_sender_clock_s"] != 11 || counts["better_gear_slots"] != 0 {
				t.Fatalf("offset %d chose report %d instead of UTC end %d: %+v", offset, counts["prep_stamped_s"], end, counts)
			}
			if v, ok := counts["fight_end_utc_s"]; !ok || v != end {
				t.Fatalf("UTC end evidence = %d present %v, want %d", v, ok, end)
			}
		})
	}
}
