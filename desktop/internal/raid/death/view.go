package death

import (
	"sort"
	"strings"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

type fightView struct {
	f         model.Fight
	events    []model.Event
	players   map[string]model.Actor
	ids       []string
	timelines map[string][]model.Event
}

func newView(f model.Fight) *fightView {
	v := &fightView{f: f, events: append([]model.Event(nil), f.Events...), players: map[string]model.Actor{}, timelines: map[string][]model.Event{}}
	sort.SliceStable(v.events, func(i, j int) bool { return v.events[i].TimeMs < v.events[j].TimeMs })
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
	start := sort.Search(end+1, func(i int) bool { return v.events[i].TimeMs >= from })
	for i := end - 1; i >= start; i-- {
		if v.events[i].Dst.ID == id && v.events[i].IsDeath() {
			start = i + 1
			break
		}
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
			break
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
	end := sort.Search(c.end+1, func(i int) bool { return c.view.events[i].TimeMs > at })
	return c.view.aliveBefore(id, end)
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
