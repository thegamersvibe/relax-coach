package score_test

import (
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

// Actual fights/offsets, synthetic receipts at the UTC end and one hour later.
// No real preparation store accompanied the private logs. Emit counts only.
func realClockCounts(t *testing.T, f model.Fight, c cfg.Config) (selected, after int) {
	t.Helper()
	if len(f.Events) == 0 {
		return
	}
	if f.Events[0].OffsetH != 3 {
		t.Fatal("private fight offset differs from expected suffix")
	}
	end := (f.EndMs - int64(f.Events[0].OffsetH)*int64(time.Hour/time.Millisecond)) / int64(time.Second/time.Millisecond)
	var reports []model.PrepReport
	for _, p := range f.Players {
		early, late := stock(p), stock(p)
		early.Stamped, late.Stamped = end, end+int64(time.Hour/time.Second)
		reports = append(reports, early, late)
	}
	for _, n := range score.Score([]model.Fight{f}, nil, reports, c) {
		for _, fs := range n.Fights {
			for _, p := range fs.Parts {
				if p.Name != "preparation" {
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
	return
}
