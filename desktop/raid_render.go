package main

import (
	"html/template"
	"strings"
)

// Display-only: never change names used by parsing, scoring or matching.
func raidDisplayName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '\u200b' && r <= '\u200f' || r >= '\u202a' && r <= '\u202e' || r >= '\u2066' && r <= '\u2069' {
			return -1
		}
		return r
	}, s)
}

var raidTemplate = template.Must(template.New("raid.html").Funcs(template.FuncMap{"raidName": raidDisplayName}).ParseFS(assets, "raid.html"))

func renderRaidBody(p raidPageModel) (string, error) {
	var out strings.Builder
	err := raidTemplate.ExecuteTemplate(&out, "raid-body", p)
	return out.String(), err
}
func renderRaidShell() (string, error) {
	var out strings.Builder
	err := raidTemplate.ExecuteTemplate(&out, "raid-shell", nil)
	return out.String(), err
}
