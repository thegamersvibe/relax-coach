package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"testing"
)

func TestFightInstanceComesFromItsStart(t *testing.T) {
	es := append([]model.Event{modeltest.At(1).Zone(43, "Invented Raid", 2).Build()}, boss(7, 1000)...)
	r := only(t, build(t, es, params()))
	if r.InstanceID != 43 || r.Fights[0].InstanceID != 42 {
		t.Fatal("run zone replaced server-sent fight instance")
	}
	es[1].Enc.InstanceID = model.Opt{}
	if only(t, build(t, es, params())).Fights[0].InstanceID != 0 {
		t.Fatal("missing start instance invented")
	}
}
func TestNoSignatureMeansUnknownSpec(t *testing.T) {
	p := params()
	p.Profiles = []SpecProfile{{ID: "INVENTED.first"}}
	p.InferSpecs = func([]model.Event, []SpecProfile) map[string]string {
		return map[string]string{alia.ID: "INVENTED.first"}
	}
	r := only(t, build(t, boss(7, 1000), p))
	if len(r.Fights[0].Specs) != 0 {
		t.Fatal("empty signatures guessed a spec")
	}
}
func TestAdvancedEvidenceWithoutHeaderFlag(t *testing.T) {
	es := boss(7, 1000)
	e := modeltest.At(1004).Src(alia).Power(0, 800, 0).Build()
	e.InfoUnit = alia.ID
	es = append(es[:4], append([]model.Event{e}, es[4:]...)...)
	if !only(t, build(t, es, params())).Advanced {
		t.Fatal("typed advanced block ignored")
	}
}
