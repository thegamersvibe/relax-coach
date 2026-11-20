package main

import (
	"encoding/json"
	"html/template"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestRaidPageNamesIsolated(t *testing.T) {
	const forbidden = "\u200b\u200c\u200d\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069"
	const visible = `العربية <img src="x" onerror="alert(1)">&'`
	var expected []string
	name := func(prefix string) string {
		expected = append(expected, prefix+visible)
		return forbidden + prefix + forbidden + visible + forbidden
	}
	p := raidPageModel{Title: name("Instance"), Logger: name("Logger"),
		Ranked:      []raidPlayerView{{Name: name("Ranked"), Fights: []raidFightView{{Name: name("Fight"), Parts: []raidPartView{{Name: name("Part"), Measures: []raidMeasureView{{Name: name("Measure")}}}}}}}},
		Provisional: []raidPlayerView{{Name: name("Provisional")}},
		Unscored:    []raidPlayerView{{Name: name("Unscored")}},
		Unmatched:   []model.PrepReport{{Name: name("Unmatched")}},
		Deaths:      []raidDeathView{{Player: name("Death"), Fight: name("DeathFight"), KilledBy: name("Killer"), Owners: []string{name("Owner")}}},
		Wipes:       []raidWipeView{{HasFirst: true, Fight: name("WipeFight"), First: raidDeathView{Player: name("First"), KilledBy: name("WipeKiller")}, Following: []raidDeathView{{Player: name("Following")}}}},
		Comparison: &compare.Report{Ranked: 4, Officer: name("Officer"),
			Top:         []compare.Entry{{Name: name("Top"), Verdict: "agreement", Part: &compare.PartComparison{Name: name("ComparisonPart"), Score: big.NewRat(20, 1), Median: big.NewRat(30, 1), Weight: big.NewRat(35, 100), Impact: big.NewRat(-7, 2)}}},
			Bottom:      []compare.Entry{{Name: name("Bottom"), Status: "not in this night"}},
			ScoreTop:    []compare.Entry{{LoggedName: name("ScoreTop")}},
			ScoreBottom: []compare.Entry{{LoggedName: name("ScoreBottom")}}},
	}
	text, err := renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	// Emit even on a failing assertion: jsdom must challenge the actual renderer,
	// not a hand-written sanitized fixture, during RED as well as GREEN.
	if out := os.Getenv("NATIVE_HTML_OUT"); out != "" {
		data, err := json.Marshal(expected)
		if err != nil {
			t.Fatal(err)
		}
		for suffix, body := range map[string][]byte{".raid-bidi": []byte(text), ".raid-bidi-names.json": data} {
			if err := os.WriteFile(out+suffix, body, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if strings.ContainsAny(text, forbidden) {
		t.Fatal("name contains a zero-width or bidi control in display")
	}
	for _, want := range expected {
		if !strings.Contains(text, "<bdi>"+template.HTMLEscapeString(want)+"</bdi>") {
			t.Fatal("name is missing, changed, unescaped or not isolated")
		}
	}
	if !strings.ContainsAny(p.Logger, forbidden) || !strings.ContainsAny(p.Comparison.Officer, forbidden) {
		t.Fatal("display rewrote an engine name")
	}
}
