package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"testing"
)

func TestRolesRequireActivity(t *testing.T) {
	for _, inclusive := range []bool{false, true} {
		p := params()
		p.HealerStrictlyGreater = !inclusive
		es := boss(7, 1000)
		es[3].InfoUnit, es[3].HPMax = giant.ID, model.Some(900)
		es = append(es[:4], modeltest.At(1010).Src(giant).Dst(cato).Damage(10, -1).Landed().Build(), es[4])
		f := only(t, build(t, es, p)).Fights[0]
		if _, ok := f.Roles[bram.ID]; ok {
			t.Fatal("inactive combatant assigned a role")
		}
		if f.Roles[alia.ID] != "dps" {
			t.Fatal("actual damage is role evidence")
		}
	}
}
func TestTrashTankEvidence(t *testing.T) {
	p := params()
	es := []model.Event{{TimeMs: 0}, modeltest.At(20000).Src(giant).Dst(alia).Damage(10, -1).Landed().Build(), modeltest.At(20001).Src(giant).Dst(alia).Damage(10, -1).Landed().Build(), modeltest.At(20002).Src(giant).Dst(bram).Damage(10, -1).Landed().Build()}
	f := only(t, build(t, es, p)).Fights[0]
	if f.Boss || f.Roles[alia.ID] != "tank" || f.Roles[bram.ID] != "tank" {
		t.Fatal("trash landed swings did not assign tanks at top-ratio equality")
	}
	p.TankMinTopRatio = .51
	f = only(t, build(t, es, p)).Fights[0]
	if f.Roles[alia.ID] != "tank" || f.Roles[bram.ID] != "" {
		t.Fatal("trash top-ratio ignored or role invented")
	}
	p = params()
	p.TankMinBossSwingShare = .34
	if only(t, build(t, es, p)).Fights[0].Roles[bram.ID] != "" {
		t.Fatal("trash share ignored")
	}
}
func TestNoSwingsNoInventedRoles(t *testing.T) {
	es := []model.Event{{TimeMs: 0}, modeltest.At(20000).Src(giant).Dst(bram).Miss("DODGE").Build()}
	if len(only(t, build(t, es, params())).Fights[0].Roles) != 0 {
		t.Fatal("trash without output or landed swings assigned role")
	}
	es = boss(7, 1000)
	es[3] = modeltest.At(1003).Src(giant).Dst(alia).Damage(10, -1).Landed().Build()
	if len(only(t, build(t, es, params())).Fights[0].Roles) != 0 {
		t.Fatal("unknown boss unit invented a role")
	}
}
