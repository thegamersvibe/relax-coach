package score

import (
	"slices"
	"sort"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func mechanics(f model.Fight, v *playerView, deaths []model.Death, c cfg.Config) model.Part {
	counts := map[string]int64{"owned_deaths": 0, "unresolved_deaths": 0, "boss_swings_taken": 0}
	var at []int64
	severity := integer(0)
	d := duration(f)
	for _, death := range deaths {
		if death.FightID != f.ID {
			continue
		}
		if death.Player.ID == v.actor.ID && death.Cause == "unresolved" {
			counts["unresolved_deaths"]++
		}
		if death.OwnerKind != "player" || death.Cause == "unhealed" || death.Cause == "unresolved" || death.Cause == "afterWipeCall" || !slices.Contains(death.Owners, v.actor.Name) {
			continue
		}
		counts["owned_deaths"]++
		elapsed := min(max(int64(0), death.TimeMs-f.StartMs), d.V)
		at = append(at, max(int64(0), death.TimeMs-f.StartMs))
		severity = add(severity, sub(integer(1), div(integer(elapsed), mul(integer(2), integer(d.V)))))
	}
	for _, e := range f.Events {
		if e.Kind == model.Damage && e.Stream == model.SwingLanded && e.Dst.ID == v.actor.ID && slices.Contains(f.BossUnits, e.Src.ID) {
			counts["boss_swings_taken"]++
		}
	}
	counts["death_severity_num"], counts["death_severity_den"] = severity.Num().Int64(), severity.Denom().Int64()
	measures := []measure{{"deaths", nil}, {"avoidable", nil}}
	if penalty, ok := c.Scoring.CountPenalties["owned_death"]; ok && d.Present && d.V > 0 {
		measures[0].value = clamp(sub(decimal(c.Scoring.CountStartScore), mul(decimal(penalty), severity)))
	}
	if v.role != "tank" {
		m := measure{"boss_swings", nil}
		if penalty, ok := c.Scoring.CountPenalties["boss_swing_taken"]; ok {
			m.value = clamp(sub(decimal(c.Scoring.CountStartScore), mul(decimal(penalty), integer(counts["boss_swings_taken"]))))
		}
		measures = append(measures, m)
	}
	p := makePart("mechanics", measures, c.Scoring.MeasureWeights["mechanics"], counts, c)
	sort.Slice(at, func(i, j int) bool { return at[i] < at[j] })
	if len(at) > 20 {
		at = at[:20]
	}
	p.Evidence.AtMs = at
	return p
}
func utility(f model.Fight, v *playerView, c cfg.Config) model.Part {
	counts := map[string]int64{"interrupts": 0, "dispels": 0}
	for _, e := range f.Events {
		if owner(f, e.Src.ID) == v.actor.ID {
			if e.Kind == model.Interrupt {
				counts["interrupts"]++
			}
			if e.Kind == model.Dispel {
				counts["dispels"]++
			}
		}
	}
	return makePart("utility", nil, nil, counts, c)
}
