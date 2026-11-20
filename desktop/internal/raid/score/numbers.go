package score

import (
	"math/big"
	"strconv"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Config numbers are interpreted as their decimal spelling, not binary floats.
// All interpolation, weighting and half-away rounding after this boundary is
// rational/integer arithmetic. Never mutate a caller's Rat.
func integer(n int64) *big.Rat { return new(big.Rat).SetInt64(n) }
func decimal(n float64) *big.Rat {
	r, _ := new(big.Rat).SetString(strconv.FormatFloat(n, 'f', -1, 64))
	return r
}
func add(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
func mul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
func div(a, b *big.Rat) *big.Rat {
	if b.Sign() == 0 {
		return integer(0)
	}
	return new(big.Rat).Quo(a, b)
}
func sub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func clamp(a *big.Rat) *big.Rat {
	if a.Sign() < 0 {
		return integer(0)
	}
	if a.Cmp(integer(100)) > 0 {
		return integer(100)
	}
	return a
}
func round(a *big.Rat) int64 {
	n := new(big.Int).Abs(a.Num())
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(n, a.Denom(), rem)
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(a.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if a.Sign() < 0 {
		q.Neg(q)
	}
	return q.Int64()
}
func points(a *big.Rat, c cfg.Config) model.Opt {
	if a == nil || c.Scoring.RoundingStep == nil {
		return model.Opt{}
	}
	step := decimal(*c.Scoring.RoundingStep)
	if step.Sign() <= 0 {
		return model.Opt{}
	}
	quantized := mul(integer(round(div(clamp(a), step))), step)
	return model.Some(round(mul(clamp(quantized), integer(10))))
}

type measure struct {
	name  string
	value *big.Rat
}

func makePart(name string, measures []measure, weights map[string]float64, counts map[string]int64, c cfg.Config) model.Part {
	p := model.Part{Name: name, Evidence: model.Evidence{Text: "Measured values only; unavailable measures are excluded.", Counts: counts}}
	total, weighted := integer(0), integer(0)
	for _, m := range measures {
		p.Evidence.Counts["measured."+m.name] = 0
		v := points(m.value, c)
		if !v.Present || weights[m.name] <= 0 {
			continue
		}
		p.Evidence.Counts["measured."+m.name] = 1
		p.Evidence.Counts["points_tenths."+m.name] = v.V
		w := decimal(weights[m.name])
		total = add(total, w)
		weighted = add(weighted, mul(w, div(integer(v.V), integer(10))))
	}
	if total.Sign() > 0 {
		p.Coverage = int(round(total))
		p.Tenths = points(div(weighted, total), c)
	}
	return p
}

func partWeights(role string, c cfg.Config) map[string]float64 {
	var w cfg.PartWeights
	switch role {
	case "dps":
		w = c.Scoring.Weights.DPS
	case "healer":
		w = c.Scoring.Weights.Healer
	case "tank":
		w = c.Scoring.Weights.Tank
	}
	return map[string]float64{"output": w.Output, "activity": w.Activity, "mechanics": w.Mechanics, "utility": w.Utility, "preparation": w.Preparation}
}

func combine(parts []model.Part, weights map[string]float64, c cfg.Config) (model.Opt, int) {
	measured, weighted := integer(0), integer(0)
	for _, p := range parts {
		if !p.Tenths.Present {
			continue
		}
		w := mul(decimal(weights[p.Name]), div(integer(int64(p.Coverage)), integer(100)))
		measured = add(measured, w)
		weighted = add(weighted, mul(w, div(integer(p.Tenths.V), integer(10))))
	}
	coverage := int(round(measured))
	if !enough(coverage, c.Scoring.Confidence.ProvisionalMinCoverage) || measured.Sign() == 0 {
		return model.Opt{}, coverage
	}
	return points(div(weighted, measured), c), coverage
}
