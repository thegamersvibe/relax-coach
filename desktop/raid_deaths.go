package main

import (
	"slices"
	"sort"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

type raidDeathView struct {
	Time, Fight, Player, KilledBy, Cause, Filter, Confidence, Suggestion, OwnerKind string
	Owners                                                                          []string
	Evidence                                                                        []int64
}
type raidWipeView struct {
	HasFirst                bool
	Number                  int
	Fight, Window, Duration string
	First                   raidDeathView
	Following               []raidDeathView
}

func raidDeathRows(f model.Fight, deaths []model.Death, c cfg.Config) []raidDeathView {
	events := append([]model.Event{}, f.Events...)
	sort.SliceStable(events, func(i, j int) bool { return events[i].TimeMs < events[j].TimeMs })
	used := map[int]bool{}
	out := []raidDeathView{}
	for _, d := range deaths {
		v := raidDeathView{Time: time.UnixMilli(d.TimeMs).UTC().Format("15:04:05.000"), Fight: f.Name, Player: d.Player.Name, KilledBy: "unknown", Cause: d.Cause, Filter: "other", Confidence: d.Confidence, Suggestion: d.SuggestedCause, OwnerKind: d.OwnerKind, Owners: d.Owners, Evidence: d.Evidence}
		if !f.Boss {
			v.Fight = "trash"
		}
		if d.Cause == model.CauseUnresolved || d.Cause == "" {
			v.Cause = "unknown"
			v.Filter = "unknown"
		}
		if d.Cause == model.CauseAvoidable {
			v.Filter = "avoidable"
		}
		end := -1
		for i, e := range events {
			if !used[i] && e.IsDeath() && e.Dst.ID == d.Player.ID && e.TimeMs == d.TimeMs {
				end = i
				used[i] = true
				break
			}
		}
		for i := end - 1; i >= 0; i-- {
			e := events[i]
			if e.TimeMs < d.TimeMs-c.Scoring.Death.WindowMs {
				break
			}
			if e.Dst.ID != d.Player.ID {
				continue
			}
			if e.IsDeath() {
				break
			}
			if e.Kind != model.Damage || e.Stream == model.SwingSource {
				continue
			}
			v.KilledBy = e.SpellName
			if e.Stream == model.SwingLanded {
				v.KilledBy = "melee"
			}
			if v.KilledBy == "" {
				v.KilledBy = e.Src.Name
			}
			if v.KilledBy == "" {
				v.KilledBy = "unknown"
			}
			// A missing inferred role is not evidence that the player is a
			// non-tank. Keep unknown/trash deaths unknown, even with a unit hint.
			role := f.Roles[d.Player.ID]
			if f.Boss && d.Cause == model.CauseUnresolved && d.SuggestedCause == "" && (role == "dps" || role == "healer") && e.Stream == model.SwingLanded && slices.Contains(f.BossUnits, e.Src.ID) {
				v.Cause = "aggro: puller or tank"
			}
			break
		}
		out = append(out, v)
	}
	return out
}
