package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestRaidPageNightTable(t *testing.T) {
	for _, role := range []string{"dps", "tank", "healer"} {
		t.Run(role, func(t *testing.T) {
			r := raidPageFixture()
			for _, a := range r.Roster {
				r.Fights[0].Roles[a.ID] = role
			}
			c := raidPrepConfig(t)
			page, err := buildRaidPage(r, logfmt.ParseReport{}, nil, c)
			if err != nil {
				t.Fatal(err)
			}
			html, err := renderRaidBody(page)
			if err != nil {
				t.Fatal(err)
			}
			start := strings.Index(html, `<table class="farm-table raid-night-parts">`)
			if start < 0 {
				t.Fatal("missing night part table")
			}
			end := strings.Index(html[start:], "</table>") + start
			table := html[start:end]
			if strings.Count(table, "<tr>") != 6 || strings.Contains(table, "Coverage") || strings.Contains(table, "Against") || strings.Contains(table, "Baseline") {
				t.Fatal("night table shape")
			}
			if strings.Index(html[start:], "Training Colossus") < end-start {
				t.Fatal("night table not above fights")
			}
			if !strings.Contains(html, "THE NIGHT, by part (weighted over his fights as "+role+")") {
				t.Fatal("night role title")
			}
			weights := c.Scoring.Weights.DPS
			if role == "tank" {
				weights = c.Scoring.Weights.Tank
			}
			if role == "healer" {
				weights = c.Scoring.Weights.Healer
			}
			nights := score.Score(r.Fights, nil, nil, c)
			for _, n := range nights {
				summary, err := score.NightRoleParts(r.Fights, n, c)
				if err != nil {
					t.Fatal(err)
				}
				for part, w := range map[string]float64{"output": weights.Output, "activity": weights.Activity, "mechanics": weights.Mechanics, "utility": weights.Utility, "preparation": weights.Preparation} {
					value := "not measured"
					if points := summary.Parts[part]; points != nil {
						value = points.FloatString(1)
					}
					want := fmt.Sprintf("<bdi>%s</bdi></td><td>%g%%</td><td>%s</td>", strings.ToUpper(part[:1])+part[1:], w, value)
					if !strings.Contains(html, want) {
						t.Fatalf("missing engine night row %s", want)
					}
				}
			}
		})
	}
}

func TestRaidPageNightNoScoredRole(t *testing.T) {
	r := raidPageFixture()
	r.Roster = []model.Actor{{ID: "unscored", Name: "Faye-PvP-EU"}}
	p, err := buildRaidPage(r, logfmt.ParseReport{}, nil, raidPrepConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "no scored fights, no part comparison") || strings.Contains(html, `class="farm-table raid-night-parts"`) {
		t.Fatal("missing no-role reason or invented table")
	}
}
