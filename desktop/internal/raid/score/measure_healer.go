package score

import (
	"math/big"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func lowHealthHealing(f model.Fight, all map[string]*playerView, c cfg.Config) {
	if c.Scoring.Healer.LowHealthFraction == nil {
		return
	}
	low := map[string]bool{}
	for _, e := range ordered(f.Events) {
		// The preceding sample describes the target before the heal, not the
		// heal's own post-event HP. Never borrow the source's advanced block.
		if low[e.Dst.ID] && all[e.Dst.ID] != nil {
			id, amount := owner(f, e.Src.ID), int64(0)
			switch e.Kind {
			case model.Heal:
				amount = effective(e.Amount, e.Overheal)
			case model.Absorb:
				id = owner(f, e.Caster.ID)
				amount = effective(e.Amount, model.Opt{})
			}
			if v := all[id]; v != nil && v.role == "healer" {
				v.lowHealing += amount
			}
		}
		if all[e.InfoUnit] != nil && e.HP.Present && e.HPMax.Present && e.HPMax.V > 0 {
			low[e.InfoUnit] = integer(e.HP.V).Cmp(mul(integer(e.HPMax.V), decimal(*c.Scoring.Healer.LowHealthFraction))) <= 0
		}
	}
}
func aliveAt(v *playerView, at int64) bool {
	live := true
	for _, e := range v.life {
		if e.TimeMs > at {
			break
		}
		if e.IsDeath() {
			live = false
		}
		if e.Kind == model.Resurrect {
			live = true
		}
	}
	return live
}
func healerOutput(f model.Fight, v *playerView, all map[string]*playerView, deaths []model.Death, c cfg.Config) model.Part {
	n, total := 0, int64(0)
	for _, p := range all {
		if p.role == "healer" && p.aliveMs > 0 {
			n++
			total += p.lowHealing
		}
	}
	unhealed := int64(0)
	for _, d := range deaths {
		if d.FightID == f.ID && d.Cause == "unhealed" && aliveAt(v, d.TimeMs) {
			unhealed++
		}
	}
	team := decimal(c.Scoring.CountStartScore)
	if unhealed > 0 {
		team = decimal(c.Scoring.Healer.UnhealedTeamScore)
	}
	var individual *big.Rat
	if n >= c.Scoring.Confidence.MinMeasurablePlayers && total > 0 && c.Scoring.Healer.LowHealthFraction != nil {
		individual = ratioPoints(div(mul(integer(v.lowHealing), integer(int64(n))), integer(total)), c)
	}
	if v.aliveMs <= 0 {
		team = nil
		individual = nil
	}
	counts := map[string]int64{"alive_ms": v.aliveMs, "low_health_effective": v.lowHealing, "team_low_health_effective": total, "baseline_players": int64(n), "unhealed_deaths": unhealed}
	weights := map[string]float64{"healer_team": c.Scoring.Healer.TeamShare * 100, "healer_individual": c.Scoring.Healer.IndividualShare * 100}
	p := makePart("output", []measure{{"healer_team", team}, {"healer_individual", individual}}, weights, counts, c)
	if p.Tenths.Present {
		p.Baseline = "role"
		p.Provisional = n < c.Scoring.Confidence.MinBaselinePlayers
		if unhealed > 0 {
			p.Tenths.V = min(p.Tenths.V, points(decimal(c.Scoring.Healer.UnhealedOutputCap), c).V)
		}
	}
	return p
}
