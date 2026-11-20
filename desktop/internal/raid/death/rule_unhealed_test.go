package death_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func slowDeath(first int64) model.Fight {
	return fight(sample(5000, 900, 1000), hit(first, boss, victim, 81, 100), sample(8000, 300, 1000), hit(9999, boss, victim, 82, 100), died(10000, victim))
}
func TestUnhealedThresholds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		first int64
		cause string
	}{
		{"at", 6000, model.CauseUnhealed}, {"below", 6001, model.CauseUnresolved}, {"above", 5999, model.CauseUnhealed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := forVictim(t, slowDeath(tc.first), config())
			if tc.cause == model.CauseUnresolved {
				wantDeath(t, d, tc.cause, "low", model.OwnerUndecided, "")
			} else {
				wantDeath(t, d, tc.cause, "high", model.OwnerHealers, "", healer.Name, healer2.Name)
			}
		})
	}
}
func TestSparseSamplesAndWindow(t *testing.T) {
	wrongUnit := slowDeath(6000)
	wrongUnit.Events[0].InfoUnit = healer.ID
	wrongUnit.Events[0].Dst = victim
	wantDeath(t, forVictim(t, wrongUnit, config()), model.CauseUnresolved, "low", model.OwnerUndecided, model.CauseUnhealed)
	for _, tc := range []struct {
		name       string
		change     func(*model.Fight)
		suggestion string
	}{
		{"one", func(f *model.Fight) { f.Events[0].HPMax = model.Some(0) }, model.CauseUnhealed},
		{"wrong unit", func(f *model.Fight) { f.Events[0].InfoUnit = healer.ID }, model.CauseUnhealed},
		{"outside window", func(f *model.Fight) { f.Events[0].TimeMs = -1 }, model.CauseUnhealed},
		{"short span", func(f *model.Fight) { f.Events[0].HPMax = model.Some(0); f.Events[1].TimeMs = 6001 }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := slowDeath(6000)
			tc.change(&f)
			wantDeath(t, forVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, tc.suggestion)
		})
	}
	f := slowDeath(6000)
	f.Events[0] = sample(0, 899, 1000)
	wantDeath(t, forVictim(t, f, config()), model.CauseUnhealed, "high", model.OwnerHealers, "", healer.Name, healer2.Name)
	c := config()
	c.Scoring.Death.MinHealthSamples = 3
	wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, model.CauseUnhealed)
	c = config()
	c.Scoring.Death.WindowMs = 3999
	wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestFullHealthResetsSpan(t *testing.T) {
	for _, tc := range []struct {
		hp    int64
		cause string
	}{{899, model.CauseUnhealed}, {900, model.CauseUnresolved}, {901, model.CauseUnresolved}} {
		f := slowDeath(6000)
		f.Events[2] = sample(8000, tc.hp, 1000)
		d := forVictim(t, f, config())
		if d.Cause != tc.cause {
			t.Fatalf("hp=%d cause=%s", tc.hp, d.Cause)
		}
	}
	f := slowDeath(6000)
	f.Events[0] = sample(5000, 0, 1000) // no full sample: first damage in the window
	wantDeath(t, forVictim(t, f, config()), model.CauseUnhealed, "high", model.OwnerHealers, "", healer.Name, healer2.Name)
	f = slowDeath(6000)
	c := config()
	c.Scoring.Death.FullHealthFraction = .3
	wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestEffectiveHealingAndAbsorbs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		e      model.Event
		blocks bool
	}{
		{"overheal only", modeltest.At(7000).Src(healer).Dst(victim).Heal(100, 100).Build(), false},
		{"one point", modeltest.At(7000).Src(healer).Dst(victim).Heal(100, 99).Build(), true},
		{"negative overheal", modeltest.At(7000).Src(healer).Dst(victim).Heal(1, -1).Build(), true},
		{"other target", modeltest.At(7000).Src(healer).Dst(other).Heal(1, 0).Build(), false},
		{"before span", modeltest.At(5999).Src(healer).Dst(victim).Heal(1, 0).Build(), false},
		{"at span", modeltest.At(6000).Src(healer).Dst(victim).Heal(1, 0).Build(), true},
		{"at death", modeltest.At(10000).Src(healer).Dst(victim).Heal(1, 0).Build(), true},
		{"after death", modeltest.At(10001).Src(healer).Dst(victim).Heal(1, 0).Build(), false},
		{"absorb", modeltest.At(7000).Src(boss).Dst(victim).Absorb(healer, 91, 1).Build(), true},
		{"empty absorb", modeltest.At(7000).Src(boss).Dst(victim).Absorb(healer, 91, 0).Build(), false},
		{"self absorb miss", modeltest.At(7000).Src(victim).Dst(victim).Miss("ABSORB").Build(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := slowDeath(6000)
			// The inclusive timestamp control precedes this UNIT_DIED line.
			f.Events = append(f.Events[:len(f.Events)-1], tc.e, f.Events[len(f.Events)-1])
			d := forVictim(t, f, config())
			if tc.blocks {
				wantDeath(t, d, model.CauseUnresolved, "low", model.OwnerUndecided, "")
			} else {
				wantDeath(t, d, model.CauseUnhealed, "high", model.OwnerHealers, "", healer.Name, healer2.Name)
			}
		})
	}
	f := slowDeath(6000)
	f.Events[1].Absorbed = model.Some(1)
	wantDeath(t, forVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestHealerAliveWholeSpan(t *testing.T) {
	for _, at := range []int64{5999, 6000, 8000, 10000} {
		f := slowDeath(6000)
		delete(f.Roles, healer2.ID)
		// At-death transitions count only when logged before the victim's death.
		f.Events = append(f.Events[:len(f.Events)-1], died(at, healer), f.Events[len(f.Events)-1])
		wantDeath(t, forVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	}
	f := slowDeath(6000)
	delete(f.Roles, healer2.ID)
	f.Events = append(f.Events, died(5000, healer), model.Event{TimeMs: 6000, Kind: model.Resurrect, Dst: healer})
	wantDeath(t, forVictim(t, f, config()), model.CauseUnhealed, "high", model.OwnerHealers, "", healer.Name)
	f.Events[len(f.Events)-1].TimeMs = 6001
	wantDeath(t, forVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f = slowDeath(6000)
	f.Events = append(f.Events, died(8000, healer))
	wantDeath(t, forVictim(t, f, config()), model.CauseUnhealed, "high", model.OwnerHealers, "", healer2.Name)
	f = slowDeath(6000)
	delete(f.Roles, healer.ID)
	delete(f.Roles, healer2.ID)
	wantDeath(t, forVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestRecapLastTenAndNegativeOverkill(t *testing.T) {
	f := slowDeath(5001)
	f.Events = []model.Event{sample(0, 900, 1000), sample(1, 800, 1000)}
	for i := 0; i < 12; i++ {
		f.Events = append(f.Events, hit(1000+int64(i)*700, boss, victim, 81, 1))
	}
	f.Events = append(f.Events, died(10000, victim))
	d := forVictim(t, f, config())
	wantDeath(t, d, model.CauseUnhealed, "high", model.OwnerHealers, "", healer.Name, healer2.Name)
	if len(d.Evidence) != 10 || d.Evidence[0] != 2400 || d.Evidence[9] != 8700 {
		t.Fatalf("recap=%v", d.Evidence)
	}
}
