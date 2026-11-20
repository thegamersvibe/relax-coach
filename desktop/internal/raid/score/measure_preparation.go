package score

import (
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func preparation(f model.Fight, reports []model.PrepReport, c cfg.Config) model.Part {
	counts := map[string]int64{}
	measures := []measure{{"stock", nil}, {"bag_gear", nil}, {"used", nil}}
	if len(reports) == 0 {
		return makePart("preparation", measures, c.Scoring.MeasureWeights["preparation"], counts, c)
	}
	chosen := reports[0]
	// First find the earliest, then replace it with the latest eligible report.
	for _, r := range reports {
		if r.Stamped < chosen.Stamped {
			chosen = r
		}
	}
	found := false
	for _, r := range reports {
		if r.Stamped <= f.EndMs/int64(time.Second/time.Millisecond) && (!found || r.Stamped > chosen.Stamped) {
			chosen = r
			found = true
		}
	}
	counts["prep_stamped_s"], counts["prep_sender_clock_s"] = chosen.Stamped, chosen.SenderClock
	readable := false
	for _, r := range reports {
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
