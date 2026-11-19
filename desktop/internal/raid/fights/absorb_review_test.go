package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"testing"
)

func TestAbsorbCreditsCaster(t *testing.T) {
	es := boss(7, 1000)
	absorb := modeltest.At(1010).Src(alia).Dst(bram).Absorb(cato, 55, 23).Build()
	es = append(es[:4], append([]model.Event{absorb}, es[4:]...)...)
	r := only(t, build(t, es, params()))
	d := r.Details[0]
	if d.Timelines[cato.ID] == nil || d.Timelines[cato.ID].HealingDone != 23 {
		t.Fatal("absorb caster not credited")
	}
	if d.Timelines[alia.ID].HealingDone != 0 || d.Timelines[bram.ID].HealingDone != 0 {
		t.Fatal("attacker or shield recipient credited")
	}
	if len(d.Timelines[cato.ID].Events) != 1 || r.Fights[0].Roles[cato.ID] != "healer" {
		t.Fatal("caster missing from timeline or role")
	}
	// A caster also present as recipient gets one timeline event, not two.
	absorb.Dst = cato
	es = append(boss(7, 1000)[:4], absorb, boss(7, 1000)[4])
	if len(only(t, build(t, es, params())).Details[0].Timelines[cato.ID].Events) != 1 {
		t.Fatal("caster timeline duplicate")
	}
}
