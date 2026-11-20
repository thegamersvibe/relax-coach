package death_test

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func TestHealthSampleDoesNotAddOtherTargetsToRecap(t *testing.T) {
	f := quickDeath()
	e := hit(9800, victim, boss, 91, 999)
	e.InfoUnit = victim.ID
	e.HPMax = model.Some(1000)
	e.HP = model.Some(300)
	f.Events = append(f.Events, e)
	d := forVictim(t, f, config())
	if !reflect.DeepEqual(d.Evidence, []int64{9900}) {
		t.Fatalf("other target entered recap: %v", d.Evidence)
	}
}
func TestEnvironmentalKillingHitCannotChargeEarlierMechanic(t *testing.T) {
	f := slowDeath(6000)
	f.Events[1].Amount = model.Some(500)
	f.Events[3].Src = model.Actor{}
	f.Events[3].SpellID = 0
	f.Events[3].SpellName = "FALLING" // logfmt environmental type, not merely an absent actor
	c := config()
	mechanic(&c, 81)
	wantDeath(t, forVictim(t, f, c), model.CauseUnavoidable, "high", model.OwnerNobody, "")
}
func TestAggroEvidenceIncludesEveryBossUnit(t *testing.T) {
	f := meleeDeath()
	f.Events[0].TimeMs = 7000
	f.Events[1].TimeMs = 7100
	second := hostile("Synthetic second boss", 72)
	f.BossUnits = append(f.BossUnits, second.ID)
	f.Events = append(f.Events, swing(9800, second, victim, 1))
	d := forVictim(t, f, config())
	wantDeath(t, d, model.CauseUnresolved, "low", model.OwnerUndecided, "")
	if !reflect.DeepEqual(d.Evidence, []int64{9000, 9800, 9900}) {
		t.Fatalf("boss evidence=%v", d.Evidence)
	}
}
func TestHealerResurrectedMidSpanStillBelongsToLivingTeam(t *testing.T) {
	f := slowDeath(6000)
	f.Events = append(f.Events, died(5000, healer2), model.Event{TimeMs: 8000, Kind: model.Resurrect, Dst: healer2})
	wantDeath(t, forVictim(t, f, config()), model.CauseUnhealed, "high", model.OwnerHealers, "", healer.Name, healer2.Name)
}
func TestSameTimestampFullSampleUsesEventOrder(t *testing.T) {
	inlineSample := slowDeath(6000)
	inlineSample.Events[1].InfoUnit = victim.ID
	inlineSample.Events[1].HP = model.Some(900)
	inlineSample.Events[1].HPMax = model.Some(1000)
	wantDeath(t, forVictim(t, inlineSample, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f := slowDeath(6000)
	f.Events[0].TimeMs = 6000
	wantDeath(t, forVictim(t, f, config()), model.CauseUnhealed, "high", model.OwnerHealers, "", healer.Name, healer2.Name)
	f.Events[0], f.Events[1] = f.Events[1], f.Events[0]
	wantDeath(t, forVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestNoDamageDoesNotInventSpan(t *testing.T) {
	f := fight(sample(0, 0, 1000), sample(8000, 0, 1000), modeltest.At(100).Src(victim).Dst(victim).Miss("ABSORB").Build(), died(10000, victim))
	wantDeath(t, forVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
