package fights

import (
	"fmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"io"
	"runtime"
	"strings"
	"testing"
)

func TestReadClosesFightsWhileDecoding(t *testing.T) {
	closed := 0
	p := params()
	p.DetailsFor = func(model.Fight) bool { closed++; return false }
	decode := func(_ io.Reader, emit func(model.Event)) error {
		for i := 0; i < 3; i++ {
			for _, e := range boss(int64(7+i), int64(i)*100000) {
				emit(e)
			}
			emit(model.Event{TimeMs: int64(i)*100000 + 60000})
			if closed != i+1 {
				t.Fatalf("closed=%d while decoding, want=%d", closed, i+1)
			}
		}
		return nil
	}
	if _, err := Read(strings.NewReader(""), decode, p); err != nil {
		t.Fatal(err)
	}
}
func TestReadRetainedHeapBoundedPerFight(t *testing.T) {
	if testing.Short() {
		t.Skip("retained-heap stress")
	}
	runtime.GC()
	var base, m runtime.MemStats
	var retainedPeak uint64
	runtime.ReadMemStats(&base)
	p := params()
	p.DetailsFor = nil // The default must release closed fights without a selector.
	decode := func(_ io.Reader, emit func(model.Event)) error {
		for i := 0; i < 120; i++ {
			es := boss(int64(i+1), int64(i)*100000)
			for _, e := range es[:4] {
				emit(e)
			}
			for j := 0; j < 256; j++ {
				e := hit(int64(i)*100000 + int64(j) + 100)
				e.Raw = []string{fmt.Sprintf("%d/%d/%s", i, j, strings.Repeat("x", 1024))}
				emit(e)
			}
			emit(es[4])
			emit(model.Event{TimeMs: int64(i)*100000 + 60000})
			if i%20 == 19 {
				runtime.GC()
				runtime.ReadMemStats(&m)
				retainedPeak = max(retainedPeak, m.HeapAlloc)
				if m.HeapAlloc > base.HeapAlloc+12*1024*1024 {
					t.Fatalf("retained heap grew with discarded events: delta=%d limit=12582912", m.HeapAlloc-base.HeapAlloc)
				}
			}
		}
		return nil
	}
	r, err := Read(strings.NewReader(""), decode, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Runs) != 1 || len(r.Runs[0].Fights) != 120 {
		t.Fatal("lost fights")
	}
	t.Logf("120 fights retained-heap peak delta=%d limit=12582912", int64(retainedPeak)-int64(base.HeapAlloc))
	runtime.KeepAlive(r)
}
func TestLateOwnershipAcrossClosedFights(t *testing.T) {
	pet := modeltest.Pet("Invented late ward", alia)
	pet.OwnerID = ""
	for _, selected := range []bool{false, true} {
		p := params()
		p.DetailsFor = func(model.Fight) bool { return selected }
		es := boss(7, 1000)
		es[3].Src = pet
		es[3].InfoUnit, es[3].HPMax = giant.ID, model.Some(900)
		next := boss(8, 100000)
		next[3].Src = pet
		next[3].InfoUnit = pet.ID
		next[3].InfoOwnerID = alia.ID
		r := only(t, build(t, append(es, next...), p))
		if r.Fights[0].Owners[pet.ID] != alia.ID || len(r.Fights[0].Unowned) != 0 {
			t.Fatal("late run ownership lost")
		}
		if len(r.Fights[0].BossUnits) != 1 || r.Fights[0].BossUnits[0] != giant.ID {
			t.Fatal("late owner lost boss selection")
		}
		if selected && r.Details[0].Timelines[alia.ID].DamageDone != 100 {
			t.Fatal("late damage attribution lost")
		}
	}
}
