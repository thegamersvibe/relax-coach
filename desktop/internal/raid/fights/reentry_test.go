package fights

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func instanceBoss(id, start, instance, size int64) []model.Event {
	es := boss(id, start)
	es[0].Enc.InstanceID = model.Some(instance)
	es[0].Enc.GroupSize, es[len(es)-1].Enc.GroupSize = size, size
	return es
}

func TestReentryGapBoundary(t *testing.T) {
	for _, tc := range []struct {
		name       string
		gap, limit int64
		runs       int
	}{
		{"below-default", 1800000 - 1, 1800000, 1},
		{"at-default", 1800000, 1800000, 2},
		{"thirty-one-minutes", 31 * 60 * 1000, 1800000, 2},
		{"configured-below", 299999, 300000, 1},
		{"configured-at", 300000, 300000, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := boss(7, 1000)
			// Measure from the zone-out, not the last combat/inside event.
			out := first[len(first)-1].TimeMs + 20*60*1000
			back := out + tc.gap
			es := append(first, modeltest.At(out).Zone(0, "Invented Outside", 0).Build(),
				modeltest.At(out+tc.gap/2).Zone(0, "Invented Road", 0).Build(),
				modeltest.At(back).Zone(42, "Invented Halls", 2).Build())
			es = append(es, boss(8, back+1)...)
			p := params()
			p.ReentryGapMs = tc.limit
			r := build(t, es, p)
			if len(r.Runs) != tc.runs {
				t.Fatalf("gap=%d limit=%d: runs=%d want=%d", tc.gap, tc.limit, len(r.Runs), tc.runs)
			}
			if tc.runs == 2 && (r.Runs[0].EndMs != out || r.Runs[0].Partial || r.Runs[1].StartMs != back+1) {
				t.Fatal("expired return moved the old exit or the new start")
			}
		})
	}
}

func TestReentryDifferentInstance(t *testing.T) {
	for _, instance := range []int64{43, 98} {
		for _, hasFights := range []bool{false, true} {
			t.Run(fmt.Sprintf("instance-%d/fights-%v", instance, hasFights), func(t *testing.T) {
				first := boss(7, 1000)
				out := first[len(first)-1].TimeMs + 1
				es := append(first, modeltest.At(out).Zone(0, "Invented Outside", 0).Build(),
					modeltest.At(out+1).Zone(instance, "Invented Detour", 2).Build())
				wantInstances := []int64{42}
				if hasFights {
					es = append(es, instanceBoss(9, out+2, instance, 20)...)
					wantInstances = append(wantInstances, instance)
				}
				es = append(es, modeltest.At(out+60000).Zone(0, "Invented Road", 0).Build(),
					modeltest.At(out+300000).Zone(42, "Invented Halls", 2).Build())
				es = append(es, boss(8, out+300001)...)
				wantInstances = append(wantInstances, 42)
				r := build(t, es, params())
				if len(r.Runs) != len(wantInstances) {
					t.Fatalf("runs=%d want=%d (empty visits do not emit runs)", len(r.Runs), len(wantInstances))
				}
				for i, want := range wantInstances {
					if r.Runs[i].InstanceID != want || len(r.Runs[i].Fights) != 1 {
						t.Fatalf("run %d: instance=%d fights=%d", i, r.Runs[i].InstanceID, len(r.Runs[i].Fights))
					}
				}
				if r.Runs[0].EndMs != out || r.Runs[0].NightID == r.Runs[len(r.Runs)-1].NightID {
					t.Fatal("different instance retained the previous night")
				}
			})
		}
	}
}

func TestReentryClosesFightAtExit(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(fmt.Sprintf("complete-%v", complete), func(t *testing.T) {
			first := boss(7, 1000)
			if !complete {
				first = first[:4]
			}
			out := first[len(first)-1].TimeMs + 1
			es := append(first, modeltest.At(out).Zone(0, "Invented Outside", 0).Build(),
				hit(out+1), modeltest.At(out+2).Zone(42, "Invented Halls", 2).Build(), hit(out+3),
				modeltest.At(out+4).Zone(0, "Invented Outside", 0).Build())
			r := only(t, build(t, es, params()))
			if len(r.Fights) != 2 || !reflect.DeepEqual(r.Fights[0].Events, first) || len(r.Fights[1].Events) != 1 || r.Fights[1].StartMs != out+3 {
				t.Fatal("fight or boss tail crossed an exit")
			}
			if r.Fights[0].Partial == complete || r.Fights[0].EndMs != first[len(first)-1].TimeMs || r.EndMs != out+4 {
				t.Fatal("exit lost completeness or moved the last inside fight event")
			}
		})
	}
}

