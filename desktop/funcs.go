package main

import (
	"fmt"
	"html/template"
	"strings"
)

var funcs = template.FuncMap{
	"lower": strings.ToLower,
	"ago":   ago,
	"title": func(s string) string {
		s = strings.ReplaceAll(strings.ToLower(s), "_", " ")
		w := strings.Fields(s)
		for i, x := range w {
			w[i] = strings.ToUpper(x[:1]) + x[1:]
		}
		return strings.Join(w, " ")
	},
	"inc":   func(n int) int { return n + 1 },
	"coin":  coin,
	"atCap": func(c int) bool { return c >= ahpMaxCopper }, // a total that hit the overflow cap is a lower bound
	"sub":   func(a, b int) int { return a - b },
	"add":   func(a, b int) int { return a + b },
	"commas": func(n int) string {
		s := fmt.Sprint(n)
		for i := len(s) - 3; i > 0; i -= 3 {
			s = s[:i] + "," + s[i:]
		}
		return s
	},
	"dur": func(min int) string {
		if min < 60 {
			return fmt.Sprintf("%dm", min)
		}
		return fmt.Sprintf("%dh %02dm", min/60, min%60)
	},
	"css": func(s string) template.CSS { return template.CSS(s) },
	"scoreColor": func(p int) string {
		switch {
		case p >= 85:
			return "var(--ok)"
		case p >= 60:
			return "var(--warn)"
		}
		return "var(--bad)"
	},
	"procColor": func(p int) string {
		switch {
		case p >= 70:
			return "var(--ok)"
		case p >= 40:
			return "var(--warn)"
		}
		return "var(--bad)"
	},
	"gapColor": func(gap int, equipped bool, status string) string {
		switch {
		case status == "unknown" || gap < 0:
			return "#4a5468"
		case !equipped || gap >= 40:
			return "var(--bad)"
		case gap >= 12:
			return "var(--warn)"
		}
		return "var(--ok)"
	},
	"join": strings.Join,
	"icon": func(area string) template.HTML {
		return template.HTML(icons[area])
	},
	"bars": func(impact float64) []bool {
		on := int(impact/20 + 0.5)
		b := make([]bool, 5)
		for i := range b {
			b[i] = i < on
		}
		return b
	},
	"short": func(s string) string {
		r := strings.NewReplacer("&nbsp;", " ", "&lt;", "<", "&gt;", ">", "&amp;", "&", "&#39;", "'", "&quot;", "\"")
		s = r.Replace(s)
		if i := strings.Index(s, "Objective:"); i > 0 {
			s = s[:i]
		}
		if len(s) > 240 {
			s = s[:237] + "..."
		}
		return strings.TrimSpace(s)
	},
}

// Inline SVG icons (emoji fonts are unreliable on Windows browsers).
var icons = map[string]string{
	"Build": `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5V5a2 2 0 0 1 2-2h12v15H6a2 2 0 0 0-2 2z"/><path d="M8 7h6M8 11h6"/></svg>`,
	"Gear":  `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z"/></svg>`,
	"Play":  `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 17.5L3 6V3h3l11.5 11.5"/><path d="M13 19l6-6M16 16l4 4M19 21l2-2"/></svg>`,
	"Next":  `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v4M12 17v4M3 12h4M17 12h4M6 6l2.5 2.5M15.5 15.5L18 18M6 18l2.5-2.5M15.5 8.5L18 6"/></svg>`,
}
