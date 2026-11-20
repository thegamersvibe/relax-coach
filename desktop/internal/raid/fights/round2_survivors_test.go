package fights

import (
	"reflect"
	"slices"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func TestCasterOnlyActorObservation(t *testing.T) {
	caster := cato
	caster.Flags = 1
	es := boss(7, 1000)
	es = append(es[:4], modeltest.At(1010).Src(giant).Dst(alia).Absorb(caster, 55, 23).Build(), es[4])
	r := only(t, build(t, es, params()))
	for _, actors := range [][]model.Actor{r.Roster, r.Fights[0].Players} {
		i := slices.IndexFunc(actors, func(a model.Actor) bool { return a.ID == caster.ID })
		if i < 0 || !reflect.DeepEqual(actors[i], caster) {
			t.Fatal("caster-only actor lost name/flags in roster or fight")
		}
	}
	if r.Logger != caster || r.Details[0].Timelines[caster.ID].HealingDone != 23 {
		t.Fatal("caster-only logger or healing lost")
	}
	// A creature GUID carrying pet type bits only in Caster must be indexed too.
	ward := modeltest.Creature("Invented shield ward", 89)
	ward.Flags = 0x1000
	es[4].Caster = ward
	r = only(t, build(t, es, params()))
	if !slices.Contains(r.Fights[0].Unowned, ward.ID) {
		t.Fatal("caster-only pet flags were not observed")
	}
}

func TestLateBossSelectionClearsStaleRoles(t *testing.T) {
	ward := modeltest.Pet("Invented late attacker", cato)
	ward.OwnerID = ""
	larger := modeltest.Creature("Invented larger boss", 90)
	es := boss(7, 1000)
	es[3].InfoUnit, es[3].HPMax = giant.ID, model.Some(100)
	largerHit := modeltest.At(1011).Src(ward).Dst(larger).Damage(7, -1).Build()
	largerHit.InfoUnit, largerHit.HPMax = larger.ID, model.Some(900)
	es = append(es[:4], modeltest.At(1010).Src(giant).Dst(alia).Damage(10, -1).Landed().Build(), largerHit, es[4], model.Event{TimeMs: 60000})
	before := only(t, build(t, es, params()))
	if !reflect.DeepEqual(before.Fights[0].BossUnits, []string{giant.ID}) || before.Fights[0].Roles[alia.ID] != "tank" {
		t.Fatal("fixture did not first assign a tank from the smaller boss")
	}
	es = append(es, model.Event{TimeMs: 60001, InfoUnit: ward.ID, InfoOwnerID: cato.ID})
	after := only(t, build(t, es, params()))
	f, d := after.Fights[0], after.Details[0]
	if !reflect.DeepEqual(f.BossUnits, []string{larger.ID}) || f.Owners[ward.ID] != cato.ID || d.Timelines[cato.ID].DamageDone != 7 || !d.RolesComputed {
		t.Fatal("late ownership did not recompute selected detail")
	}
	if len(f.Roles) != 0 {
		t.Fatal("late boss reselection retained stale roles without new tank evidence")
	}
}

func TestIndexBossEvidenceKeepsSourceCreaturePairs(t *testing.T) {
	ward := modeltest.Pet("Invented pending attacker", alia)
	ward.OwnerID = ""
	intruder := modeltest.Creature("Invented hostile intruder", 90)
	es := boss(7, 1000)
	es[3].Src = ward
	es[3].InfoUnit, es[3].HPMax = giant.ID, model.Some(900)
	// Last damage to the same target is from an unowned hostile creature.
	// A destination-only key would overwrite the earlier owner-eligible source.
	es = append(es[:4], modeltest.At(1010).Src(intruder).Dst(giant).Damage(7, -1).Build(), es[4], model.Event{TimeMs: 60000})
	p := params()
	p.DetailsFor = nil
	before := only(t, build(t, es, p))
	if len(before.Fights[0].BossUnits) != 0 {
		t.Fatal("fixture already selected a boss without player-owned damage")
	}
	es = append(es, model.Event{TimeMs: 60001, InfoUnit: ward.ID, InfoOwnerID: alia.ID})
	r := only(t, build(t, es, p))
	f := r.Fights[0]
	if !reflect.DeepEqual(f.BossUnits, []string{giant.ID}) || f.Owners[ward.ID] != alia.ID {
		t.Fatal("compacted evidence lost an independently owned source/creature pair")
	}
	if len(f.Events) != 0 || len(r.Details[0].Timelines) != 0 || r.Details[0].RolesComputed {
		t.Fatal("late index repair computed detail")
	}
}

func TestHeaderResetsSeenEncounterEnds(t *testing.T) {
	es := boss(7, 1000)
	orphan := boss(7, 100000)[4]
	es = append(es, model.Event{Kind: model.Header, TimeMs: 60000}, orphan)
	r := build(t, es, params())
	if len(r.Runs) != 2 || len(r.Runs[1].Fights) != 1 {
		t.Fatal("header retained duplicate-END state and lost the new orphan")
	}
	f := r.Runs[1].Fights[0]
	if !f.Partial || !r.Runs[1].Partial || f.EncounterID != 7 || f.StartMs != orphan.TimeMs || !reflect.DeepEqual(f.Events, []model.Event{orphan}) {
		t.Fatal("orphan after header was not a fresh partial fight")
	}
	if r.DuplicateEncounterEnds != 0 || r.Runs[0].DuplicateEncounterEnds != 0 || r.Runs[1].DuplicateEncounterEnds != 0 {
		t.Fatal("cross-header END counted as duplicate")
	}
}

func TestNegativeAbsorbDoesNotSubtractHealing(t *testing.T) {
	es := boss(7, 1000)
	positive := modeltest.At(1010).Src(giant).Dst(alia).Absorb(cato, 55, 23).Build()
	negative := modeltest.At(1011).Src(giant).Dst(alia).Absorb(cato, 55, -99).Build()
	missing := modeltest.At(1012).Src(giant).Dst(alia).Absorb(cato, 55, 0).Build()
	missing.Amount = model.Opt{V: 50}
	es = append(es[:4], positive, negative, missing, es[4])
	d := only(t, build(t, es, params())).Details[0]
	if d.Timelines[cato.ID].HealingDone != 23 || len(d.Timelines[cato.ID].Events) != 3 {
		t.Fatal("negative/absent absorb changed healing or removed original events")
	}
}

func TestPetAbsorbCasterCreditsOwner(t *testing.T) {
	for _, late := range []bool{false, true} {
		ward := modeltest.Pet("Invented shield pet", cato)
		if late {
			ward.OwnerID = ""
		}
		es := boss(7, 1000)
		absorb := modeltest.At(1010).Src(giant).Dst(alia).Absorb(ward, 55, 23).Build()
		es = append(es[:4], absorb, es[4], model.Event{TimeMs: 60000})
		if late {
			es = append(es, model.Event{TimeMs: 60001, InfoUnit: ward.ID, InfoOwnerID: cato.ID})
		}
		r := only(t, build(t, es, params()))
		f, d := r.Fights[0], r.Details[0]
		tl := d.Timelines[cato.ID]
		if f.Owners[ward.ID] != cato.ID || tl == nil || tl.HealingDone != 23 || len(tl.Events) != 1 || !reflect.DeepEqual(tl.Events[0], absorb) {
			t.Fatalf("late=%v: pet caster healing or original event not assigned to owner", late)
		}
		var total int64
		for _, timeline := range d.Timelines {
			total += timeline.HealingDone
		}
		if total != 23 || d.Timelines[ward.ID] != nil {
			t.Fatal("pet caster created duplicate healing or a separate player timeline")
		}
	}
}

func TestFriendlyDamageRequiresNPCControlToExclude(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		src, dst := cato, bram
		src.Flags, dst.Flags = 0x100, 0x10
		want := int64(17)
		if controlled {
			src.Flags, want = 0x200, 0
		}
		es := boss(7, 1000)
		es = append(es[:4], modeltest.At(1010).Src(src).Dst(dst).Damage(17, -1).Build(), es[4])
		d := only(t, build(t, es, params())).Details[0]
		if d.Timelines[cato.ID].DamageDone != want || d.Timelines[bram.ID].DamageTaken != 17 {
			t.Fatal("friendly reaction alone excluded normal player damage or changed incoming")
		}
	}
}

