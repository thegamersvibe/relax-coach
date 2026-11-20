// Package score computes raid-one scores from model values and a configuration
// snapshot. It performs no I/O and imports no parser or other engine.
package score

import "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"

// MatchPrep matches whole logged names exactly before trying ASCII lowercase.
// Reports with no unique match are returned, in input order, for the page to list.
// No realm stripping, whitespace trimming, or Unicode normalization is performed.
func MatchPrep(reports []model.PrepReport, names []string) (map[string][]model.PrepReport, []model.PrepReport) {
	exact := make(map[string]bool)
	folded := make(map[string]string)
	ambiguous := make(map[string]bool)
	for _, name := range names {
		if exact[name] {
			continue
		}
		exact[name] = true
		key := asciiLower(name)
		if _, ok := folded[key]; ok {
			ambiguous[key] = true
		}
		folded[key] = name
	}
	matched := make(map[string][]model.PrepReport)
	var unmatched []model.PrepReport
	for _, report := range reports {
		name := report.Name
		if !exact[name] {
			key := asciiLower(name)
			var ok bool
			name, ok = folded[key]
			if !ok || ambiguous[key] {
				unmatched = append(unmatched, report)
				continue
			}
		}
		matched[name] = append(matched[name], report)
	}
	return matched, unmatched
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
