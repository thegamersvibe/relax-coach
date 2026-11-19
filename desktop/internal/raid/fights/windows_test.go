package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"testing"
)

func hit(ms int64) model.Event { return modeltest.At(ms).Src(alia).Dst(giant).Damage(10, -1).Build() }
func TestTailAndPullBoundaries(t *testing.T) {
	es := boss(7, 1000)
	end := es[len(es)-1].TimeMs
	es = append(es, modeltest.At(end+35).Dst(giant).Died(0).Build(), hit(end+2000), hit(end+2500), hit(end+2501), hit(end+12501), hit(end+22502))
	r := only(t, build(t, es, params()))
	if len(r.Fights) != 3 {
		t.Fatalf("want boss + 2 pulls, got %d", len(r.Fights))
	}
	f := r.Fights[0]
	if f.EndMs != end || f.DurationMs.V != 50000 || len(f.Events) != 8 {
		t.Fatalf("tail not inclusive: end=%d events=%d", f.EndMs, len(f.Events))
	}
	if r.Fights[1].StartMs != end+2501 || len(r.Fights[1].Events) != 2 || r.Fights[2].StartMs != end+22502 {
		t.Fatal("pull gap is not based on combat, inclusive at the gap")
	}
	if r.Fights[1].ID != f.ID+":1" || r.Fights[2].ID != f.ID+":2" {
		t.Fatal("file-local pull ids")
	}
	p := params()
	p.TailMs = 0
	p.PullGapMs = 1
	if len(only(t, build(t, es, p)).Fights) == len(r.Fights) {
		t.Fatal("config ignored")
	}
}
func TestNextStartClosesPreviousBeforeTail(t *testing.T) {
	es := boss(7, 1000)
	next := boss(8, es[len(es)-1].TimeMs+1)
	es = append(es, next...)
	r := only(t, build(t, es, params()))
	if len(r.Fights) != 2 || len(r.Fights[0].Events) != 5 || r.Fights[1].EncounterID != 8 {
		t.Fatal("start swallowed by tail")
	}
	// Format reported, not seen: a second start without a preceding end.
	es = append(boss(7, 1000)[:4], boss(8, 2000)...)
	r = only(t, build(t, es, params()))
	if len(r.Fights) != 2 || !r.Fights[0].Partial || r.Fights[0].Success.Present || r.Fights[0].ID != "start:1" {
		t.Fatal("incomplete encounter lost")
	}
}
func TestRunEndsAtZoneOut(t *testing.T) {
	es := []model.Event{{Kind: model.Header, TimeMs: 0}, modeltest.At(1).Zone(42, "Invented Halls", 2).Build()}
	es = append(es, boss(7, 1000)...)
	out := es[len(es)-1].TimeMs + 100
	es = append(es, modeltest.At(out).Zone(0, "Outside", 0).Build(), hit(out+600000))
	r := only(t, build(t, es, params()))
	if r.EndMs != out {
		t.Fatalf("run ends %d, want zone-out %d", r.EndMs, out)
	}
	if len(r.Fights) != 1 || len(r.Fights[0].Events) != 5 || r.Kind != model.RunDungeon {
		t.Fatal("outside events admitted")
	}
}
func TestHeaderPartialAndUnknownRuns(t *testing.T) {
	es := append([]model.Event{hit(100)}, boss(7, 20000)[:4]...)
	es = append(es, model.Event{Kind: model.Header, TimeMs: 21000})
	es = append(es, boss(8, 22000)...)
	r := build(t, es, params())
	if len(r.Runs) != 2 || !r.Runs[0].Partial || len(r.Runs[0].Fights) != 2 || !r.Runs[0].Fights[0].Partial || !r.Runs[0].Fights[1].Partial {
		t.Fatalf("header/mid-pull loss: %+v", r)
	}
	unknown := boss(7, 1000)
	unknown[0].Enc.InstanceID = model.Some(99)
	u := only(t, build(t, unknown, params()))
	if u.Kind != model.RunUnknown || u.Listed {
		t.Fatal("unknown instance guessed")
	}
	es = append(boss(7, 1000)[:4], modeltest.At(4000).Zone(43, "Invented Raid", 2).Build())
	es = append(es, boss(8, 5000)...)
	if len(build(t, es, params()).Runs) != 2 {
		t.Fatal("instance transition lost")
	}
}
func TestThirteenBossesCrossLoggerIdentity(t *testing.T) {
	var es []model.Event
	for id := int64(1); id <= 13; id++ {
		es = append(es, boss(id, id*100000)...)
	}
	r := only(t, build(t, es, params()))
	if len(r.Fights) != 13 || r.Kind != model.RunDungeon {
		t.Fatal("dungeon pairs")
	}
	other := append([]model.Event{hit(0)}, es...)
	for i := range other {
		other[i].TimeMs += 2000
	}
	s := only(t, build(t, other, params()))
	if s.NightID != r.NightID {
		t.Fatal("trash/clock contaminated night")
	}
	fewer := only(t, build(t, es[5:], params()))
	if fewer.NightID == r.NightID || len(fewer.Fights) != 12 {
		t.Fatal("missing first boss not distinguished")
	}
	for i, f := range fewer.Fights {
		if f.ID != r.Fights[i+1].ID {
			t.Fatal("shared boss changed")
		}
	}
}
