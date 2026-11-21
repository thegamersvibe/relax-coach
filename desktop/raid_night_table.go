package main

import (
	"strconv"
	"strings"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

type raidNightPartView struct{ Name, Weight, Score string }

// NightRoleParts owns all arithmetic and role selection. Its empty-role/Split
// states carry the reason; this is display wording only, not another scorer.
func raidNightParts(summary score.NightSummary, c cfg.Config) ([]raidNightPartView, string) {
	if summary.Role == "" {
		if summary.Split {
			return nil, "role split evenly, no part comparison"
		}
		return nil, "no scored fights, no part comparison"
	}
	w := c.Scoring.Weights.DPS
	if summary.Role == "tank" {
		w = c.Scoring.Weights.Tank
	} else if summary.Role == "healer" {
		w = c.Scoring.Weights.Healer
	}
	rows := []raidNightPartView{}
	for _, part := range []struct {
		name   string
		weight float64
	}{
		{"output", w.Output}, {"activity", w.Activity}, {"mechanics", w.Mechanics}, {"utility", w.Utility}, {"preparation", w.Preparation},
	} {
		value := "not measured"
		if points := summary.Parts[part.name]; points != nil {
			value = raidDecimal(points)
		}
		rows = append(rows, raidNightPartView{Name: strings.ToUpper(part.name[:1]) + part.name[1:], Weight: strconv.FormatFloat(part.weight, 'f', -1, 64) + "%", Score: value})
	}
	return rows, ""
}
