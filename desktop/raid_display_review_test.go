package main

import (
	"encoding/json"
	"html/template"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare"
)

func raidDisplayFixture(t *testing.T, suffix, text string) {
	t.Helper()
	if out := os.Getenv("NATIVE_HTML_OUT"); out != "" {
		if err := os.WriteFile(out+suffix, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRaidPageScreenDisplay(t *testing.T) {
	visible := `العربية <img src="x" onerror="alert(1)">&'`
	raw := raidForbiddenDisplayAll() + visible + raidForbiddenDisplayAll()
	screen := raidScreen{LogPath: raw, Error: raw}
	text, err := renderRaidScreen(screen)
	if err != nil {
		t.Fatal(err)
	}
	raidDisplayFixture(t, ".raid-screen-display", text)
	values, err := json.Marshal(map[string]string{"raw": raw, "visible": visible})
	if err != nil {
		t.Fatal(err)
	}
	raidDisplayFixture(t, ".raid-display-values.json", string(values))
	if !strings.Contains(text, `value="`+template.HTMLEscapeString(visible)+`"`) {
		t.Error("displayed log path was not stripped and escaped")
	}
	if !strings.Contains(text, `<bdi>`+template.HTMLEscapeString(visible)+`</bdi>`) {
		t.Error("error was not stripped, escaped and isolated")
	}
	if screen.LogPath != raw || screen.Error != raw {
		t.Error("display modified source paths or diagnostics")
	}
	p := raidPageModel{ComparisonError: raw}
	text, err = renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	raidDisplayFixture(t, ".raid-error-display", text)
	if strings.ContainsAny(text, raidForbiddenDisplayAll()) || !strings.Contains(text, `<bdi>`+template.HTMLEscapeString(visible)+`</bdi>`) {
		t.Error("comparison error was not stripped, escaped and isolated")
	}
	if p.ComparisonError != raw {
		t.Error("display modified comparison error")
	}
}

func TestRaidPageTinyDecimal(t *testing.T) {
	for _, tc := range []struct {
		input *big.Rat
		want  string
	}{
		{nil, "not measured"}, {big.NewRat(0, 1), "0.0"},
		{big.NewRat(1, 25), "<0.1"}, {big.NewRat(-1, 25), ">-0.1"},
		{big.NewRat(1, 999999), "<0.1"}, {big.NewRat(-1, 999999), ">-0.1"},
		{big.NewRat(1, 20), "0.1"}, {big.NewRat(-1, 20), "-0.1"},
		{big.NewRat(1, 10), "0.1"}, {big.NewRat(-1, 10), "-0.1"},
	} {
		if got := raidDecimal(tc.input); got != tc.want {
			t.Errorf("decimal(%v)=%q, want %q", tc.input, got, tc.want)
		}
	}
	p := raidPageModel{Comparison: &compare.Report{Top: []compare.Entry{{Name: "Alia-PvP-EU", Part: &compare.PartComparison{
		Name: "output", Impact: big.NewRat(1, 25), Score: big.NewRat(-1, 25), Median: big.NewRat(0, 1),
	}}}}}
	text, err := renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	raidDisplayFixture(t, ".raid-tiny", text)
	for _, want := range []string{"&lt;0.1 weighted impact", "Part score: &gt;-0.1", "role median: 0.0", "configured weight: not measured"} {
		if !strings.Contains(text, want) {
			t.Errorf("tiny comparison display missing %q", want)
		}
	}
	if p.Comparison.Top[0].Part.Impact.Cmp(big.NewRat(1, 25)) != 0 || p.Comparison.Top[0].Part.Score.Cmp(big.NewRat(-1, 25)) != 0 {
		t.Error("display mutated exact measured values")
	}
}
