// Package prepread reads Relax Raid preparation SavedVariables as data only.
// It does not execute Lua, modify saves, score players, or infer missing data.
package prepread

import (
	"errors"
	"io"
	"os"
	"sort"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Report contains no input text or player names. Warnings has one reason when
// the file/store is refused; Skipped counts distinct stored bodies by reason,
// including valid bodies outside the requested night. Multiple references to
// one invalid body increment its reason only once.
type Report struct {
	Warnings []string
	Skipped  map[string]int
}

// ReadFile reads only the caller-selected account RelaxRaid.lua. No directory
// search, fallback, cache, or write is performed. Only regular non-symlink files
// are accepted; both stat and actual bytes read are bounded. I/O errors are
// returned separately from invalid-data diagnostics.
func ReadFile(path string, firstFightStartS, lastFightEndS int64) ([]model.PrepReport, Report, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, Report{}, err
	}
	if !before.Mode().IsRegular() {
		return nil, Report{}, errors.New("prep save is not a regular file")
	}
	if before.Size() > int64(limits.MaxFileBytes) {
		return nil, Report{Warnings: []string{"file too large"}}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, Report{}, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, Report{}, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, Report{}, errors.New("prep save changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(limits.MaxFileBytes)+1))
	if err != nil {
		return nil, Report{}, err
	}
	reports, summary := Parse(data, firstFightStartS, lastFightEndS)
	return reports, summary, nil
}

// Parse takes the first fight start and last fight end in Unix SERVER SECONDS,
// not model.Fight's milliseconds. The caller owns PC/server clock alignment.
// It selects bodies with a receipt in the inclusive interval [first-2h,last].
// Stamped is the earliest of ALL references to that body, even when that first
// receipt falls outside the interval. Identity is (night,key,body index), not
// equality of body bytes: an A-B-A history keeps both A entries. This follows
// PHASE1-BRIEF's "Prep reader" section, without requiring fights or UI imports.
// Output sorts by Stamped, then exact Name; ties retain night insertion order,
// ASCII key order, then body index. Missing/invalid input yields no fake report.
func Parse(data []byte, firstFightStartS, lastFightEndS int64) ([]model.PrepReport, Report) {
	summary := Report{Skipped: map[string]int{}}
	if firstFightStartS < 0 || lastFightEndS < firstFightStartS || lastFightEndS > limits.MaxStamp {
		summary.Warnings = []string{"invalid fight bounds"}
		return nil, summary
	}
	root, err := parseLiteral(data)
	if err != nil {
		summary.Warnings = []string{err.Error()}
		return nil, summary
	}
	bodies, why := scanStore(root)
	if why != "" {
		summary.Warnings = []string{why}
		return nil, summary
	}
	var out []model.PrepReport
	for _, b := range bodies {
		r, why := decodeBody(b.body)
		if why == "" && nameKey(r.Name) != b.key {
			why = "body identity"
		}
		if why != "" {
			summary.Skipped[why]++
			continue
		}
		r.Stamped = b.receipts[0]
		eligible := false
		for _, at := range b.receipts {
			if at < r.Stamped {
				r.Stamped = at
			}
			if at >= firstFightStartS-limits.LookbackSeconds && at <= lastFightEndS {
				eligible = true
			}
		}
		if !eligible {
			summary.Skipped["outside night"]++
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Stamped != out[j].Stamped {
			return out[i].Stamped < out[j].Stamped
		}
		return out[i].Name < out[j].Name
	})
	return out, summary
}
