package main

import (
	"html/template"
	"math/big"
	"strconv"
	"strings"
)

// Display-only: never change names used by parsing, scoring or matching.
func raidDisplayName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '\u200b' && r <= '\u200f' || r >= '\u202a' && r <= '\u202e' || r >= '\u2066' && r <= '\u2069' || r == '\u061c' || r == '\ufeff' || r >= '\u2060' && r <= '\u2064' || r == '\u00ad' || r == '\u180e' || r == '\u115f' || r == '\u1160' || r == '\u3164' || r == '\uffa0' || r == '\u034f' || r == '\u17b4' || r == '\u17b5' || r >= '\U000e0000' && r <= '\U000e007f' {
			return -1
		}
		return r
	}, s)
}

func raidDecimal(v *big.Rat) string {
	if v == nil {
		return "not measured"
	}
	text := v.FloatString(1)
	// Preserve the sign and the fact of a measurement below display precision.
	if text == "-0.0" {
		return ">-0.1"
	}
	if text == "0.0" && v.Sign() > 0 {
		return "<0.1"
	}
	return text
}
func raidPercent(v *big.Rat) string {
	if v == nil {
		return "not measured"
	}
	percent := new(big.Rat).Mul(v, big.NewRat(100, 1))
	value, _ := percent.Float64()
	return strconv.FormatFloat(value, 'f', -1, 64) + "%"
}
func raidTemplateFuncs() map[string]any {
	return map[string]any{"raidName": raidDisplayName, "raidDecimal": raidDecimal, "raidPercent": raidPercent}
}

var raidTemplate = template.Must(template.New("raid.html").Funcs(raidTemplateFuncs()).ParseFS(assets, "raid.html"))

func renderRaidBody(p raidPageModel) (string, error) {
	var out strings.Builder
	err := raidTemplate.ExecuteTemplate(&out, "raid-body", p)
	return out.String(), err
}
func renderRaidShell() (string, error) { return renderRaidScreen(raidScreen{}) }
func renderRaidScreen(p raidScreen) (string, error) {
	var out strings.Builder
	err := raidTemplate.ExecuteTemplate(&out, "raid-screen", p)
	return out.String(), err
}
