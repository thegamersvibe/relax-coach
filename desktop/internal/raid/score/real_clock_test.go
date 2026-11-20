package score_test

import (
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

type realPrepCoverage struct {
	rows, measured, prepCoverage, fightCoverage int
}

// Actual fights/offsets; synthetic receipts at the UTC end and one hour later.
// Replay no reports, future-only, and mixed receipts. No real prep store exists.
func realClockCounts(t *testing.T, f model.Fight, c cfg.Config) (selected, after int, coverage [3]realPrepCoverage) {
	t.Helper()
	if len(f.Events) == 0 {
		return
	}
	if f.Events[0].OffsetH != 3 {
		t.Fatal("private fight offset differs from expected suffix")
	}
	end := (f.EndMs - int64(f.Events[0].OffsetH)*int64(time.Hour/time.Millisecond)) / int64(time.Second/time.Millisecond)
	var reports, future []model.PrepReport
	for _, p := range f.Players {
		early, late := stock(p), stock(p)
		early.Stamped, late.Stamped = end, end+int64(time.Hour/time.Second)
		reports = append(reports, early, late)
		future = append(future, late)
	}
	for mode, rs := range [][]model.PrepReport{nil, future, reports} {
		for _, n := range score.Score([]model.Fight{f}, nil, rs, c) {
			for _, fs := range n.Fights {
				for _, p := range fs.Parts {
					if p.Name != "preparation" {
						continue
					}
					coverage[mode].rows++
					coverage[mode].prepCoverage += p.Coverage
					coverage[mode].fightCoverage += fs.Coverage
					if p.Tenths.Present {
						coverage[mode].measured++
					}
					if mode != 2 {
						continue
					}
					stamp, ok := p.Evidence.Counts["prep_stamped_s"]
					if !ok {
						continue
					}
					selected++
					if stamp > end {
						after++
					}
				}
			}
		}
	}
	return
}
