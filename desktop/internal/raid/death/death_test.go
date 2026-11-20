package death_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

var (
	victim  = modeltest.Player("Alia-Stone-PvP-EU")
	tank    = modeltest.Player("Bram-Oak-Hill-PvP-EU")
	healer  = modeltest.Player("Cato-PvP-EU")
	healer2 = modeltest.Player("Dara-PvP-EU")
	other   = modeltest.Player("Eris-PvP-EU")
	boss    = hostile("Synthetic boss", 71)
)

func hostile(name string, id int64) model.Actor {
	a := modeltest.Creature(name, id)
	a.Flags = 0x40 // REACTION_HOSTILE
	return a
}
func ptr(v int64) *int64 { return &v }
func config() cfg.Config {
	return cfg.Config{Scoring: cfg.Scoring{Death: cfg.Death{
		FullHealthFraction: .9, MinHealthSamples: 2, RaidwideShare: .6, RaidwideWindowMs: 2000,
		WindowMs: 10000, PulledAggroMs: 10000, WipeDeadShare: 1, OneShotWindowMs: 1500, UnhealedMinMs: 4000,
		LostBossGapMs: ptr(4000), SummaryWindowMs: ptr(10000),
	}}}
}
func fight(events ...model.Event) model.Fight {
	f := modeltest.NewFight("synthetic", model.Encounter{ID: 71, Success: model.Some(1), FightMs: model.Some(60000)}, events...).
		Players(victim, tank, healer, healer2, other).BossUnits(boss).
		Role(victim, "dps").Role(tank, "tank").Role(healer, "healer").Role(healer2, "healer").Role(other, "dps").Build()
	f.StartMs, f.EndMs = 0, 20000
	return f
}
func hit(at int64, src, dst model.Actor, spell, amount int64) model.Event {
	return modeltest.At(at).Src(src).Dst(dst).Spell(spell, "Synthetic ability").Damage(amount, -1).Build()
}
func swing(at int64, src, dst model.Actor, amount int64) model.Event {
	return modeltest.At(at).Src(src).Dst(dst).Damage(amount, -1).Landed().Build()
}
func sample(at, hp, hpMax int64) model.Event {
	return model.Event{TimeMs: at, InfoUnit: victim.ID, HP: model.Some(hp), HPMax: model.Some(hpMax)}
}
func died(at int64, who model.Actor) model.Event { return modeltest.At(at).Dst(who).Died(0).Build() }
func forVictim(t *testing.T, f model.Fight, c cfg.Config) model.Death {
	t.Helper()
	ds, _ := death.Review(f, c)
	for _, d := range ds {
		if d.Player.ID == victim.ID {
			return d
		}
	}
	t.Fatal("victim death absent")
	return model.Death{}
}
func wantDeath(t *testing.T, got model.Death, cause, confidence, owner, suggestion string, owners ...string) {
	t.Helper()
	if got.Cause != cause || got.Confidence != confidence || got.OwnerKind != owner || got.SuggestedCause != suggestion {
		t.Fatalf("cause/confidence/owner/suggestion = %q/%q/%q/%q, want %q/%q/%q/%q", got.Cause, got.Confidence, got.OwnerKind, got.SuggestedCause, cause, confidence, owner, suggestion)
	}
	if len(got.Owners) != len(owners) {
		t.Fatalf("owners=%v, want %v", got.Owners, owners)
	}
	for i := range owners {
		if got.Owners[i] != owners[i] {
			t.Fatalf("owners=%v, want %v", got.Owners, owners)
		}
	}
}
func sorted(events ...model.Event) []model.Event {
	sort.SliceStable(events, func(i, j int) bool { return events[i].TimeMs < events[j].TimeMs })
	return events
}

func TestDeathsOnlyFromSelectedPlayerEvents(t *testing.T) {
	pet := modeltest.Pet("Synthetic pet", victim)
	events := []model.Event{died(9000, pet), died(10000, victim)}
	for i := 0; i < 15; i++ {
		events = append(events, modeltest.At(10001+int64(i)).Dst(victim).Died(1).Build())
	}
	events = append(events, model.Event{TimeMs: 11000, Kind: model.UnitDied, Dst: other}) // absent is not zero
	f := fight(events...)
	f.Owners = map[string]string{pet.ID: victim.ID}
	ds, wipes := death.Review(f, config())
	if len(ds) != 1 || len(wipes) != 0 {
		t.Fatalf("deaths=%d wipes=%d", len(ds), len(wipes))
	}
	wantDeath(t, ds[0], model.CauseUnresolved, "low", model.OwnerUndecided, "")
	if ds[0].Player != victim || ds[0].TimeMs != 10000 || ds[0].FightID != f.ID {
		t.Fatal("death identity lost")
	}
	f.Events = nil
	ds, wipes = death.Review(f, config())
	if len(ds) != 0 || len(wipes) != 0 {
		t.Fatal("index-only fight was judged")
	}
}

func TestReviewDeterministicAndDoesNotMutate(t *testing.T) {
	f := fight(died(12000, other), died(10000, victim))
	before := append([]model.Event(nil), f.Events...)
	a, aw := death.Review(f, config())
	b, bw := death.Review(f, config())
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(aw, bw) || !reflect.DeepEqual(before, f.Events) {
		t.Fatal("review mutated input or is not deterministic")
	}
	if len(a) != 2 || a[0].Player.ID != victim.ID {
		t.Fatal("deaths not in stable time order")
	}
}
