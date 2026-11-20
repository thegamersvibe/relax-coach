package score

import (
	"math/big"
	"sort"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Only completed casts supply intervals. A failed cast is private to the
// logger, so it never supplies activity. RANGE events supply none either:
// shots already have CastSuccess lines (PHASE1-BRIEF, shots in active time).
func activeTime(v *playerView, events []model.Event, c cfg.Config) (int64, bool) {
	if c.Scoring.GCDMs == nil || c.Scoring.GCDEnergyMs == nil {
		return 0, false
	}
	var spans []interval
	starts := map[int64]int64{}
	for _, e := range events {
		if e.Src.ID != v.actor.ID {
			continue
		} // Pet uptime is a phase-two measure.
		switch e.Kind {
		case model.CastStart:
			starts[e.SpellID] = e.TimeMs
		case model.CastFailed:
			delete(starts, e.SpellID)
		case model.CastSuccess:
			start := e.TimeMs
			if s, ok := starts[e.SpellID]; ok {
				start = s
			}
			delete(starts, e.SpellID)
			gcd := *c.Scoring.GCDMs
			if e.PowerType == model.Some(3) {
				gcd = *c.Scoring.GCDEnergyMs
			}
			spans = append(spans, interval{start, e.TimeMs + gcd})
		case model.Damage, model.Missed:
			if e.Stream == model.SwingLanded || e.Kind == model.Missed && e.SpellID == 0 {
				spans = append(spans, interval{e.TimeMs, e.TimeMs + *c.Scoring.GCDMs})
			}
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var union []interval
	for _, s := range spans {
		n := len(union)
		if n > 0 && s.start <= union[n-1].end {
			union[n-1].end = max(union[n-1].end, s.end)
		} else {
			union = append(union, s)
		}
	}
	total := int64(0)
	for _, s := range union {
		for _, a := range v.alive {
			total += max(int64(0), min(s.end, a.end)-max(s.start, a.start))
		}
	}
	return total, true
}
func activity(v *playerView, all map[string]*playerView, c cfg.Config) model.Part {
	counts := map[string]int64{"active_ms": v.activeMs, "alive_ms": v.aliveMs}
	value, n, base := roleRatio(v, all, func(p *playerView) *big.Rat {
		if !p.activityKnown {
			return nil
		}
		return div(integer(p.activeMs), integer(p.aliveMs))
	}, c)
	counts["baseline_players"] = int64(n)
	if base != nil {
		counts["baseline_active_num"] = base.Num().Int64()
		counts["baseline_active_den"] = base.Denom().Int64()
	}
	p := makePart("activity", []measure{{"active_time", value}}, c.Scoring.MeasureWeights["activity"], counts, c)
	if p.Tenths.Present {
		p.Baseline = "role"
		p.Provisional = n < c.Scoring.Confidence.MinBaselinePlayers
	}
	return p
}
