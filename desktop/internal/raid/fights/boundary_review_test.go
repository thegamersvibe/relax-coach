package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"testing"
)

func TestBackwardTimeBounds(t *testing.T) {
	for _, es := range [][]model.Event{
		{hit(4000000), hit(400000), {Kind: model.Header, TimeMs: 400001}},
		{hit(4000000), hit(400000)},
		{boss(7, 4000000)[0], boss(7, 1000)[4]},
	} {
		r := only(t, build(t, es, params()))
		if r.EndMs < r.StartMs {
			t.Fatal("run end before start")
		}
		for _, f := range r.Fights {
			if f.EndMs < f.StartMs {
				t.Fatal("fight end before start")
			}
		}
	}
}
func TestRunPartialMeansMissingStartOnly(t *testing.T) {
	for _, header := range []bool{false, true} {
		es := append(boss(7, 1000), model.Event{TimeMs: 100000})
		if header {
			es = append(es, model.Event{Kind: model.Header, TimeMs: 100001})
		}
		r := only(t, build(t, es, params()))
		if r.Partial || r.Fights[0].Partial {
			t.Fatal("quiet EOF/header made complete run partial")
		}
	}
	es := boss(7, 1000)[:4]
	r := only(t, build(t, es, params()))
	if r.Partial || !r.Fights[0].Partial {
		t.Fatal("cut at EOF must affect fight only")
	}
	es = append(boss(7, 1000), hit(100000))
	r = only(t, build(t, es, params()))
	if r.Partial || !r.Fights[1].Partial {
		t.Fatal("cut trash must affect fight only")
	}
	r = only(t, build(t, boss(7, 1000)[4:], params()))
	if !r.Partial {
		t.Fatal("orphan first end must mark missing start")
	}
}
