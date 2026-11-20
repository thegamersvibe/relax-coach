package score

import (
	"math/big"
	"sort"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Score judges the fights of ONE run, never a mixture of instances. It returns
// players in ID order; Rank is a separate competition rank among normal scores.
// The caller owns configVersion and lists unmatched reports through MatchPrep.
// Unknown runs and index-only fights are not judged; dungeon scores have no rank.
func Score(fights []model.Fight, deaths []model.Death, reports []model.PrepReport, c cfg.Config) []model.NightScore {
	c = c.Snapshot()
	names := []string{}
	players := map[string]model.Actor{}
	kind := model.RunUnknown
	for _, f := range fights {
		k, _ := c.InstanceKind(f.InstanceID, int64(f.GroupSize))
		if k != model.RunUnknown {
			kind = k
		}
		for _, p := range f.Players {
			names = append(names, p.Name)
			players[p.ID] = p
		}
	}
	matched, _ := MatchPrep(reports, names)
	ids := []string{}
	for id := range players {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]model.NightScore, len(ids))
	index := map[string]int{}
	sums := make([]nightTotals, len(ids))
	for i, id := range ids {
		index[id] = i
		out[i].PlayerID = id
	}
	for _, f := range fights {
		if len(f.Events) == 0 || kind == model.RunUnknown {
			continue
		}
		all := views(f, c)
		for _, id := range ids {
			v := all[id]
			if v == nil || (v.role != "tank" && v.role != "healer" && v.role != "dps") {
				continue
			}
			fs := model.FightScore{FightID: f.ID, PlayerID: id, Role: v.role}
			coverage := integer(0)
			d := duration(f)
			if d.Present && d.V < c.Scoring.Confidence.LowConfidenceFightMs {
				fs.Flags = append(fs.Flags, "lowConfidenceFight")
			}
			if f.Partial {
				fs.Flags = append(fs.Flags, "partialFight")
			}
			if profile, ok := c.Profile(f.Specs[id]); !ok || profile.Uncalibrated {
				fs.Flags = append(fs.Flags, "uncalibratedSpec")
			}
			if !f.Partial && d.Present && d.V > 0 && v.observed {
				parts := []partResult{output(f, v, all, deaths, c), activity(v, all, c), mechanics(f, v, deaths, c), utility(f, v, c), preparation(f, matched[v.actor.Name], c)}
				fs.Tenths, fs.Coverage, coverage = combine(parts, partWeights(v.role, c), c)
				for _, p := range parts {
					fs.Parts = append(fs.Parts, p.Part)
				}
			} else {
				for _, name := range []string{"output", "activity", "mechanics", "utility", "preparation"} {
					fs.Parts = append(fs.Parts, model.Part{Name: name})
				}
			}
			i := index[id]
			out[i].Fights = append(out[i].Fights, fs)
			sums[i].include(f, fs, coverage, c)
			for _, p := range fs.Parts {
				if p.Tenths.Present && p.Baseline != "" {
					out[i].Baseline = p.Baseline
				}
			}
		}
	}
	coverage := make([]*big.Rat, len(out))
	for i := range out {
		out[i].Tenths, out[i].Coverage, coverage[i] = sums[i].result(c)
	}
	if kind == model.RunRaid {
		rank(out, coverage, c)
	}
	// Invalid generated metadata is a programming error, never publishable output.
	if err := Validate(out); err != nil {
		panic(err)
	}
	return out
}
