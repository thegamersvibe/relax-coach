package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"io"
	"strings"
	"testing"
)

func TestPowerSamplesBelongToInfoUnit(t *testing.T) {
	es := boss(7, 1000)
	power := modeltest.At(1004).Src(alia).Dst(bram).Power(0, 800, 0).Build()
	power.InfoUnit = bram.ID
	missing := modeltest.At(1005).Src(bram).Dst(alia).Build()
	missing.InfoUnit = alia.ID
	missing.HP = model.Some(50)
	es = append(es[:4], append([]model.Event{power, missing}, es[4:]...)...)
	d := only(t, build(t, es, params())).Details[0]
	bs, as := d.Timelines[bram.ID].Samples, d.Timelines[alia.ID].Samples
	if len(bs) != 1 || len(as) != 1 {
		t.Fatalf("power-only sample missing: bram=%d alia=%d", len(bs), len(as))
	}
	if bs[0].Power != model.Some(0) || bs[0].PowerMax != model.Some(800) || bs[0].PowerType != model.Some(0) || bs[0].HP.Present {
		t.Fatal("zero mana conflated with absent")
	}
	if as[0].Power.Present || as[0].PowerMax.Present || as[0].PowerType.Present || as[0].HP != model.Some(50) {
		t.Fatal("power borrowed from another unit")
	}
}

// No power sample without a power bar (PowerMax 0, a creature's 0 of 0) or from a health-cost cast (type -2,
// whose current/max repeat health). Health and position on the same line are still sampled.
func TestNoPowerSampleWithoutABarOrFromHealthCost(t *testing.T) {
	es := boss(7, 1000)
	nobar := modeltest.At(1004).Src(alia).Dst(bram).Power(0, 0, 1).Build()
	nobar.InfoUnit, nobar.HP = bram.ID, model.Some(70)
	health := modeltest.At(1005).Src(alia).Dst(bram).Power(3848, 4014, -2).Build()
	health.InfoUnit = alia.ID
	energy := modeltest.At(1006).Src(alia).Dst(bram).Power(62, 115, 3).Build()
	energy.InfoUnit = alia.ID
	es = append(es[:4], append([]model.Event{nobar, health, energy}, es[4:]...)...)
	d := only(t, build(t, es, params())).Details[0]
	bs, as := d.Timelines[bram.ID].Samples, d.Timelines[alia.ID].Samples
	if len(bs) != 1 || bs[0].HP != model.Some(70) || bs[0].Power.Present || bs[0].PowerMax.Present || bs[0].PowerType.Present {
		t.Fatalf("no bar: health kept, no power: %+v", bs)
	}
	if len(as) != 1 || as[0].Power != model.Some(62) || as[0].PowerMax != model.Some(115) || as[0].PowerType != model.Some(3) {
		t.Fatalf("health cost gives no sample, energy does: %+v", as)
	}
}

func TestModelFightRangeAndSpecs(t *testing.T) {
	es := boss(7, 1000)
	for i := range es {
		es[i].Offset = int64(100 + i*20)
		es[i].Len = 20
	}
	tail := modeltest.At(es[4].TimeMs+35).Dst(giant).Died(0).Pos(200, 17).Build()
	es = append(es, tail)
	p := params()
	p.Profiles = []SpecProfile{{ID: "INVENTED.first", CastSignatures: []SpellSignature{{ID: 55, Name: "Invented signature"}}}}
	p.InferSpecs = func([]model.Event, []SpecProfile) map[string]string {
		return map[string]string{alia.ID: "INVENTED.first"}
	}
	r := only(t, build(t, es, p))
	f := r.Fights[0]
	if f.Offset != 100 || f.Len != 117 || f.Specs[alia.ID] != "INVENTED.first" {
		t.Fatal("derived fields missing from shared model")
	}
	p.DetailsFor = func(model.Fight) bool { return false }
	indexed := only(t, build(t, es, p)).Fights[0]
	if indexed.Offset != f.Offset || indexed.Len != f.Len {
		t.Fatal("index lost range")
	}
	if len(only(t, build(t, es, params())).Fights[0].Specs) != 0 {
		t.Fatal("unknown spec invented")
	}
}
func TestModelRunsCarrySourceAndHeader(t *testing.T) {
	raw := "invented raw file including ignored lines"
	decoder := func(r io.Reader, emit func(model.Event)) error {
		if _, err := io.Copy(io.Discard, r); err != nil {
			return err
		}
		for _, instance := range []int64{42, 43} {
			emit(model.Event{Kind: model.Header, Raw: []string{"22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "invented-build"}})
			es := boss(instance, instance*100000)
			es[0].Enc.InstanceID = model.Some(instance)
			for _, e := range es {
				emit(e)
			}
		}
		return nil
	}
	result, err := Read(strings.NewReader(raw), decoder, params())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Runs) != 2 {
		t.Fatal("runs")
	}
	d, ok := result.Runs[0].DungeonRun()
	if !ok {
		t.Fatal("kind")
	}
	n, ok := result.Runs[1].RaidNight()
	if !ok {
		t.Fatal("kind")
	}
	for _, src := range []model.Source{d.Source, n.Source} {
		if src.Bytes != int64(len(raw)) || src.SHA256 != result.Source.SHA256 {
			t.Fatal("raw source not carried to model run")
		}
	}
	if d.Build != "invented-build" || n.Build != d.Build || d.LogVersion != 22 || n.LogVersion != 22 || !d.Advanced || !n.Advanced {
		t.Fatal("header not carried to model run")
	}
}
