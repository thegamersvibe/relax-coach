package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"testing"
)

func TestInitialPullGapEquality(t *testing.T) {
	es := []model.Event{{TimeMs: 0}, hit(10000), modeltest.At(10001).Zone(0, "Outside", 0).Build()}
	r := only(t, build(t, es, params()))
	if !r.Partial || !r.Fights[0].Partial {
		t.Fatal("initial gap equality must be partial")
	}
}
func TestOnlyFirstPullCanMissRunStart(t *testing.T) {
	p := params()
	p.TailMs = 0
	es := []model.Event{hit(0), boss(7, 1)[0], boss(7, 1)[4]}
	es[2].TimeMs = 2
	es = append(es, hit(3), modeltest.At(4).Zone(0, "Outside", 0).Build())
	r := only(t, build(t, es, p))
	if len(r.Fights) != 3 || r.Fights[2].Partial {
		t.Fatal("later trash cannot miss run start")
	}
}
func TestCutPullGapEquality(t *testing.T) {
	es := []model.Event{{TimeMs: 0}, hit(20000), {TimeMs: 30000}}
	if !only(t, build(t, es, params())).Fights[0].Partial {
		t.Fatal("EOF at exact combat gap must cut pull")
	}
}
func TestZoneClosureIsNotEOFCut(t *testing.T) {
	es := []model.Event{{TimeMs: 0}, hit(20000), modeltest.At(30000).Zone(43, "Invented Raid", 2).Build()}
	if only(t, build(t, es, params())).Fights[0].Partial {
		t.Fatal("observed zone boundary was treated as EOF cut")
	}
}
func TestMismatchedEndDoesNotCompleteBoss(t *testing.T) {
	es := append(boss(7, 1000)[:4], boss(8, 1000)[4])
	r := only(t, build(t, es, params()))
	if len(r.Fights) != 2 || !r.Fights[0].Partial || !r.Fights[1].Partial || r.Fights[0].Success.Present || r.Fights[1].EncounterID != 8 {
		t.Fatal("mismatched end applied to wrong fight")
	}
}
func TestRepeatedSameZoneDoesNotSplit(t *testing.T) {
	es := append([]model.Event{modeltest.At(0).Zone(42, "Invented Halls", 2).Build()}, boss(7, 1000)[:4]...)
	es = append(es, modeltest.At(2000).Zone(42, "Invented Halls", 2).Build(), boss(7, 1000)[4])
	r := only(t, build(t, es, params()))
	if len(r.Fights) != 1 || r.Fights[0].Partial || len(r.Fights[0].Events) != 5 {
		t.Fatal("same-zone marker split fight")
	}
}
func TestInterruptDispelStartCombat(t *testing.T) {
	for _, kind := range []model.Kind{model.Interrupt, model.Dispel} {
		es := []model.Event{{TimeMs: 0}, {TimeMs: 20000, Kind: kind, Src: alia, Dst: giant}}
		r := only(t, build(t, es, params()))
		if len(r.Fights) != 1 || len(r.Fights[0].Events) != 1 {
			t.Fatal("utility combat omitted")
		}
	}
}
func TestClassifierGetsMaximumGroup(t *testing.T) {
	p := params()
	var size int64
	p.InstanceKind = func(_ int64, n int64) (model.RunKind, bool) { size = n; return model.RunRaid, true }
	build(t, append(instanceBoss(7, 1000, 99, 40), instanceBoss(8, 100000, 99, 10)...), p)
	if size != 40 {
		t.Fatal("classifier did not get maximum group size")
	}
}
func TestPartialBossExcludedFromNight(t *testing.T) {
	r := only(t, build(t, boss(7, 1000)[:4], params()))
	if r.NightID != digest("") {
		t.Fatal("partial boss polluted night")
	}
}
func TestQuietOnlyRunNotEmitted(t *testing.T) {
	if len(build(t, []model.Event{{TimeMs: 1}, {TimeMs: 2, Kind: model.Header}}, params()).Runs) != 0 {
		t.Fatal("quiet-only run emitted")
	}
}
func TestEqualRankOwnerFirstWins(t *testing.T) {
	pet := modeltest.Pet("Invented ambiguous ward", alia)
	pet.OwnerID = ""
	es := boss(7, 1000)
	es[3].Src = pet
	es = append(es[:4], model.Event{TimeMs: 1010, InfoUnit: pet.ID, InfoOwnerID: bram.ID}, model.Event{TimeMs: 1011, InfoUnit: pet.ID, InfoOwnerID: alia.ID}, es[4])
	f := only(t, build(t, es, params())).Fights[0]
	if f.Owners[pet.ID] != bram.ID {
		t.Fatal("equal-rank later owner replaced first")
	}
}
func TestBossCandidateNeedsPlayerDamage(t *testing.T) {
	decoy := modeltest.Creature("Invented unengaged giant", 90)
	es := boss(7, 1000)
	es[3].InfoUnit = giant.ID
	es[3].HPMax = model.Some(100)
	e := modeltest.At(1010).Src(giant).Dst(decoy).Damage(1, -1).Build()
	e.InfoUnit = decoy.ID
	e.HPMax = model.Some(900)
	es = append(es[:4], e, es[4])
	f := only(t, build(t, es, params())).Fights[0]
	if len(f.BossUnits) != 1 || f.BossUnits[0] != giant.ID {
		t.Fatal("NPC-only fight selected boss")
	}
}
func TestSelectorRejectsTruncatedGUID(t *testing.T) {
	es := boss(7, 1000)
	es[3].Dst.ID = "Creature-0-0-0-0-81"
	p := params()
	p.BossUnits = func(int64) []CreatureSelector { return []CreatureSelector{{ID: 81}} }
	if len(only(t, build(t, es, p)).Fights[0].BossUnits) != 0 {
		t.Fatal("truncated creature id matched")
	}
}
func TestConfidenceThresholdEquality(t *testing.T) {
	es := boss(7, 1000)
	es[4].Enc.FightMs = model.Some(params().LowConfidenceFightMs)
	if only(t, build(t, es, params())).Details[0].LowConfidence {
		t.Fatal("equality is not low confidence")
	}
}
func TestSelfEventAppearsOnce(t *testing.T) {
	es := boss(7, 1000)
	e := modeltest.At(1010).Src(cato).Dst(cato).Heal(9, 0).Build()
	es = append(es[:4], e, es[4])
	if len(only(t, build(t, es, params())).Details[0].Timelines[cato.ID].Events) != 1 {
		t.Fatal("self event duplicated")
	}
}
func TestOverkillExcludedFromDamage(t *testing.T) {
	es := boss(7, 1000)
	es[3] = modeltest.At(1003).Src(cato).Dst(alia).Damage(30, 10).Build()
	d := only(t, build(t, es, params())).Details[0]
	if d.Timelines[cato.ID].DamageDone != 20 || d.Timelines[alia.ID].DamageTaken != 20 {
		t.Fatal("overkill included")
	}
}
func TestTankPrecedesHealerWhenRequired(t *testing.T) {
	es := boss(7, 1000)
	es[3].InfoUnit = giant.ID
	es[3].HPMax = model.Some(900)
	es = append(es[:4], modeltest.At(1010).Src(giant).Dst(alia).Damage(10, -1).Landed().Build(), modeltest.At(1011).Src(alia).Dst(bram).Heal(200, 0).Build(), es[4])
	p := params()
	if only(t, build(t, es, p)).Fights[0].Roles[alia.ID] != "tank" {
		t.Fatal("healing overrode required tank")
	}
	p.HealerRequiresNotTank = false
	if only(t, build(t, es, p)).Fights[0].Roles[alia.ID] != "healer" {
		t.Fatal("config ignored")
	}
}
func TestUnknownBossHealthStaysAbsent(t *testing.T) {
	for _, maximum := range []bool{false, true} {
		es := boss(7, 1000)
		e := model.Event{TimeMs: 1010, InfoUnit: giant.ID}
		if maximum {
			e.HPMax = model.Some(900)
		} else {
			e.HP = model.Some(40)
		}
		es = append(es[:4], e, es[4])
		p := params()
		p.BossUnits = func(int64) []CreatureSelector { return []CreatureSelector{{ID: 81}} }
		d := only(t, build(t, es, p)).Details[0]
		if d.RemainingHP.Present || d.MaximumHP.Present {
			t.Fatal("missing health invented")
		}
	}
}
func TestHeaderAdvancedZeroMeansOff(t *testing.T) {
	es := append([]model.Event{{Kind: model.Header, Raw: []string{"22", "ADVANCED_LOG_ENABLED", "0"}}}, boss(7, 1000)...)
	if only(t, build(t, es, params())).Advanced {
		t.Fatal("advanced zero treated as true")
	}
}
