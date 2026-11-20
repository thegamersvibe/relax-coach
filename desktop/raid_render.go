package main

import (
	"html/template"
	"strings"
)

var raidTemplate = template.Must(template.New("raid.html").ParseFS(assets, "raid.html"))

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