func TestReentryHeaderResetsRun(t *testing.T) {
	es := boss(7, 1000)
	out := es[len(es)-1].TimeMs + 1
	es = append(es, modeltest.At(out).Zone(0, "Invented Outside", 0).Build(),
		model.Event{Kind: model.Header, TimeMs: out + 1},
		modeltest.At(out+2).Zone(42, "Invented Halls", 2).Build())
	es = append(es, boss(8, out+3)...)
	r := build(t, es, params())
	if len(r.Runs) != 2 || r.Runs[0].EndMs != out || r.Runs[0].Partial {
		t.Fatal("header resumed a pending run or moved its exit")
	}
}

func TestOpenWorldZoneAfterUnknownEncounter(t *testing.T) {
	es := instanceBoss(7, 1000, 0, 5)
	out := es[len(es)-1].TimeMs + 100
	es = append(es, modeltest.At(out).Zone(0, "Invented Outside", 0).Build())
	es = append(es, boss(8, out+1)...)
	r := only(t, build(t, es, params()))
	if len(r.Fights) != 1 || r.EndMs != out {
		t.Fatal("an inferred zero instance kept outside encounters enabled")
	}
}

func TestReentryGapMustBePositive(t *testing.T) {
	for _, gap := range []int64{0, -1} {
		p := params()
		p.ReentryGapMs = gap
		if _, err := Build(nil, p); err == nil {
			t.Errorf("invalid re-entry gap %d accepted", gap)
		}
	}
}

func TestReentryFiveMinutesOneRun(t *testing.T) {
	for _, tc := range []struct {
		name     string
		instance int64
		size     int64
		zone     bool
	}{
		{"dungeon", 42, 5, true},
		{"raid", 43, 20, true},
		{"unlisted-raid", 99, 10, true},
		{"inferred-dungeon", 42, 5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := instanceBoss(7, 1000, tc.instance, tc.size)
			out := first[len(first)-1].TimeMs + 100
			back := out + 5*60*1000
			second := instanceBoss(8, back+1, tc.instance, tc.size)
			finalOut := second[len(second)-1].TimeMs + 100
			var es []model.Event
			if tc.zone {
				es = append(es, modeltest.At(1).Zone(tc.instance, "Invented Halls", 2).Build())
			}
			es = append(es, first...)
			es = append(es, modeltest.At(out).Zone(0, "Invented Outside", 0).Build(), hit(out+1))
			// Even encounter lines outside cannot create a fight or influence the run.
			es = append(es, instanceBoss(9, out+2, 98, 40)...)
			es = append(es, modeltest.At(out+100000).Zone(0, "Invented Road", 0).Build(),
				modeltest.At(back).Zone(tc.instance, "Invented Return", 2).Build())
			es = append(es, second...)
			es = append(es, modeltest.At(finalOut).Zone(0, "Invented Outside", 0).Build(),
				hit(finalOut+1), modeltest.At(finalOut+252000).Zone(0, "Invented Road", 0).Build(), hit(finalOut+600000))

			r := only(t, build(t, es, params()))
			if r.InstanceID != tc.instance || r.StartMs != first[0].TimeMs || r.EndMs != finalOut || r.Partial || len(r.Fights) != 2 {
				t.Fatalf("re-entry run: instance=%d start=%d end=%d partial=%v fights=%d", r.InstanceID, r.StartMs, r.EndMs, r.Partial, len(r.Fights))
			}
			for i, want := range [][]model.Event{first, second} {
				if r.Fights[i].Partial || !reflect.DeepEqual(r.Fights[i].Events, want) {
					t.Fatalf("fight %d lost inside events or admitted outside/zone events", i)
				}
			}
			ids := []string{r.Fights[0].ID, r.Fights[1].ID}
			sort.Strings(ids)
			wantNight := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(ids, "\n"))))
			continuous := only(t, build(t, append(first, second...), params()))
			if r.NightID != wantNight || r.NightID != continuous.NightID {
				t.Fatal("re-entry changed the sorted complete-boss night identity")
			}
		})
	}
}
