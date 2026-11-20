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
func (v *fightView) window(id string, from, to int64) []model.Event {
	es := v.timelines[id]
	start := sort.Search(len(es), func(i int) bool { return es[i].TimeMs >= from })
	end := sort.Search(len(es), func(i int) bool { return es[i].TimeMs > to })
	return es[start:end]
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
func (v *fightView) aliveThroughout(id string, from, to int64) bool {
	if !v.alive(id, from) {
		return false
	}
	for _, e := range v.window(id, from, to) {
		if e.Dst.ID == id && e.IsDeath() {
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
