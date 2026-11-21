package main

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"math/big"
	"testing"
)

func TestRaidPageNightExactDisplay(t *testing.T) {
	c := raidPrepConfig(t)
	s := score.NightSummary{Role: "dps", Parts: map[string]*big.Rat{"output": big.NewRat(12449, 10000), "activity": big.NewRat(125, 100), "mechanics": big.NewRat(0, 1)}}
	rows, reason := raidNightParts(s, c)
	if reason != "" || len(rows) != 5 || rows[0].Score != "1.2" || rows[1].Score != "1.3" || rows[2].Score != "0.0" || rows[3].Score != "not measured" {
		t.Fatalf("not rounded once: %+v %s", rows, reason)
	}
	if s.Parts["output"].Cmp(big.NewRat(12449, 10000)) != 0 {
		t.Fatal("page changed exact input")
	}
	for _, split := range []bool{false, true} {
		s = score.NightSummary{Split: split, Parts: s.Parts}
		rows, reason = raidNightParts(s, c)
		want := "no scored fights, no part comparison"
		if split {
			want = "role split evenly, no part comparison"
		}
		if len(rows) != 0 || reason != want {
			t.Fatal("empty role rendered values or lost reason")
		}
	}
}
