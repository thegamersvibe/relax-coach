package death_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Reviewer round 4: a guard failure must never let a later rule charge.
func TestRollbackNeverPromotesLaterRule(t *testing.T) {
	t.Run("prewindow_wipe_to_victim", func(t *testing.T) {
		c := config()
		c.Scoring.Death.WipeAllTanksDead = true
		mechanic(&c, 81)
		f := fight(died(8000, tank), model.Event{TimeMs: 8001}, hit(19999, boss, victim, 81, 1), died(20000, victim))
		wantDeath(t, forVictim(t, f, c), model.CauseAfterWipe, "medium", model.OwnerNobody, "")
		f.Events[1].TimeMs = 7999
		wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	})
	t.Run("postdeath_raidwide_to_healers", func(t *testing.T) {
		c := config()
		f := fight(sample(10000, 1000, 1000), hit(14000, boss, victim, 81, 1), sample(16000, 500, 1000), hit(19999, boss, victim, 81, 1), died(20000, victim), hit(20001, boss, tank, 81, 1), hit(20002, boss, other, 81, 1))
		wantDeath(t, forVictim(t, f, c), model.CauseUnavoidable, "medium", model.OwnerNobody, "")
		f.Events[6].TimeMs = 20000
		wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	})
	t.Run("postdeath_step_and_equal_control", func(t *testing.T) {
		c := config()
		base := fight(sample(10000, 1000, 1000), hit(14000, boss, victim, 81, 1), sample(16000, 500, 1000), hit(19999, boss, victim, 81, 1), hit(19999, boss, tank, 81, 1), hit(19999, boss, other, 81, 1), died(20000, victim))
		f := base
		f.Events = append(append([]model.Event(nil), base.Events...), hit(20002, boss, tank, 81, 1), hit(20001, boss, other, 81, 1))
		wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
		f.Events[len(f.Events)-1].TimeMs = 20002
		if d := forVictim(t, f, c); d.Cause != model.CauseUnavoidable || d.OwnerKind != model.OwnerNobody {
			t.Fatalf("20002/20002 control: %+v", d)
		}
	})
}

// shapeFight builds a clean fight whose victim death exercises one cause shape.
func shapeFight(r *rand.Rand, shape int) (model.Fight, cfg.Config, int64) {
	c := config()
	var es []model.Event
	at := int64(1000)
	players := []model.Actor{tank, tankB, healer, healer2, other}
	for i := 0; i < 6+r.Intn(12); i++ {
		at += 1 + int64(r.Intn(900))
		p := players[r.Intn(len(players))]
		if r.Intn(2) == 0 {
			es = append(es, hit(at, p, boss, 91, 1))
		} else {
			es = append(es, swing(at, boss, p, 1))
		}
	}
	switch shape {
	case 0: // wipe called, then an avoidable death
		c.Scoring.Death.WipeAllTanksDead = true
		mechanic(&c, 81)
		at += 10
		es = append(es, died(at, tank), died(at+1, tankB))
		at += 100 + int64(r.Intn(3000))
		es = append(es, hit(at, boss, victim, 81, 100))
	case 1: // avoidable
		mechanic(&c, 81)
		at += 10
		es = append(es, hit(at, boss, victim, 81, 100))
	case 2: // raid-wide unavoidable, with post-death same-spell hits
		at += 10
		es = append(es, hit(at, boss, victim, 81, 1), hit(at, boss, tank, 81, 1), hit(at, boss, other, 81, 1))
	case 3: // unhealed over time
		at += 10
		es = append(es, sample(at, 1000, 1000))
		at += 4000
		es = append(es, hit(at, boss, victim, 81, 1), sample(at+1, 500, 1000))
		at += 4000
		es = append(es, hit(at, boss, victim, 81, 1))
	}
	at++
	es = append(es, died(at, victim))
	if shape == 2 {
		es = append(es, hit(at+1, boss, healer, 81, 1), hit(at+2, boss, healer2, 81, 1))
	}
	f := fight(es...)
	f.EndMs = at + 1000
	return f, c, at
}

func TestClockPerturbationCauseShapes(t *testing.T) {
	r := rand.New(rand.NewSource(20261121))
	seen := map[string]int{}
	for k := 0; k < 8000; k++ {
		shape := k % 4
		f, c, at := shapeFight(r, shape)
		line := -1
		for j, e := range f.Events {
			if e.IsDeath() && e.Dst.ID == victim.ID && e.TimeMs == at {
				line = j
			}
		}
		victimDeath := func(f model.Fight) model.Death {
			ds, _, _ := review(t, f, 100000000, c)
			for _, d := range ds {
				if d.Player.ID == victim.ID && d.TimeMs == f.Events[line].TimeMs {
					return d
				}
			}
			t.Fatal("victim death missing")
			return model.Death{}
		}
		before := victimDeath(f)
		seen[fmt.Sprint(shape, ":", before.Cause)]++
		es := append([]model.Event(nil), f.Events...)
		i := 1 + r.Intn(len(es)-1)
		es[i].TimeMs = es[i-1].TimeMs - 1 - int64(r.Intn(5000))
		g := f
		g.Events = es
		after := victimDeath(g)
		// Any backward step: the death is declined, whatever it was before.
		if after.Cause != model.CauseUnresolved || after.Confidence != "low" || after.OwnerKind != model.OwnerUndecided || len(after.Owners) != 0 || after.SuggestedCause != "" {
			t.Fatalf("shape %d step at line %d: %s -> %s", shape, i, clockCharge(before), clockCharge(after))
		}
	}
	for _, want := range []string{"0:" + model.CauseAfterWipe, "1:" + model.CauseAvoidable, "2:" + model.CauseUnavoidable, "3:" + model.CauseUnhealed} {
		if seen[want] == 0 {
			t.Fatalf("generator never produced %s on a clean clock: %v", want, seen)
		}
	}
	t.Logf("8000 cause-shape perturbations, clean causes: %v", seen)
}
