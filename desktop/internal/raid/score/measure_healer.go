package score

import (
	"math/big"
	"slices"

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
func healerOutput(f model.Fight, v *playerView, all map[string]*playerView, deaths []model.Death, c cfg.Config) partResult {
	n, total := 0, int64(0)
	for _, p := range all {
		if p.role == "healer" && p.aliveMs > 0 && p.observed && !p.uncalibrated {
			n++
			total += p.lowHealing
		}
	}
	// v is already a roster member. Exact owner names must identify only its
	// ID across the entire roster, not just the measured/living healer subset.
	ownerUnique := v.actor.Name != ""
	for _, p := range f.Players {
		if p.Name == v.actor.Name && p.ID != v.actor.ID {
			ownerUnique = false
			break
		}
	}
	unhealed := int64(0)
	for _, d := range deaths {
		if ownerUnique && d.FightID == f.ID && d.Cause == model.CauseUnhealed && d.OwnerKind == model.OwnerHealers && slices.Contains(d.Owners, v.actor.Name) {
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
	if v.aliveMs <= 0 || v.uncalibrated {
		team = nil
		individual = nil
	}
	counts := map[string]int64{"alive_ms": v.aliveMs, "low_health_effective": v.lowHealing, "team_low_health_effective": total, "baseline_players": int64(n), "unhealed_deaths": unhealed}
	weights := map[string]*big.Rat{"healer_team": mul(decimal(c.Scoring.Healer.TeamShare), integer(100)), "healer_individual": mul(decimal(c.Scoring.Healer.IndividualShare), integer(100))}
	p := makePartExact("output", []measure{{"healer_team", team}, {"healer_individual", individual}}, weights, counts, c)
	if p.Tenths.Present {
		p.Baseline = "role"
		p.Provisional = n < c.Scoring.Confidence.MinBaselinePlayers
		if unhealed > 0 {
			p.Tenths.V = min(p.Tenths.V, points(decimal(c.Scoring.Healer.UnhealedOutputCap), c).V)
		}
	}
	return p
}
