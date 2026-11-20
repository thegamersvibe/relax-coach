package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"math"
	"testing"
)

func TestRunKeepsFirstInferredInstance(t *testing.T) {
	es := boss(7, 1000)
	next := boss(8, 100000)
	next[0].Enc.InstanceID = model.Some(99)
	r := only(t, build(t, append(es, next...), params()))
	if r.InstanceID != 42 || r.Kind != model.RunDungeon {
		t.Fatal("later start replaced run's first inferred instance")
	}
}
func TestOwnerIncomingIsNotPlayerIncoming(t *testing.T) {
	pet := modeltest.Pet("Invented Ward", alia)
	es := boss(7, 1000)
	additions := []model.Event{modeltest.At(1010).Src(giant).Dst(pet).Damage(25, -1).Landed().Build(), modeltest.At(1020).Src(bram).Dst(pet).Heal(15, 0).Build()}
	es = append(es[:4], append(additions, es[4:]...)...)
	d := only(t, build(t, es, params())).Details[0]
	if d.Timelines[alia.ID].DamageTaken != 0 || d.Timelines[alia.ID].HealingReceived != 0 {
		t.Fatal("pet incoming charged to player")
	}
}
func TestRoleStrictnessAndMissingDuration(t *testing.T) {
	es := boss(7, 1000)
	es[4].Enc.FightMs = model.Opt{}
	es[3].InfoUnit, es[3].HPMax = giant.ID, model.Some(900)
	es = append(es[:4], modeltest.At(1009).Src(giant).Dst(bram).Damage(10, -1).Landed().Build(), modeltest.At(1010).Src(alia).Dst(bram).Heal(100, 0).Build(), es[4])
	p := params()
	r := only(t, build(t, es, p))
	if r.Fights[0].Roles[alia.ID] != "dps" || r.Details[0].LowConfidence || r.Fights[0].DurationMs.Present {
		t.Fatal("unknown duration or strict equality")
	}
	p.HealerStrictlyGreater = false
	if only(t, build(t, es, p)).Fights[0].Roles[alia.ID] != "healer" {
		t.Fatal("strictness ignored")
	}
}
func TestInvalidParamsRejected(t *testing.T) {
	for _, mutate := range []func(*Params){func(p *Params) { p.PullGapMs = -1 }, func(p *Params) { p.TailMs = -1 }, func(p *Params) { p.TankMinBossSwingShare = math.NaN() }, func(p *Params) { p.HealerMinHealingDamageRatio = -1 }, func(p *Params) { p.BossUnitSelection = "guessed" }, func(p *Params) { p.InstanceKind = nil }} {
		p := params()
		mutate(&p)
		if _, err := Build(nil, p); err == nil {
			t.Error("invalid config accepted")
		}
	}
}
func TestOrphanEndUnknownProgressAndEmptyStream(t *testing.T) {
	es := boss(7, 1000)
	r := only(t, build(t, es[4:], params()))
	if !r.Fights[0].Partial || r.Fights[0].ID != "start:1" || r.Details[0].RemainingHP.Present {
		t.Fatal("missing evidence invented")
	}
	if len(build(t, nil, params()).Runs) != 0 {
		t.Fatal("empty run invented")
	}
}
