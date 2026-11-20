package main

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

type raidFightView struct {
	Name, Outcome, Duration, Role, Score string
	Coverage                             int
	Flags                                []string
	Parts                                []raidPartView
}
type raidPartView struct {
	Name, Weight, Score, Against string
	Coverage                     int
	Provisional                  bool
	Evidence                     model.Evidence
	Measures                     []raidMeasureView
}
type raidMeasureView struct{ Name, Score string }

func raidPlayerEvidence(n model.NightScore, fights []model.Fight, c cfg.Config) ([]raidFightView, []string) {
	byID := map[string]model.Fight{}
	for _, f := range fights {
		byID[f.ID] = f
	}
	var out []raidFightView
	flags := map[string]bool{}
	for _, s := range n.Fights {
		f := byID[s.FightID]
		v := raidFightView{Name: f.Name, Outcome: "unknown outcome", Duration: "unknown duration", Role: s.Role, Score: raidTenths(s.Tenths, "not enough data"), Coverage: s.Coverage, Flags: s.Flags}
		if !f.Boss {
			v.Name = "trash"
			v.Duration = (time.Duration(f.EndMs-f.StartMs) * time.Millisecond).String()
		}
		if f.DurationMs.Present {
			v.Duration = (time.Duration(f.DurationMs.V) * time.Millisecond).String()
		}
		if f.Success.Present {
			if f.Success.V == 1 {
				v.Outcome = "kill"
			} else {
				v.Outcome = "wipe"
			}
		}
		w := c.Scoring.Weights.DPS
		if s.Role == "tank" {
			w = c.Scoring.Weights.Tank
		} else if s.Role == "healer" {
			w = c.Scoring.Weights.Healer
		}
		weights := map[string]float64{"output": w.Output, "activity": w.Activity, "mechanics": w.Mechanics, "utility": w.Utility, "preparation": w.Preparation}
		for _, part := range s.Parts {
			pv := raidPartView{Name: part.Name, Weight: strconv.FormatFloat(weights[part.Name], 'f', -1, 64) + "%", Score: raidTenths(part.Tenths, "not measured"), Coverage: part.Coverage, Provisional: part.Provisional, Evidence: part.Evidence}
			switch part.Baseline {
			case "role":
				pv.Against = "role median, this fight"
			case "own":
				pv.Against = "own report / counts"
			case "guild":
				pv.Against = "guild"
			default:
				pv.Against = "no baseline"
			}
			names := []string{}
			for key := range part.Evidence.Counts {
				if strings.HasPrefix(key, "measured.") {
					names = append(names, strings.TrimPrefix(key, "measured."))
				}
			}
			sort.Strings(names)
			for _, name := range names {
				value, present := part.Evidence.Counts["points_tenths."+name]
				measured := part.Evidence.Counts["measured."+name] == 1
				pv.Measures = append(pv.Measures, raidMeasureView{Name: name, Score: raidTenths(model.Opt{V: value, Present: measured && present}, "not measured")})
			}
			v.Parts = append(v.Parts, pv)
		}
		for _, flag := range s.Flags {
			flags[flag] = true
		}
		out = append(out, v)
	}
	all := []string{}
	for flag := range flags {
		all = append(all, flag)
	}
	sort.Strings(all)
	return out, all
}
