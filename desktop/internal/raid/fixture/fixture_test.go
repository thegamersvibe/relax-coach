package fixture

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestTypedFixtureFields(t *testing.T) {
	actor := model.Actor{ID: "Player-999-FA001ABC", Name: "Alia-Stone-PvP-EU", Flags: 0x511}
	if got := join(Actor(actor)); got != `Player-999-FA001ABC,"Alia-Stone-PvP-EU",0x511,0x0` {
		t.Fatal(got)
	}
	e := model.Event{InfoUnit: actor.ID, InfoOwnerID: "Player-999-FA002ABC", HP: model.Some(0), HPMax: model.Some(700), X: model.Some(-125), Y: model.Some(201)}
	a := Advanced(e)
	if len(a) != 19 || a[0] != Field(actor.ID) || a[1] != Field(e.InfoOwnerID) || a[2] != "0" || a[3] != "700" || a[14] != "-1.25" || a[15] != "2.01" {
		t.Fatal(a)
	}
	c := model.Combatant{PlayerID: actor.ID, Stats: make([]int64, 24), Talents: [][3]int64{{101, 102, 3}}, Gear: []model.GearSlot{{ItemID: 901, ItemLevel: 60, Extra: []string{"(1,0,0)", "[2]", "()"}}}, Auras: []model.Aura{{CasterID: actor.ID, SpellID: 103, Stacks: 2}}}
	fields := Combatant(c)
	if len(fields) != 33 || fields[25] != "[(101,102,3)]" || fields[27] != "[(901,60,(1,0,0),[2],())]" || fields[28] != Field("["+actor.ID+",103,2]") {
		t.Fatal(fields)
	}
}

func TestWriteReportedEscapingAndOffsets(t *testing.T) {
	// Escaping is format-reported, not observed in the private live logs.
	at := time.Date(2026, 6, 7, 8, 9, 10, 123000000, time.UTC)
	for _, offset := range []int8{2, -4, 10} {
		var b bytes.Buffer
		err := Write(&b, at, offset, "INVENTED_EVENT", Text("Alia-Stone-PvP-EU"), Text("Bram, \"Oak\"\\Hill-PvP-EU"), List(Number(7), Tuple(Number(8), Number(9))))
		suffix := map[int8]string{2: "2", -4: "-4", 10: "10"}[offset]
		want := "6/7/2026 8:09:10.123" + suffix + "  INVENTED_EVENT,\"Alia-Stone-PvP-EU\",\"Bram, \\\"Oak\\\"\\\\Hill-PvP-EU\",[7,(8,9)]\r\n"
		if err != nil || b.String() != want {
			t.Fatalf("offset %d: got %q (%v), want %q", offset, b.String(), err, want)
		}
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestWriteRejectsShortWrite(t *testing.T) {
	if err := Write(shortWriter{}, time.Time{}, 0, "INVENTED_EVENT"); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("got %v", err)
	}
}
func TestFixturePowerFields(t *testing.T) {
	a := Advanced(model.Event{Power: model.Some(0), PowerMax: model.Some(900), PowerType: model.Some(0)})
	if a[10] != "0" || a[11] != "0" || a[12] != "900" {
		t.Fatal(a)
	}
	a = Advanced(model.Event{})
	if a[10] != Nil || a[11] != Nil || a[12] != Nil {
		t.Fatal(a)
	}
}

func TestAbsentFixtureFields(t *testing.T) {
	a := Advanced(model.Event{})
	for _, i := range []int{2, 3, 14, 15} {
		if a[i] != Nil {
			t.Fatal(a)
		}
	}
	if Actor(model.Actor{})[1] != Nil {
		t.Fatal("absent name not nil")
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestWritePropagatesFailure(t *testing.T) {
	if err := Write(failedWriter{}, time.Time{}, 0, "INVENTED_EVENT"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("got %v", err)
	}
}
