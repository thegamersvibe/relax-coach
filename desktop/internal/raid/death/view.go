package death

import (
	"strings"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

type fightView struct {
	f         model.Fight
	events    []model.Event
	players   map[string]model.Actor
	ids       []string
	timelines map[string][]model.Event
	clock     bool // no backward timestamp step anywhere in log order
}

// monotonic checks every adjacent pair of the fight in log order. Equal
// timestamps are valid; irrelevant events must not hide a clock step.
func monotonic(events []model.Event) bool {
	for i := 1; i < len(events); i++ {
		if events[i].TimeMs < events[i-1].TimeMs {
			return false
		}
	}
	return true
}

func newView(f model.Fight) *fightView {
	v := &fightView{f: f, events: append([]model.Event(nil), f.Events...), players: map[string]model.Actor{}, timelines: map[string][]model.Event{}}
	// Fight.Events is log order. Positions must never be timestamp-sorted.
	v.clock = monotonic(v.events)
	for _, p := range f.Players {
		if !strings.HasPrefix(p.ID, "Player-") {
			continue
		}
		if _, exists := v.players[p.ID]; !exists {
			v.ids = append(v.ids, p.ID)
		}
		v.players[p.ID] = p
	}
	// Owners are emitted in roster order, not randomized map iteration order.
	for _, e := range v.events {
		if _, ok := v.players[e.Dst.ID]; ok {
			v.timelines[e.Dst.ID] = append(v.timelines[e.Dst.ID], e)
		}
		if _, ok := v.players[e.InfoUnit]; ok && e.InfoUnit != e.Dst.ID {
			v.timelines[e.InfoUnit] = append(v.timelines[e.InfoUnit], e)
		}
	}
	return v
}

// deathWindow is bounded by event identity, not just its millisecond. A
// preceding real death cuts the life even when no resurrection was logged.
func (v *fightView) deathWindow(id string, from int64, end int) (int, []model.Event) {
	start := 0
	for i := end - 1; i >= 0; i-- {
		if v.events[i].Dst.ID == id && v.events[i].IsDeath() {
			start = i + 1
			break
		}
	}
	// Locate the first in-range line only after the previous death line.
	// Retain the positional span through this death, even across clock steps.
	for start < end && v.events[start].TimeMs < from {
		start++
	}
	var window []model.Event
	for _, e := range v.events[start : end+1] {
		if e.Dst.ID == id || e.InfoUnit == id {
			window = append(window, e)
		}
	}
	return start, window
}
func (v *fightView) alive(id string, at int64) bool {
	alive := true
	for _, e := range v.timelines[id] {
		if e.TimeMs > at {
			continue
		}
		if e.Dst.ID != id {
			continue
		}
		if e.IsDeath() {
			alive = false
		}
		if e.Kind == model.Resurrect {
			alive = true
		}
	}
	return alive
}

// aliveBefore observes transitions only before a particular log line.
func (v *fightView) aliveBefore(id string, before int) bool {
	alive := true
	for _, e := range v.events[:before] {
		if e.Dst.ID != id {
			continue
		}
		if e.IsDeath() {
			alive = false
		}
		if e.Kind == model.Resurrect {
			alive = true
		}
	}
	return alive
}
func (c *context) alive(id string, at int64) bool {
	alive := true
	for _, e := range c.view.events[:c.end+1] {
		if e.TimeMs > at || e.Dst.ID != id {
			continue
		}
		if e.IsDeath() {
			alive = false
		}
		if e.Kind == model.Resurrect {
			alive = true
		}
	}
	return alive
}
func (c *context) aliveThroughout(id string, from, to int64) bool {
	if !c.alive(id, from) {
		return false
	}
	for _, e := range c.view.events[:c.end+1] {
		if e.TimeMs >= from && e.TimeMs <= to && e.Dst.ID == id && e.IsDeath() {
			return false
		}
	}
	return true
}
func damage(e model.Event) bool { return e.Kind == model.Damage && e.Stream != model.SwingSource }
func nonnegative(o model.Opt) int64 {
	if !o.Present {
		return 0
	}
	return max(int64(0), o.V)
}
func effective(e model.Event) int64 {
	over := e.Overkill
	if e.Kind == model.Heal {
		over = e.Overheal
	}
	return max(int64(0), nonnegative(e.Amount)-nonnegative(over))
}
