package logfmt_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

var alia = model.Actor{ID: "Player-999-FA001ABC", Name: "Alia-Stone-PvP-EU", Flags: 0x511}
var bram = model.Actor{ID: "Player-999-FA002ABC", Name: "Bram-Oak-Hill-PvP-EU", Flags: 0x512}

func common() []fixture.Field { return append(fixture.Actor(alia), fixture.Actor(bram)...) }
func spell() []fixture.Field {
	return []fixture.Field{fixture.Number(900001), fixture.Text("Practice Bolt"), "0x4"}
}
func nums(v ...int64) []fixture.Field {
	out := make([]fixture.Field, len(v))
	for i, n := range v {
		out[i] = fixture.Number(n)
	}
	return out
}
func advanced(unit string) []fixture.Field {
	return fixture.Advanced(model.Event{InfoUnit: unit, InfoOwnerID: alia.ID, HP: model.Some(0), HPMax: model.Some(900), X: model.Some(-125), Y: model.Some(201)})
}
func damage(t *testing.T, name string, adv bool) string {
	f := common()
	if name == "ENVIRONMENTAL_DAMAGE" {
		f = append(fixture.Actor(model.Actor{}), fixture.Actor(bram)...)
	} else if name != "SWING_DAMAGE" && name != "SWING_DAMAGE_LANDED" {
		f = append(f, spell()...)
	}
	who := bram.ID
	if name == "SWING_DAMAGE" {
		who = alia.ID
	}
	if adv {
		f = append(f, advanced(who)...)
	}
	if name == "ENVIRONMENTAL_DAMAGE" {
		f = append(f, "FALLING")
	}
	f = append(f, nums(87, 99, -1, 4, 0, 3, 7, 1, 0, 0)...)
	if name != "SWING_DAMAGE" && name != "SWING_DAMAGE_LANDED" && name != "ENVIRONMENTAL_DAMAGE" {
		f = append(f, "ST")
	}
	return line(t, name, f...)
}

func TestDamageLayoutsAndAdvancedAttribution(t *testing.T) {
	for _, name := range []string{"SWING_DAMAGE", "SWING_DAMAGE_LANDED", "SPELL_DAMAGE", "SPELL_PERIODIC_DAMAGE", "RANGE_DAMAGE", "DAMAGE_SHIELD", "DAMAGE_SPLIT", "ENVIRONMENTAL_DAMAGE"} {
		for _, adv := range []bool{true, false} {
			events, r := decode(damage(t, name, adv))
			balanced(t, r)
			if len(events) != 1 {
				t.Fatalf("%s advanced=%v: %+v", name, adv, r)
			}
			e := events[0]
			if e.Kind != model.Damage || e.Amount != model.Some(87) || e.Overkill != model.Some(-1) || e.Absorbed != model.Some(7) || !e.Crit || len(e.Raw) != 0 || e.Dst != bram {
				t.Fatalf("%s: %+v", name, e)
			}
			stream := model.StreamNone
			if name == "SWING_DAMAGE" {
				stream = model.SwingSource
			}
			if name == "SWING_DAMAGE_LANDED" {
				stream = model.SwingLanded
			}
			if e.Stream != stream {
				t.Fatalf("%s stream=%v", name, e.Stream)
			}
			if adv {
				who := bram.ID
				if name == "SWING_DAMAGE" {
					who = alia.ID
				}
				if e.InfoUnit != who || e.InfoOwnerID != alia.ID || e.HP != model.Some(0) || e.HPMax != model.Some(900) || e.X != model.Some(-125) || e.Y != model.Some(201) {
					t.Fatalf("%s advanced=%+v", name, e)
				}
			} else if e.HP.Present || e.HPMax.Present || e.X.Present || e.Y.Present || e.InfoUnit != "" {
				t.Fatalf("nonadvanced=%+v", e)
			}
			if name == "ENVIRONMENTAL_DAMAGE" {
				if e.Src != (model.Actor{}) || e.SpellID != 0 || e.SpellName != "FALLING" {
					t.Fatal(e)
				}
			} else if stream == model.StreamNone && (e.SpellID != 900001 || e.SpellName != "Practice Bolt") {
				t.Fatal(e)
			}
		}
	}
}

func TestKnownWidthSkippedAndOptionalNumbersStayUnknown(t *testing.T) {
	f := append(common(), spell()...)
	a := advanced(bram.ID)
	a[2] = fixture.Nil
	a[3] = "?"
	a[14] = "1|2"
	a[15] = "NaN"
	f = append(f, a...)
	f = append(f, fixture.Nil, "99", "?", "4", "0", "0", "nil", "nil", "nil", "nil", "ST")
	s := line(t, "SPELL_DAMAGE", f...) + line(t, "SPELL_DAMAGE", append(f, "extra")...) + damage(t, "SWING_DAMAGE_LANDED", true)
	events, r := decode(s)
	balanced(t, r)
	if len(events) != 2 || r.Skipped["unknown field count"] != 1 || r.EventCounts["SPELL_DAMAGE"] != 2 {
		t.Fatal(r)
	}
	e := events[0]
	if e.Amount.Present || e.Overkill.Present || e.Absorbed.Present || e.HP.Present || e.HPMax.Present || e.X.Present || e.Y.Present {
		t.Fatal(e)
	}
}
