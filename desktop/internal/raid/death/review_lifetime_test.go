package death_test

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

// Round-1 reviewer's same-ms resurrection/damage reproduction, including two
// deaths at the same timestamp: timestamps alone cannot identify the cutoff.
func TestZZSameTimestampPostDeath(t *testing.T) {
	for _, secondDeath := range []int64{10000, 10001} {
		f := quickDeath()
		f.Events[2].SpellID = 82
		f.Events = append(f.Events, model.Event{TimeMs: 10000, Kind: model.Resurrect, Dst: victim}, hit(10000, boss, victim, 81, 100), died(secondDeath, victim))
		c := config()
		mechanic(&c, 81)
		ds, _, _ := review(t, chronological(f), 100000, c)
		if len(ds) != 2 {
			t.Fatalf("deaths=%d", len(ds))
		}
		wantDeath(t, ds[0], death.Unresolved, "low", model.OwnerUndecided, "")
		wantDeath(t, ds[1], death.Avoidable, "high", model.OwnerPlayer, "", victim.Name)
		if !reflect.DeepEqual(ds[0].Evidence, []int64{9900}) || !reflect.DeepEqual(ds[1].Evidence, []int64{10000}) {
			t.Fatalf("evidence crossed a death line: %v / %v", ds[0].Evidence, ds[1].Evidence)
		}
	}
}

func TestZZLifetimeEvidence(t *testing.T) {
	f := fight(sample(1000, 300, 1000), hit(2000, boss, victim, 81, 600), died(2001, victim), model.Event{TimeMs: 6000, Kind: model.Resurrect, Dst: victim}, sample(6001, 200, 1000), hit(9999, boss, victim, 82, 20), died(10000, victim))
	c := config()
	mechanic(&c, 81)
	ds, _, _ := review(t, chronological(f), 100000, c)
	if len(ds) != 2 {
		t.Fatalf("deaths=%d", len(ds))
	}
	wantDeath(t, ds[0], death.Avoidable, "high", model.OwnerPlayer, "", victim.Name)
	wantDeath(t, ds[1], death.Unresolved, "low", model.OwnerUndecided, "")
	if !reflect.DeepEqual(ds[1].Evidence, []int64{9999}) {
		t.Fatalf("prior-life evidence: %v", ds[1].Evidence)
	}
}

func TestDeathWindowUsesPreviousDeathNotResurrection(t *testing.T) {
	for _, res := range []bool{false, true} {
		f := fight(died(2000, victim), sample(2000, 800, 1000), hit(2001, boss, victim, 81, 1), sample(8000, 500, 1000), hit(9999, boss, victim, 82, 1), died(10000, victim))
		// Evidence after the preceding death but before a possible res belongs
		// to this window; a missing res line must not erase that evidence.
		if res {
			f.Events = append(f.Events, model.Event{TimeMs: 7000, Kind: model.Resurrect, Dst: victim})
		}
		ds, _, _ := review(t, chronological(f), 100000, config())
		if len(ds) != 2 {
			t.Fatalf("deaths=%d", len(ds))
		}
		wantDeath(t, ds[1], death.Unhealed, "high", model.OwnerHealers, "", healer.Name, healer2.Name)
	}
	f := slowDeath(6000)
	f.Events = append(f.Events, modeltest.At(7000).Dst(victim).Died(1).Build())
	wantDeath(t, forChronologicalVictim(t, f, config()), death.Unhealed, "high", model.OwnerHealers, "", healer.Name, healer2.Name)
}

func TestDeathLineCutsHealthAndHealerState(t *testing.T) {
	for _, what := range []string{"heal", "health", "healer_death", "healer_resurrection"} {
		t.Run(what, func(t *testing.T) {
			f := slowDeath(6000)
			delete(f.Roles, healer2.ID)
			cause, confidence, owner, suggestion := death.Unhealed, "high", model.OwnerHealers, ""
			owners := []string{healer.Name}
			switch what {
			case "heal":
				f.Events = append(f.Events, modeltest.At(10000).Src(healer).Dst(victim).Heal(1, 0).Build())
			case "health":
				f.Events = append(f.Events, sample(10000, 1000, 1000))
			case "healer_death":
				f.Events = append(f.Events, died(10000, healer))
			case "healer_resurrection":
				f.Events = append(f.Events, died(5000, healer), model.Event{TimeMs: 10000, Kind: model.Resurrect, Dst: healer})
				cause, confidence, owner, suggestion, owners = death.Unresolved, "low", model.OwnerUndecided, "", nil
			}
			wantDeath(t, forChronologicalVictim(t, f, config()), cause, confidence, owner, suggestion, owners...)
		})
	}
}
