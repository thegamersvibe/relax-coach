package score

import (
	"sort"
	"strings"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

type interval struct{ start, end int64 }
type playerView struct {
	actor                                 model.Actor
	role                                  string
	alive                                 []interval
	life                                  []model.Event
	aliveMs, damage, activeMs, lowHealing int64
	activityKnown                         bool
	uncalibrated                          bool
	observed                              bool
}

func duration(f model.Fight) model.Opt {
	if f.Boss {
		return f.DurationMs
	}
	return model.Some(max(int64(0), f.EndMs-f.StartMs))
}
func ordered(events []model.Event) []model.Event {
	out := append([]model.Event(nil), events...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].TimeMs < out[j].TimeMs })
	return out
}
func owner(f model.Fight, id string) string {
	if o := f.Owners[id]; o != "" {
		return o
	}
	return id
}
func effective(amount, excess model.Opt) int64 {
	if !amount.Present {
		return 0
	}
	v := max(int64(0), amount.V)
	if excess.Present {
		v = max(int64(0), v-max(int64(0), excess.V))
	}
	return v
}
func views(f model.Fight, c cfg.Config) map[string]*playerView {
	out := map[string]*playerView{}
	d := duration(f)
	end := f.StartMs + max(int64(0), d.V)
	for _, p := range f.Players {
		profile, known := c.Profile(f.Specs[p.ID])
		out[p.ID] = &playerView{actor: p, role: f.Roles[p.ID], uncalibrated: known && profile.Uncalibrated}
	}
	events := ordered(f.Events)
	for id, v := range out {
		for _, e := range events {
			if e.TimeMs >= f.StartMs && e.TimeMs <= end && (owner(f, e.Src.ID) == id || e.Dst.ID == id || e.InfoUnit == id || owner(f, e.Caster.ID) == id) {
				v.observed = true
				break
			}
		}
		live, since := true, f.StartMs
		for _, e := range events {
			if e.Dst.ID != id || e.TimeMs < f.StartMs {
				continue
			}
			if e.IsDeath() || e.Kind == model.Resurrect {
				v.life = append(v.life, e)
			}
			at := min(end, e.TimeMs)
			if e.IsDeath() && live {
				v.alive = append(v.alive, interval{since, at})
				live = false
			}
			if e.Kind == model.Resurrect && !live {
				since = at
				live = true
			}
		}
		if live {
			v.alive = append(v.alive, interval{since, end})
		}
		for _, s := range v.alive {
			v.aliveMs += max(int64(0), s.end-s.start)
		}
		v.activeMs, v.activityKnown = activeTime(v, events, c)
	}
	bosses := map[string]bool{}
	for _, id := range f.BossUnits {
		bosses[id] = true
	}
	for _, e := range events {
		if e.Kind != model.Damage || e.Stream == model.SwingSource {
			continue
		}
		target := bosses[e.Dst.ID]
		if !f.Boss {
			target = strings.HasPrefix(e.Dst.ID, "Creature-") && f.Owners[e.Dst.ID] == "" && e.Dst.Flags&0x10 == 0
		}
		if v := out[owner(f, e.Src.ID)]; v != nil && target {
			v.damage += effective(e.Amount, e.Overkill)
		}
	}
	lowHealthHealing(f, out, c)
	return out
}
