package fights

import (
	"crypto/sha256"
	"fmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRepeatBossIdentity(t *testing.T) {
	ids := []string{alia.ID, bram.ID}
	sort.Strings(ids)
	text := "boss\n42\n7\n2\n5\n0\n50000\n" + strings.Join(ids, "\n")
	hash := func(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
	wipe := func(start int64) []model.Event { es := boss(7, start); es[4].Enc.Success = model.Some(0); return es }
	for _, n := range []int{2, 3} {
		var es []model.Event
		for i := 0; i < n; i++ {
			es = append(es, wipe(int64(i+1)*100000)...)
		}
		decode := func(_ io.Reader, emit func(model.Event)) error {
			for _, e := range es {
				emit(e)
			}
			return nil
		}
		read := func() Result {
			r, err := Read(strings.NewReader("synthetic"), decode, params())
			if err != nil {
				t.Fatal(err)
			}
			return r
		}
		res := read()
		r := only(t, res)
		var got []string
		for i, f := range r.Fights {
			want := text
			if i > 0 {
				want += fmt.Sprintf("\nrepeat %d", i+1)
			}
			if f.ID != hash(want) {
				t.Fatalf("repeat %d has wrong identity", i+1)
			}
			got = append(got, f.ID)
		}
		sort.Strings(got)
		if r.NightID != hash(strings.Join(got, "\n")) {
			t.Fatal("night omitted repeat")
		}
		if !reflect.DeepEqual(res, read()) {
			t.Fatal("repeat read unstable")
		}
	}
	second := wipe(200000)
	if only(t, build(t, second, params())).Fights[0].ID != hash(text) {
		t.Fatal("standalone second pull must use legacy identity")
	}
	second[4].Enc.FightMs = model.Some(50001)
	r := only(t, build(t, append(wipe(100000), second...), params()))
	if r.Fights[1].ID != hash(strings.Replace(text, "\n50000\n", "\n50001\n", 1)) {
		t.Fatal("different duration wrongly repeated")
	}
	// File-local sequence continues across headers, but partials do not consume it.
	es := append(wipe(1)[:4], model.Event{Kind: model.Header, TimeMs: 60000})
	es = append(es, wipe(100000)...)
	es = append(es, model.Event{Kind: model.Header, TimeMs: 180000})
	es = append(es, wipe(200000)...)
	rs := build(t, es, params()).Runs
	if rs[1].Fights[0].ID != hash(text) || rs[2].Fights[0].ID != hash(text+"\nrepeat 2") {
		t.Fatal("repeat scope must be complete fights per file")
	}
}
