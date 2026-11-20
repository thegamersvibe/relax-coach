package score

import (
	"math/big"
	"slices"
	"sort"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

type groupTotal struct{ weight, coverage, scored, points big.Rat }
type nightTotals struct{ boss, trash groupTotal }

func (n *nightTotals) include(f model.Fight, fs model.FightScore, coverage *big.Rat, c cfg.Config) {
	g := &n.trash
	if f.Boss {
		g = &n.boss
	}
	w := fightWeight(f, fs, c)
	g.weight.Add(&g.weight, w)
	covered := mul(w, coverage)
	g.coverage.Add(&g.coverage, covered)
	if fs.Tenths.Present {
		g.scored.Add(&g.scored, covered)
		g.points.Add(&g.points, mul(covered, div(integer(fs.Tenths.V), integer(10))))
	}
}
func (n *nightTotals) result(c cfg.Config) (model.Opt, int, *big.Rat) {
	boss, trash := nightShares(n.trash.weight.Sign() > 0, c)
	coverage := add(mul(boss, div(&n.boss.coverage, &n.boss.weight)), mul(trash, div(&n.trash.coverage, &n.trash.weight)))
	cov := floor(coverage)
	if !enough(coverage, c.Scoring.Confidence.ProvisionalMinCoverage) || (n.boss.scored.Sign() == 0 && n.trash.scored.Sign() == 0) {
		return model.Opt{}, cov, coverage
	}
	// An absent group contributes no invented zero score: normalize only its
	// score share, while coverage still exposes any missing boss evidence.
	total, weight := integer(0), integer(0)
	if n.boss.scored.Sign() > 0 {
		total = add(total, mul(boss, div(&n.boss.points, &n.boss.scored)))
		weight = add(weight, boss)
	}
	if n.trash.scored.Sign() > 0 {
		total = add(total, mul(trash, div(&n.trash.points, &n.trash.scored)))
		weight = add(weight, trash)
	}
	if weight.Sign() == 0 {
		return model.Opt{}, cov, coverage
	}
	return points(div(total, weight), c), cov, coverage
}

// nightShares interprets all configured shares as authored decimals, including
// the complement of MinBossShare. Both consumers normalize missing score groups.
func nightShares(hasTrash bool, c cfg.Config) (boss, trash *big.Rat) {
	trash = integer(0)
	if hasTrash {
		trash = decimal(c.Scoring.Night.TrashShare)
		for _, cap := range []*big.Rat{decimal(c.Scoring.Night.MaxTrashShare), sub(integer(1), decimal(c.Scoring.Night.MinBossShare))} {
			if cap.Cmp(trash) < 0 {
				trash = cap
			}
		}
	}
	return sub(integer(1), trash), trash
}

// fightWeight is shared by score aggregation and the transient comparison.
func fightWeight(f model.Fight, fs model.FightScore, c cfg.Config) *big.Rat {
	w := integer(1)
	if f.Boss {
		w = decimal(c.Scoring.Night.KillWeight)
		if f.Success == model.Some(0) {
			w = decimal(c.Scoring.Night.WipeWeight)
		}
	}
	if slices.Contains(fs.Flags, "lowConfidenceFight") {
		w = mul(w, decimal(c.Scoring.Confidence.ShortFightWeight))
	}
	return w
}

func enough(coverage *big.Rat, threshold float64) bool {
	return coverage.Cmp(mul(decimal(threshold), integer(100))) >= 0
}
func rank(out []model.NightScore, coverage []*big.Rat, c cfg.Config) {
	var eligible []int
	for i, n := range out {
		if n.Tenths.Present && enough(coverage[i], c.Scoring.Confidence.MinCoverage) {
			eligible = append(eligible, i)
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool { return out[eligible[i]].Tenths.V > out[eligible[j]].Tenths.V })
	var r int64
	for i, j := range eligible {
		if i == 0 || out[j].Tenths.V != out[eligible[i-1]].Tenths.V {
			r = int64(i + 1)
		}
		out[j].Rank = model.Some(r)
	}
}
