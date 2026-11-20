package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"testing"
)

func TestFirstEncounterEndWins(t *testing.T) {
	es := boss(7, 1000)
	duplicate := es[4]
	copyEnc := *duplicate.Enc
	duplicate.Enc = &copyEnc
	duplicate.Enc.Success = model.Some(0)
	duplicate.Enc.FightMs = model.Some(99)
	duplicate.TimeMs++
	es = append(es, duplicate)
	duplicate.TimeMs += 100000
	es = append(es, duplicate)
	cut := only(t, build(t, es, params()))
	if cut.EndMs != es[4].TimeMs {
		t.Fatal("ignored duplicate END moved run end")
	}
	es = append(es, boss(7, 200000)...)
	res := build(t, es, params())
	r := only(t, res)
	if len(r.Fights) != 2 || r.Fights[0].Success != model.Some(1) || r.Fights[0].DurationMs != model.Some(50000) || len(r.Fights[0].Events) != 5 {
		t.Fatal("duplicate END changed first end or created fight")
	}
	if res.DuplicateEncounterEnds != 2 {
		t.Fatal("duplicate END counter missing or wrong")
	}
}
