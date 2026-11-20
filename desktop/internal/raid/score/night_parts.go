package score

import (
	"errors"
	"math/big"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// NightSummary is a transient comparison input, not a new stored score. Parts
// are exact points (not tenths), owned by the caller; absent means unmeasured.
// Split means equal positive leading role weights; no positive weight is no data.
type NightSummary struct {
	Role  string
	Split bool
	Parts map[string]*big.Rat
}

// NightRoleParts summarizes one player's fight scores using the original fight
// metadata. The caller supplies the same configuration used for scoring.
func NightRoleParts(fights []model.Fight, night model.NightScore, c cfg.Config) (NightSummary, error) {
	c = c.Snapshot()
	out := NightSummary{Parts: map[string]*big.Rat{}}
	byID := map[string]model.Fight{}
	for _, f := range fights {
		byID[f.ID] = f
	}
	roles := map[string]*big.Rat{}
	for _, fs := range night.Fights {
		if _, ok := byID[fs.FightID]; !ok {
			return NightSummary{}, errors.New("score: night summary requires original fight metadata")
		}
		if !fs.Tenths.Present {
			continue
		}
		w := fightWeight(byID[fs.FightID], fs, c)
		if roles[fs.Role] == nil {
			roles[fs.Role] = integer(0)
		}
		roles[fs.Role] = add(roles[fs.Role], w)
	}
	best := integer(0)
	for role, w := range roles {
		if w.Sign() <= 0 {
			continue
		}
		switch w.Cmp(best) {
		case 1:
			out.Role, out.Split, best = role, false, w
		case 0:
			out.Split = true
		}
	}
	if out.Split {
		out.Role = ""
	}
	if out.Role == "" {
		return out, nil
	}
	sums := map[string]*nightTotals{}
	for _, fs := range night.Fights {
		if fs.Role != out.Role {
			continue
		}
		f := byID[fs.FightID]
		w := fightWeight(f, fs, c)
		for _, p := range fs.Parts {
			if !p.Tenths.Present {
				continue
			}
			if sums[p.Name] == nil {
				sums[p.Name] = &nightTotals{}
			}
			g := &sums[p.Name].trash
			if f.Boss {
				g = &sums[p.Name].boss
			}
			g.weight.Add(&g.weight, w)
			g.points.Add(&g.points, mul(w, div(integer(p.Tenths.V), integer(10))))
		}
	}
	for name, sum := range sums {
		boss, trash := nightShares(sum.trash.weight.Sign() > 0, c)
		total, weight := integer(0), integer(0)
		for _, group := range []struct {
			g     *groupTotal
			share *big.Rat
		}{{&sum.boss, boss}, {&sum.trash, trash}} {
			if group.g.weight.Sign() > 0 {
				total = add(total, mul(group.share, div(&group.g.points, &group.g.weight)))
				weight = add(weight, group.share)
			}
		}
		if weight.Sign() > 0 {
			out.Parts[name] = div(total, weight)
		}
	}
	return out, nil
}
