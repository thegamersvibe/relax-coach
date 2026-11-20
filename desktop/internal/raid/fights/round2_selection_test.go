package fights

import (
	"io"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestRolesComputedDistinguishesIndexFromNoEvidence(t *testing.T) {
	for _, mode := range []string{"nil-index", "rejected-index", "selected-no-evidence", "selected-evidence"} {
		t.Run(mode, func(t *testing.T) {
			p := params()
			es := boss(7, 1000)
			selected := strings.HasPrefix(mode, "selected-")
			if mode == "nil-index" {
				p.DetailsFor = nil
			} else {
				p.DetailsFor = func(model.Fight) bool { return selected }
			}
			if mode == "selected-evidence" {
				es[3].InfoUnit, es[3].HPMax = giant.ID, model.Some(900)
				es = append(es[:4], model.Event{TimeMs: 1010, Kind: model.Damage, Stream: model.SwingLanded, Src: giant, Dst: bram, Amount: model.Some(10)}, es[4])
			}
			r := only(t, build(t, es, p))
			if r.Details[0].RolesComputed != selected {
				t.Fatalf("mode=%s: roles-computed must distinguish index from missing evidence", mode)
			}
			if (len(r.Fights[0].Roles) > 0) != (mode == "selected-evidence") {
				t.Fatal("computed status invented roles or suppressed known roles")
			}
		})
	}
}

func TestNilDetailsForIsIndexOnly(t *testing.T) {
	es := append(boss(7, 1000), boss(8, 100000)...)
	for i := range es {
		es[i].Offset, es[i].Len = int64(i*20), 20
	}
	for _, mode := range []string{"Build", "Read"} {
		t.Run(mode, func(t *testing.T) {
			call := func(p Params) Run {
				t.Helper()
				if mode == "Build" {
					return only(t, build(t, es, p))
				}
				r, err := Read(strings.NewReader("synthetic"), func(_ io.Reader, emit func(model.Event)) error {
					for _, e := range es {
						emit(e)
					}
					return nil
				}, p)
				if err != nil {
					t.Fatal(err)
				}
				return only(t, r)
			}
			p := params()
			p.DetailsFor = nil
			inferences := 0
			p.Profiles = []SpecProfile{{ID: "INVENTED.first", CastSignatures: []SpellSignature{{ID: 55}}}}
			p.InferSpecs = func([]model.Event, []SpecProfile) map[string]string {
				inferences++
				return map[string]string{alia.ID: "INVENTED.first"}
			}
			index := call(p)
			if len(index.Fights) != 2 || len(index.Details) != 2 {
				t.Fatal("index lost fights or sidecars")
			}
			for i, f := range index.Fights {
				d := index.Details[i]
				if len(f.Events) != 0 || len(d.Timelines) != 0 || len(f.Specs) != 0 || len(d.Specs) != 0 || len(f.Roles) != 0 || d.UnscoredReason != "detailNotSelected" {
					t.Fatal("nil DetailsFor retained/computed detail instead of index only")
				}
				if f.Len != 100 || f.Offset != int64(i*100) || f.Partial || len(f.Players) != 2 {
					t.Fatal("index lost byte range, completeness or participants")
				}
			}
			if inferences != 0 {
				t.Fatal("index mode ran detail spec inference")
			}
			selectedID := index.Fights[1].ID
			p.DetailsFor = func(f model.Fight) bool { return f.ID == selectedID }
			selected := call(p)
			if selected.NightID != index.NightID || inferences != 1 {
				t.Fatal("explicit selection changed identity or computed unrequested detail")
			}
			for i, f := range selected.Fights {
				if f.ID != index.Fights[i].ID || f.Offset != index.Fights[i].Offset || f.Len != index.Fights[i].Len {
					t.Fatal("explicit selection changed index metadata")
				}
				if (len(f.Events) > 0) != (i == 1) || (len(selected.Details[i].Timelines) > 0) != (i == 1) || (len(f.Specs) > 0) != (i == 1) {
					t.Fatal("detail not restricted to the explicitly named fight")
				}
			}
		})
	}
}
