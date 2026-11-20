package score

import (
	"math/big"
	"sort"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func median(values []*big.Rat) *big.Rat {
	if len(values) == 0 {
		return nil
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Cmp(values[j]) < 0 })
	n := len(values)
	if n%2 == 1 {
		return values[n/2]
	}
	return div(add(values[n/2-1], values[n/2]), integer(2))
}
func ratioPoints(r *big.Rat, c cfg.Config) *big.Rat {
	ps := c.Scoring.RatioPoints
	if len(ps) == 0 {
		return nil
	}
	if r.Cmp(decimal(ps[0].Ratio)) <= 0 {
		return decimal(ps[0].Score)
	}
	for i := 1; i < len(ps); i++ {
		hi, lo := ps[i], ps[i-1]
		if r.Cmp(decimal(hi.Ratio)) <= 0 {
			fraction := div(sub(r, decimal(lo.Ratio)), sub(decimal(hi.Ratio), decimal(lo.Ratio)))
			return add(decimal(lo.Score), mul(fraction, sub(decimal(hi.Score), decimal(lo.Score))))
		}
	}
	return decimal(ps[len(ps)-1].Score)
}
func roleRatio(v *playerView, all map[string]*playerView, value func(*playerView) *big.Rat, c cfg.Config) (*big.Rat, int, *big.Rat) {
	var peers []*big.Rat
	for _, p := range all {
		if p.role == v.role && p.aliveMs > 0 && p.observed {
			if x := value(p); x != nil {
				peers = append(peers, x)
			}
		}
	}
	n := len(peers)
	base := median(peers)
	if n < c.Scoring.Confidence.MinMeasurablePlayers || base == nil || base.Sign() <= 0 || v.aliveMs <= 0 {
		return nil, n, base
	}
	x := value(v)
	if x == nil {
		return nil, n, base
	}
	return ratioPoints(div(x, base), c), n, base
}
func output(f model.Fight, v *playerView, all map[string]*playerView, deaths []model.Death, c cfg.Config) partResult {
	if v.role == "healer" {
		return healerOutput(f, v, all, deaths, c)
	}
	counts := map[string]int64{"boss_damage": v.damage, "alive_ms": v.aliveMs}
	value, n, base := roleRatio(v, all, func(p *playerView) *big.Rat {
		if p.uncalibrated {
			return nil
		}
		return div(integer(p.damage), integer(p.aliveMs))
	}, c)
	p := makePart("output", []measure{{"boss_damage", value}}, c.Scoring.MeasureWeights["output"], counts, c)
	counts["baseline_players"] = int64(n)
	if base != nil {
		counts["baseline_damage_num"] = base.Num().Int64()
		counts["baseline_damage_den"] = base.Denom().Int64()
	}
	if p.Tenths.Present {
		p.Baseline = "role"
		p.Provisional = n < c.Scoring.Confidence.MinBaselinePlayers
	}
	return p
}
