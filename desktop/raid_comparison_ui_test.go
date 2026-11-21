package main

import (
	"html/template"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestRaidPageComparisonPresentation(t *testing.T) {
	const overlap = "Fewer than 10 players are ranked: the score's top 5 and bottom 5 overlap."
	for _, n := range []int{4, 9, 10, 11} {
		report := compare.Report{Ranked: n, Top: []compare.Entry{{Name: "Alia-PvP-EU", Verdict: "disagreement", Rank: model.Some(4), Part: &compare.PartComparison{Name: "output", Role: "dps", Score: big.NewRat(125, 3), Median: big.NewRat(150, 3), Weight: big.NewRat(35, 100), Impact: big.NewRat(-35, 12)}}}}
		text, err := renderRaidBody(raidPageModel{Comparison: &report})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, overlap) != (n < 10) {
			t.Fatalf("overlap note wrong for %d ranked", n)
		}
		for _, want := range []string{"output", "dps", "41.7", "50.0", "-2.9", "35%"} {
			if !strings.Contains(text, want) {
				t.Fatalf("comparison field not rendered: %q", want)
			}
		}
	}
	for _, reason := range []string{"no scored fights, no part comparison", "no single part explains it", "role split evenly, no part comparison", "no comparison in his role"} {
		report := compare.Report{Ranked: 4, Top: []compare.Entry{{Name: "Alia-PvP-EU", Verdict: "disagreement", PartReason: reason}}}
		text, err := renderRaidBody(raidPageModel{Comparison: &report})
		if err != nil || !strings.Contains(text, reason) {
			t.Fatalf("comparison reason not passed through: %q", reason)
		}
	}
	report := compare.Report{Ranked: 3, NoReportReason: "fewer than 4 ranked players"}
	text, err := renderRaidBody(raidPageModel{Comparison: &report})
	if err != nil || !strings.Contains(text, report.NoReportReason) || !strings.Contains(text, overlap) || strings.Contains(text, "Top picks") {
		t.Fatal("no-report result lost its reason or showed a verdict")
	}
}

func TestRaidPageEscapesAllNames(t *testing.T) {
	// Each location has a distinct prefix, including the logger, every score
	// group, the opened details, deaths/wipes, unmatched prep and comparisons.
	const special = `<img src="x" onerror="alert(1)">&'`
	var names []string
	name := func(prefix string) string { s := prefix + special; names = append(names, s); return s }
	p := raidPageModel{Logger: name("Logger"),
		Ranked:      []raidPlayerView{{Name: name("Ranked")}},
		Provisional: []raidPlayerView{{Name: name("Provisional")}},
		Unscored:    []raidPlayerView{{Name: name("Unscored")}},
		Unmatched:   []model.PrepReport{{Name: name("Unmatched")}},
		Deaths:      []raidDeathView{{Player: name("Death"), Owners: []string{name("Owner")}}},
		Wipes:       []raidWipeView{{HasFirst: true, First: raidDeathView{Player: name("First")}, Following: []raidDeathView{{Player: name("Following")}}}},
		Comparison: &compare.Report{Ranked: 4, Officer: name("Officer"),
			Top:         []compare.Entry{{Name: name("Top"), Verdict: "agreement"}},
			Bottom:      []compare.Entry{{Name: name("Bottom"), Status: "not in this night"}},
			ScoreTop:    []compare.Entry{{LoggedName: name("ScoreTop")}},
			ScoreBottom: []compare.Entry{{LoggedName: name("ScoreBottom")}}},
	}
	text, err := renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range names {
		if strings.Contains(text, s) || !strings.Contains(text, template.HTMLEscapeString(s)) {
			t.Fatal("name missing or not escaped")
		}
	}
	if out := os.Getenv("NATIVE_HTML_OUT"); out != "" {
		if err := os.WriteFile(out+".raid-escape", []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
