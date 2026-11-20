package fights

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"io"
	"strings"
	"testing"
)

func TestReadRawSourceAndErrors(t *testing.T) {
	raw := "invented header\r\nignored raw line\nno ending"
	decode := func(r io.Reader, emit func(model.Event)) error {
		if _, err := io.Copy(io.Discard, r); err != nil {
			return err
		}
		for _, e := range boss(7, 1000) {
			emit(e)
		}
		return nil
	}
	r, err := Read(strings.NewReader(raw), decode, params())
	if err != nil {
		t.Fatal(err)
	}
	if r.Source.Bytes != int64(len(raw)) || r.Source.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) || len(r.Runs) != 1 {
		t.Fatal("wrong source")
	}
	// Hash bytes that a decoder elects not to consume as well.
	early := func(r io.Reader, emit func(model.Event)) error {
		b := make([]byte, 3)
		_, err := io.ReadFull(r, b)
		return err
	}
	r, err = Read(strings.NewReader(raw), early, params())
	if err != nil || r.Source.Bytes != int64(len(raw)) {
		t.Fatal("partial source hash")
	}
	sentinel := errors.New("invented failure")
	fail := func(io.Reader, func(model.Event)) error { return sentinel }
	r, err = Read(strings.NewReader(raw), fail, params())
	if !errors.Is(err, sentinel) || len(r.Runs) > 0 || r.Source.SHA256 != "" {
		t.Fatal("decoder failure published a result")
	}
	_, err = Read(failedReader{}, early, params())
	if err == nil {
		t.Fatal("read failure ignored")
	}
	if _, err = Read(strings.NewReader(raw), nil, params()); err == nil {
		t.Fatal("nil decoder accepted")
	}
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestTypedRunsAndSelectedDetails(t *testing.T) {
	p := params()
	p.DetailsFor = func(f model.Fight) bool { return f.EncounterID == 8 }
	r := only(t, build(t, append(boss(7, 1000), boss(8, 100000)...), p))
	if _, ok := r.RaidNight(); ok {
		t.Fatal("dungeon became raid")
	}
	d, ok := r.DungeonRun()
	if !ok || len(d.Fights) != 2 || d.InstanceID != 42 {
		t.Fatal("dungeon conversion")
	}
	if len(r.Fights[0].Events) != 0 || len(r.Details[0].Timelines) != 0 || len(r.Fights[1].Events) == 0 {
		t.Fatal("detail selection")
	}
	es := boss(7, 1000)
	es[0].Enc.InstanceID = model.Some(43)
	raid := only(t, build(t, es, params()))
	if _, ok := raid.DungeonRun(); ok {
		t.Fatal("raid became dungeon")
	}
	if _, ok := raid.RaidNight(); !ok {
		t.Fatal("raid conversion")
	}
	es[0].Enc.InstanceID = model.Some(99)
	u := only(t, build(t, es, params()))
	if _, ok := u.RaidNight(); ok {
		t.Fatal("unknown scored")
	}
	if _, ok := u.DungeonRun(); ok {
		t.Fatal("unknown scored")
	}
}