func TestOwnedNPCFriendlyDamageIsNotCharmedPlayerDamage(t *testing.T) {
	ward := modeltest.Creature("Invented controlled ward", 89)
	ward.Flags, ward.OwnerID = 0x1200, cato.ID
	friendly := bram
	friendly.Flags = 0x10
	es := boss(7, 1000)
	es = append(es[:4], modeltest.At(1010).Src(ward).Dst(friendly).Damage(17, -1).Build(), es[4])
	r := only(t, build(t, es, params()))
	d := r.Details[0]
	if r.Fights[0].Owners[ward.ID] != cato.ID || d.Timelines[cato.ID].DamageDone != 17 || d.Timelines[bram.ID].DamageTaken != 17 {
		t.Fatal("NPC-controlled owned creature was mistaken for a charmed player")
	}
}

func TestEnemyPetSwingsDoNotSupplyTrashTankEvidence(t *testing.T) {
	for _, flags := range []uint32{0x1000, 0x2000} {
		enemyPet := modeltest.Creature("Invented enemy ward", 89)
		enemyPet.Flags = flags
		es := []model.Event{{TimeMs: 0}, modeltest.At(20000).Src(giant).Dst(alia).Damage(10, -1).Landed().Build()}
		for i := int64(1); i <= 3; i++ {
			es = append(es, modeltest.At(20000+i).Src(enemyPet).Dst(bram).Damage(10, -1).Landed().Build())
		}
		r := only(t, build(t, es, params()))
		f := r.Fights[0]
		if f.Boss || f.Roles[alia.ID] != "tank" || len(f.Roles) != 1 {
			t.Fatal("enemy pet/guardian swings contaminated trash tank share or top player")
		}
		if r.Details[0].Timelines[bram.ID].DamageTaken != 30 || !slices.Contains(f.Unowned, enemyPet.ID) {
			t.Fatal("ignored tank evidence must still retain incoming damage and unowned pet")
		}
	}
}
