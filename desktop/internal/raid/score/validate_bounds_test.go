package score_test

import (
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestMeasuredPartBoundsAndEvidence(t *testing.T) {
	valid := model.Part{Name: "mechanics", Tenths: model.Some(700), Coverage: 70,
		Evidence: model.Evidence{Text: "Measured.", Counts: map[string]int64{}}}
	for _, tc := range []struct {
		name   string
		change func(*model.Part)
		bad    bool
	}{
		{"zero score", func(p *model.Part) { p.Tenths = model.Some(0) }, false},
		{"max score", func(p *model.Part) { p.Tenths = model.Some(1000) }, false},
		{"negative score", func(p *model.Part) { p.Tenths = model.Some(-1) }, true},
		{"above score", func(p *model.Part) { p.Tenths = model.Some(1001) }, true},
		{"min coverage", func(p *model.Part) { p.Coverage = 1 }, false},
		{"max coverage", func(p *model.Part) { p.Coverage = 100 }, false},
		{"zero coverage", func(p *model.Part) { p.Coverage = 0 }, true},
		{"above coverage", func(p *model.Part) { p.Coverage = 101 }, true},
		{"no evidence", func(p *model.Part) { p.Evidence = model.Evidence{} }, true},
		{"blank text", func(p *model.Part) { p.Evidence.Text = " \t" }, true},
		{"absent counts", func(p *model.Part) { p.Evidence.Counts = nil }, true},
		{"max text", func(p *model.Part) { p.Evidence.Text = strings.Repeat("é", 200) }, false},
		{"long text", func(p *model.Part) { p.Evidence.Text = strings.Repeat("é", 201) }, true},
		{"max times", func(p *model.Part) { p.Evidence.AtMs = make([]int64, 20) }, false},
		{"long times", func(p *model.Part) { p.Evidence.AtMs = make([]int64, 21) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := valid
			tc.change(&p)
			if err := score.ValidatePart(p); (err != nil) != tc.bad {
				t.Fatalf("error=%v want bad=%v", err, tc.bad)
			}
		})
	}
}
