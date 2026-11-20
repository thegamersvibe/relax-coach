package fights

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

// Derived from the second opinion's START(1000), hit(5000), END(2000),
// hit(2100), tail=50 repro. The END line, not the in-fight high-water,
// owns the boss end and the inclusive tail boundary.
func TestBossEndOwnsTailAfterClockSpike(t *testing.T) {
	start, end := modeltest.Encounter(model.Encounter{ID: 77, InstanceID: model.Some(42), FightMs: model.Some(100), Success: model.Some(1)}, 1000, 2000)
	p := params()
	p.TailMs, p.PullGapMs = 50, 1000
	es := []model.Event{start, hit(5000), end, hit(2050), hit(2051), hit(2100)}
	r := only(t, build(t, es, p))
	if len(r.Fights) != 2 {
		t.Fatalf("fights=%d: clock spike swallowed post-END trash", len(r.Fights))
	}
	f := r.Fights[0]
	if f.EndMs != end.TimeMs || f.DurationMs != model.Some(100) || f.Partial || !reflect.DeepEqual(f.Events, es[:4]) {
		t.Fatal("boss end, server duration, raw clock spike or inclusive tail changed")
	}
	if r.Fights[1].Boss || r.Fights[1].StartMs != 2051 || !reflect.DeepEqual(r.Fights[1].Events, es[4:]) {
		t.Fatal("trash beyond END tail not kept in a separate pull")
	}
}

func TestBossTailUsesRawEndWhenStartIsLater(t *testing.T) {
	start, end := modeltest.Encounter(model.Encounter{ID: 77, InstanceID: model.Some(42), FightMs: model.Some(100)}, 5000, 2000)
	p := params()
	p.TailMs = 50
	r := only(t, build(t, []model.Event{start, end, hit(2051)}, p))
	if len(r.Fights) != 2 || r.Fights[0].EndMs != start.TimeMs || r.Fights[0].DurationMs != model.Some(100) {
		t.Fatal("display clamp extended raw END tail or lost nonnegative bounds")
	}
}

func TestDocumentedKnownBehaviourStrayEndSplitsActiveBoss(t *testing.T) {
	start, end := modeltest.Encounter(model.Encounter{ID: 77, InstanceID: model.Some(42), Success: model.Some(1)}, 1000, 1400)
	_, stray := modeltest.Encounter(model.Encounter{ID: 88, Success: model.Some(1)}, 1000, 1200)
	p := params()
	p.TailMs = 1000
	es := []model.Event{start, hit(1100), stray, hit(1300), end}
	r := only(t, build(t, es, p))
	// Known miss, intentionally not repaired in round 2: the stray END cuts
	// the active encounter, takes the following hit, and leaves an orphan END.
	if len(r.Fights) != 3 {
		t.Fatalf("documented fragment count=%d, want 3", len(r.Fights))
	}
	for i, want := range []struct {
		id     int64
		events []model.Event
	}{{77, es[:2]}, {88, es[2:4]}, {77, es[4:]}} {
		f := r.Fights[i]
		if !f.Partial || f.EncounterID != want.id || !reflect.DeepEqual(f.Events, want.events) {
			t.Fatalf("documented stray-END fragment %d changed", i)
		}
	}
	if r.Fights[0].Success.Present || r.NightID != digest("") {
		t.Fatal("documented miss unexpectedly became a complete boss")
	}
}
