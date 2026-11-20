package score

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func preparation(f model.Fight, reports []model.PrepReport, c cfg.Config) partResult {
	// Model timestamps are the logger's wall clock parsed as UTC. Receipts
	// are server Unix seconds; remove the first event's east-of-UTC offset.
	var offset int64
	if len(f.Events) > 0 {
		offset = int64(f.Events[0].OffsetH) * int64(time.Hour/time.Millisecond)
	}
	endUTC := (f.EndMs - offset) / int64(time.Second/time.Millisecond)
	counts := map[string]int64{}
	measures := []measure{{"stock", nil}, {"bag_gear", nil}, {"used", nil}}
	if len(reports) == 0 {
		return makePart("preparation", measures, c.Scoring.MeasureWeights["preparation"], counts, c)
	}
	var chosen model.PrepReport
	found := false
	for _, r := range reports {
		if r.Stamped <= endUTC && (!found || r.Stamped > chosen.Stamped || r.Stamped == chosen.Stamped && prepTieAfter(r, chosen)) {
			chosen = r
			found = true
		}
	}
	if !found {
		return makePart("preparation", measures, c.Scoring.MeasureWeights["preparation"], counts, c)
	}
	counts["fight_end_utc_s"] = endUTC
	counts["prep_stamped_s"], counts["prep_sender_clock_s"] = chosen.Stamped, chosen.SenderClock
	readable := false
	for _, r := range reports {
		if r.Stamped > endUTC {
			continue
		}
		if r.ConsumablesUnreadable {
			continue
		}
		readable = true
		// Maximum reported stock per category avoids counting the same bag twice.
		bags := map[string]int64{}
		for _, item := range r.Consumables {
			if item.Count > 0 {
				bags[item.Category] += item.Count
			}
		}
		for _, category := range []string{"potion", "food", "flask", "elixir"} {
			key := category + "_count"
			counts[key] = max(counts[key], bags[category])
		}
	}
	if readable {
		value := int64(0)
		if counts["potion_count"] > 0 {
			value += 50
		}
		if counts["food_count"] > 0 {
			value += 50
		}
		measures[0].value = integer(value)
	}
	if chosen.GearChecked && !chosen.GearUnreadable {
		skipped := map[int64]bool{}
		for _, slot := range chosen.NotScored {
			skipped[slot] = true
		}
		slots := map[int64]bool{}
		for _, row := range chosen.BetterGear {
			if !skipped[row.Slot] {
				slots[row.Slot] = true
			}
		}
		counts["better_gear_slots"] = int64(len(slots))
		counts["not_scored_slots"] = int64(len(skipped))
		if penalty, ok := c.Scoring.CountPenalties["better_item_in_bags"]; ok {
			measures[1].value = clamp(sub(decimal(c.Scoring.CountStartScore), mul(integer(int64(len(slots))), decimal(penalty))))
		}
	}
	return makePart("preparation", measures, c.Scoring.MeasureWeights["preparation"], counts, c)
}

// Clocks establish recency; an exact clock tie uses the bytewise JSON of the
// full report as a stable key, not slice arrival order. PrepReport has no maps,
// floats or unsupported JSON values, so marshaling cannot fail.
func prepTieAfter(a, b model.PrepReport) bool {
	if a.SenderClock != b.SenderClock {
		return a.SenderClock > b.SenderClock
	}
	ak, _ := json.Marshal(a)
	bk, _ := json.Marshal(b)
	return bytes.Compare(ak, bk) > 0
}
