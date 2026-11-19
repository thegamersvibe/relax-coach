package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"reflect"
	"testing"
)

func TestOwnersBufferAndHeaderReset(t *testing.T) {
	pet := modeltest.Pet("Invented Twin", alia)
	pet.OwnerID = ""
	twin := modeltest.Pet("Invented Twin", bram)
	twin.OwnerID = ""
	lost := modeltest.Pet("Invented Lost", cato)
	lost.OwnerID = ""
	guardian := modeltest.Creature("Invented Guardian", 82)
	es := boss(7, 1000)
	es = es[:len(es)-1]
	es = append(es, modeltest.At(1004).Src(pet).Dst(giant).Damage(10, -1).Build(), modeltest.At(1005).Src(twin).Dst(giant).Damage(20, -1).Build(), modeltest.At(1006).Src(lost).Dst(giant).Damage(30, -1).Build(), model.Event{TimeMs: 1007, Kind: model.Summon, Src: bram, Dst: pet}, model.Event{TimeMs: 1008, Kind: model.Summon, Src: bram, Dst: twin}, model.Event{TimeMs: 1009, Kind: model.Summon, Src: alia, Dst: guardian}, model.Event{TimeMs: 1010, Kind: model.Other, InfoUnit: pet.ID, InfoOwnerID: alia.ID}, boss(7, 1000)[4])
	r := only(t, build(t, es, params()))
	f := r.Fights[0]
	if f.Owners[pet.ID] != alia.ID || f.Owners[twin.ID] != bram.ID || f.Owners[guardian.ID] != alia.ID || !reflect.DeepEqual(f.Unowned, []string{lost.ID}) {
		t.Fatalf("owners=%v unowned=%v", f.Owners, f.Unowned)
	}
	if r.Details[0].Timelines[alia.ID].DamageDone != 110 || r.Details[0].Timelines[bram.ID].DamageDone != 20 {
		t.Fatal("early pet damage not reassigned")
	}
	es = append(es, model.Event{Kind: model.Header, TimeMs: 60000})
	next := boss(8, 61000)
	next = append(next[:4], append([]model.Event{modeltest.At(61004).Src(pet).Dst(giant).Damage(4, -1).Build()}, next[4:]...)...)
	es = append(es, next...)
	reset := build(t, es, params())
	if len(reset.Runs) != 2 || len(reset.Runs[1].Fights[0].Owners) != 0 || len(reset.Runs[1].Fights[0].Unowned) != 1 {
		t.Fatal("ownership leaked across header")
	}
	if es[4].Src.OwnerID != "" {
		t.Fatal("input mutated")
	}
}
func TestBossUnitsRolesAndEffectiveTotals(t *testing.T) {
	decoy := modeltest.Creature("Invented Target", 83)
	decoy.Flags = 0x10000
	es := boss(7, 1000)
	es = es[:4]
	es = append(es, modeltest.At(1004).Src(alia).Dst(decoy).Damage(9, -1).Build(), model.Event{TimeMs: 1005, InfoUnit: decoy.ID, HP: model.Some(10), HPMax: model.Some(200)}, model.Event{TimeMs: 1006, InfoUnit: giant.ID, HP: model.Some(800), HPMax: model.Some(900)})
	for i := 0; i < 10; i++ {
		who := bram
		if i < 3 {
			who = alia
		}
		e := modeltest.At(int64(1010+i)).Src(giant).Dst(who).Damage(10, -1).Landed().Build()
		es = append(es, e)
		e.Stream = model.SwingSource
		es = append(es, e)
	}
	es = append(es, modeltest.At(1030).Src(cato).Dst(alia).Heal(110, 10).Build(), modeltest.At(1031).Src(cato).Dst(giant).Damage(99, -1).Build(), boss(7, 1000)[4])
	r := only(t, build(t, es, params()))
	f := r.Fights[0]
	if !reflect.DeepEqual(f.BossUnits, []string{giant.ID}) || f.Roles[alia.ID] != "tank" || f.Roles[bram.ID] != "tank" || f.Roles[cato.ID] != "healer" {
		t.Fatalf("boss=%v roles=%v", f.BossUnits, f.Roles)
	}
	if r.Details[0].Timelines[cato.ID].HealingDone != 100 || r.Details[0].Timelines[cato.ID].OverhealDone != 10 {
		t.Fatal("effective healing/overheal not split")
	}
	if r.Details[0].Timelines[alia.ID].DamageTaken != 30 {
		t.Fatal("source swings counted")
	}
	p := params()
	p.TankMinBossSwingShare = .31
	p.HealerMinHealingDamageRatio = 2
	changed := only(t, build(t, es, p))
	if changed.Fights[0].Roles[alia.ID] != "dps" || changed.Fights[0].Roles[cato.ID] != "dps" {
		t.Fatal("role config ignored")
	}
	p = params()
	p.BossUnits = func(id int64) []CreatureSelector {
		return []CreatureSelector{{ID: 83, Name: decoy.Name, Source: "synthetic"}}
	}
	f = only(t, build(t, es, p)).Fights[0]
	if !reflect.DeepEqual(f.BossUnits, []string{decoy.ID}) || f.Roles[alia.ID] != "dps" {
		t.Fatal("override did not replace heuristic")
	}
	p.BossUnits = func(int64) []CreatureSelector {
		return []CreatureSelector{{ID: 83, Name: "No match", Source: "synthetic"}}
	}
	if len(only(t, build(t, es, p)).Fights[0].BossUnits) != 0 {
		t.Fatal("selector fields not conjunctive / fell back")
	}
}
func TestTimelinesSamplesSpecHookAndMetadata(t *testing.T) {
	mine := alia
	mine.Flags = 1
	es := []model.Event{{Kind: model.Header, Raw: []string{"22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "invented-build"}}}
	pull := boss(7, 1000)
	pull[0].OffsetH = -4
	es = append(es, pull[:4]...)
	kinds := []model.Kind{model.CastStart, model.CastSuccess, model.CastFailed, model.AuraApplied, model.AuraRemoved, model.Absorb, model.Interrupt, model.Dispel, model.Resurrect}
	for i, k := range kinds {
		es = append(es, model.Event{TimeMs: int64(1100 + i), Kind: k, Src: mine, Dst: bram, SpellID: 55, Amount: model.Some(2)})
	}
	es = append(es, model.Event{TimeMs: 1200, Kind: model.Other, Src: mine, Dst: bram, InfoUnit: bram.ID, HP: model.Some(0), HPMax: model.Some(900), X: model.Some(12), Y: model.Some(34)}, modeltest.At(1300).Dst(mine).Died(1).Build(), modeltest.At(1400).Dst(bram).Died(0).Build(), pull[4])
	for i := range es {
		es[i].Offset = int64(i * 10)
		es[i].Len = 10
	}
	p := params()
	p.LowConfidenceFightMs = 50001
	p.Profiles = []SpecProfile{{ID: "INVENTED.first"}}
	p.InferSpecs = func(casts []model.Event, profiles []SpecProfile) map[string]string {
		if len(casts) != 1 || casts[0].Kind != model.CastSuccess || len(profiles) != 1 {
			t.Fatal("bad spec hook input")
		}
		return map[string]string{alia.ID: profiles[0].ID}
	}
	r := only(t, build(t, es, p))
	d := r.Details[0]
	if r.Logger.Name != alia.Name || r.Build != "invented-build" || r.LogVersion != 22 || !r.Advanced || r.OffsetH != -4 {
		t.Fatal("run metadata missing")
	}
	if d.Offset != 10 || d.Len != int64((len(es)-1)*10) || !d.LowConfidence || d.Specs[alia.ID] != "INVENTED.first" {
		t.Fatalf("sidecar=%+v", d)
	}
	if len(d.Timelines[bram.ID].Samples) != 1 || d.Timelines[bram.ID].Samples[0].HP != model.Some(0) || len(d.Timelines[alia.ID].Samples) != 0 {
		t.Fatal("sample subject guessed from source")
	}
	if len(d.Timelines[alia.ID].Deaths) != 0 || len(d.Timelines[bram.ID].Deaths) != 1 {
		t.Fatal("feign death classified as death")
	}
	if len(d.Timelines[alia.ID].Events) < len(kinds) {
		t.Fatal("timeline lost event families")
	}
	plain := only(t, build(t, es, params()))
	if len(plain.Details[0].Specs) != 0 || plain.Details[0].LowConfidence {
		t.Fatal("invented spec or duration confidence")
	}
}
func TestWipeRemainingHealth(t *testing.T) {
	second := modeltest.Creature("Invented Second Keeper", 84)
	es := boss(7, 1000)
	es[4].Enc.Success = model.Some(0)
	es = append(es[:4], append([]model.Event{{TimeMs: 1004, InfoUnit: giant.ID, HP: model.Some(40), HPMax: model.Some(100)}, {TimeMs: 1005, Src: alia, Dst: second, Kind: model.Damage, Amount: model.Some(1), InfoUnit: second.ID, HP: model.Some(20), HPMax: model.Some(100)}, modeltest.At(1006).Dst(second).Died(0).Build()}, es[4:]...)...)
	p := params()
	p.BossUnits = func(int64) []CreatureSelector { return []CreatureSelector{{ID: 81}, {ID: 84}} }
	d := only(t, build(t, es, p)).Details[0]
	if d.RemainingHP != model.Some(40) || d.MaximumHP != model.Some(200) {
		t.Fatal("multi-boss progress")
	}
}
