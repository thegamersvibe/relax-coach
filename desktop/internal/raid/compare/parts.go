package compare

import (
	"math/big"
	"sort"
	"strconv"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

// PartComparison keeps exact points and the configured percentage weight.
// Impact is (Score - Median) * Weight, without renormalizing missing parts.
// Round only for display (Rat.FloatString(1)), never before selection.
type PartComparison struct {
	Name, Role                    string
	Score, Median, Weight, Impact *big.Rat
}

func explain(player score.NightSummary, ranked map[string]score.NightSummary, c cfg.Config, top bool) (*PartComparison, string) {
	if player.Split {
		return nil, "role split evenly, no part comparison"
	}
	if player.Role == "" {
		return nil, "no scored fights, no part comparison"
	}
	weights := roleWeights(c, player.Role)
	var best *PartComparison
	for name, value := range player.Parts {
		weight, ok := weights[name]
		if !ok {
			continue
		}
		var values []*big.Rat
		for _, peer := range ranked {
			if peer.Role == player.Role && peer.Parts[name] != nil {
				values = append(values, peer.Parts[name])
			}
		}
		if len(values) < 2 {
			continue
		}
		sort.Slice(values, func(i, j int) bool { return values[i].Cmp(values[j]) < 0 })
		mid := len(values) / 2
		median := new(big.Rat).Set(values[mid])
		if len(values)%2 == 0 {
			median.Quo(new(big.Rat).Add(values[mid-1], values[mid]), big.NewRat(2, 1))
		}
		impact := new(big.Rat).Mul(new(big.Rat).Sub(value, median), weight)
		candidate := &PartComparison{Name: name, Role: player.Role, Score: new(big.Rat).Set(value), Median: median, Weight: weight, Impact: impact}
		if best == nil || betterPart(candidate, best, top) {
			best = candidate
		}
	}
	if best == nil {
		return nil, "no comparison in his role"
	}
	if (top && best.Impact.Sign() >= 0) || (!top && best.Impact.Sign() <= 0) {
		return nil, "no single part explains it"
	}
	return best, ""
}

func betterPart(a, b *PartComparison, top bool) bool {
	if cmp := a.Impact.Cmp(b.Impact); cmp != 0 {
		if top {
			return cmp < 0
		}
		return cmp > 0
	}
	if cmp := a.Weight.Cmp(b.Weight); cmp != 0 {
		return cmp > 0
	}
	return a.Name < b.Name
}

func roleWeights(c cfg.Config, role string) map[string]*big.Rat {
	var w cfg.PartWeights
	switch role {
	case "dps":
		w = c.Scoring.Weights.DPS
	case "healer":
		w = c.Scoring.Weights.Healer
	case "tank":
		w = c.Scoring.Weights.Tank
	default:
		return nil
	}
	out := map[string]*big.Rat{}
	for name, v := range map[string]float64{"output": w.Output, "activity": w.Activity, "mechanics": w.Mechanics, "utility": w.Utility, "preparation": w.Preparation} {
		// Interpret the configured decimal, not the float's binary approximation.
		if r, ok := new(big.Rat).SetString(strconv.FormatFloat(v, 'f', -1, 64)); ok {
			out[name] = r
		}
	}
	return out
}
