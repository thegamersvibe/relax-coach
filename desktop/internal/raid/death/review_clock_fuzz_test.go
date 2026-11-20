package death_test

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func clockCharge(d model.Death) string {
	return fmt.Sprintf("%s|%s|%v|%s|%s", d.Cause, d.OwnerKind, d.Owners, d.Confidence, d.SuggestedCause)
}

// Adapted from the reviewer's deterministic 40000-fight timestamp experiment.
// Each perturbation creates a backward adjacent step, without moving lines or
// changing the final death. A new/different charge is never an allowed outcome.
func checkClockPerturbation(t *testing.T, r *rand.Rand, c cfg.Config) (string, string) {
	t.Helper()
	players := []model.Actor{tank, tankB, victim, other}
	var es []model.Event
	at := int64(1000)
	lines := 8 + r.Intn(22)
	for i := 0; i < lines; i++ {
		at += int64(r.Intn(1800))
		switch k := r.Intn(10); {
		case k < 4:
			es = append(es, swing(at, boss, players[r.Intn(4)], 1))
		case k < 9:
			es = append(es, hit(at, players[r.Intn(4)], boss, 91, 1))
		default:
			es = append(es, died(at, victim), res(at+1, victim))
			at++
		}
	}
	for i := 0; i < 1+r.Intn(3); i++ {
		at += int64(r.Intn(900))
		es = append(es, swing(at, boss, victim, 1))
	}
	at += int64(r.Intn(300))
	es = append(es, died(at, victim))
	f := reviewFight(es...)
	f.EndMs = at + 1000
	last := func(f model.Fight) model.Death {
		ds, _, _ := review(t, f, 10000000, c)
		for i := len(ds) - 1; i >= 0; i-- {
			if ds[i].Player.ID == victim.ID && ds[i].TimeMs == at {
				return ds[i]
			}
		}
		t.Fatal("final victim death missing")
		return model.Death{}
	}
	before := last(f)
	f.Events = append([]model.Event(nil), es...)
	i := 1 + r.Intn(len(es)-2)
	if r.Intn(3) == 0 {
		f.Events[i].TimeMs = f.Events[i+1].TimeMs + 1 + int64(r.Intn(6000))
	} else {
		f.Events[i].TimeMs = f.Events[i-1].TimeMs - 1 - int64(r.Intn(6000))
	}
	after := last(f)
	if after.Cause == model.CauseUnresolved {
		wantDeath(t, after, model.CauseUnresolved, "low", model.OwnerUndecided, "")
		return before.Cause, after.Cause
	}
	if clockCharge(before) != clockCharge(after) {
		t.Fatalf("one-line perturbation line=%d %d->%d changed charge: %s -> %s", i, es[i].TimeMs, f.Events[i].TimeMs, clockCharge(before), clockCharge(after))
	}
	return before.Cause, after.Cause
}

func TestClockPerturbation(t *testing.T) {
	loaded, err := cfg.Load(filepath.Join("..", "..", "..", "..", "rules", "raid"))
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewSource(20261120))
	c := loaded.Snapshot()
	before, after := map[string]int{}, map[string]int{}
	for i := 0; i < 40000; i++ {
		a, b := checkClockPerturbation(t, r, c)
		before[a]++
		after[b]++
	}
	t.Logf("40000 one-line perturbations: before=%v after=%v", before, after)
}

func FuzzClockPerturbation(f *testing.F) {
	for _, seed := range []int64{0, 1, 20261120, -1, 9999, 6000} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		for i := 0; i < 32; i++ {
			checkClockPerturbation(t, r, config())
		}
	})
}
