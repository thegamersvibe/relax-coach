package logfmt_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestLineCapInclusiveBoundary(t *testing.T) {
	const capBytes = 1 << 20
	prefix := strings.TrimSuffix(line(t, "INVENTED_EVENT"), "\r\n") + ","
	next := damage(t, "SWING_DAMAGE", false)
	for _, ending := range []string{"\n", "\r\n"} {
		for _, size := range []int{capBytes - 1, capBytes, capBytes + 1} {
			t.Run(fmt.Sprintf("ending=%d/bytes=%d", len(ending), size), func(t *testing.T) {
				row := prefix + strings.Repeat("x", size-len(prefix)-len(ending)) + ending
				events, r := decode(row + next)
				balanced(t, r)
				want := 2
				if size > capBytes {
					want = 1
				}
				if len(events) != want || r.Skipped[logfmt.LineTooLong] != int64(2-want) {
					t.Fatalf("inclusive cap: yielded=%d skipped=%v", len(events), r.Skipped)
				}
				last := events[len(events)-1]
				if last.Offset != int64(size) || last.Line != 2 || last.Len != int64(len(next)) {
					t.Fatal("next line lost its byte range")
				}
				if size <= capBytes && events[0].Len != int64(size) {
					t.Fatal("line ending excluded from length")
				}
			})
		}
	}
}

func TestOffsetBoundsAndUnsignedPositive(t *testing.T) {
	good := line(t, "INVENTED_EVENT", "1")
	for _, tc := range []struct {
		suffix string
		valid  bool
		offset int8
	}{
		{"-24", false, 0}, {"-23", true, -23}, {"0", true, 0}, {"23", true, 23}, {"24", false, 0}, {"+2", false, 0}, {"+23", false, 0},
	} {
		events, r := decode(strings.Replace(good, ".1232  ", ".123"+tc.suffix+"  ", 1))
		balanced(t, r)
		if tc.valid {
			if len(events) != 1 || events[0].TimeMs != at.UnixMilli() || events[0].OffsetH != tc.offset || len(r.Skipped) != 0 {
				t.Errorf("valid offset %s changed", tc.suffix)
			}
		} else if len(events) != 0 || r.Skipped[logfmt.InvalidTime] != 1 {
			t.Errorf("invalid offset %s accepted", tc.suffix)
		}
	}
}

func TestReportLastFollowsFileOrder(t *testing.T) {
	var b bytes.Buffer
	for _, tc := range []struct {
		delta  time.Duration
		offset int8
		name   string
	}{
		{time.Hour, 2, "INVENTED_EVENT"}, {2 * time.Hour, 10, "INVENTED_EVENT"}, {-time.Hour, -23, "SPELL_DAMAGE"},
	} {
		if err := fixture.Write(&b, at.Add(tc.delta), tc.offset, tc.name, "1"); err != nil {
			t.Fatal(err)
		}
	}
	b.WriteString(strings.Replace(line(t, "INVENTED_EVENT"), ".1232  ", ".123+2  ", 1))
	_, r := decode(b.String())
	balanced(t, r)
	if r.First == nil || r.Last == nil || r.First.TimeMs != at.Add(time.Hour).UnixMilli() || r.First.OffsetH != 2 || r.Last.TimeMs != at.Add(-time.Hour).UnixMilli() || r.Last.OffsetH != -23 || r.Skipped[logfmt.UnknownWidth] != 1 || r.Skipped[logfmt.InvalidTime] != 1 {
		t.Fatal("First/Last must follow valid timestamp observations, including skipped layouts", r)
	}
}

func TestFractionalNumbersStayUnknown(t *testing.T) {
	for _, tc := range []struct {
		wire fixture.Field
		want model.Opt
	}{
		{"1.25", model.Opt{}}, {"-0.25", model.Opt{}}, {"0.0001", model.Opt{}}, {"1.", model.Opt{}}, {"8.0000", model.Some(8)}, {"0.0000", model.Some(0)},
	} {
		f := append(append(common(), spell()...), tc.wire, "0", "0", "50")
		events, r := decode(line(t, "SPELL_ENERGIZE", f...))
		if len(events) != 1 || events[0].Amount != tc.want {
			t.Errorf("amount %s not preserved/unknown: %v", tc.wire, r)
		}
	}
}

func TestNumbersKeepInt64Range(t *testing.T) {
	for _, n := range []int64{1 << 31, -1<<31 - 1, 1<<63 - 1, -1 << 63} {
		f := append(common(), spell()...)
		f = append(f, fixture.Number(n), "0", "0", "50")
		events, r := decode(line(t, "SPELL_ENERGIZE", f...))
		if len(events) != 1 || events[0].Amount != model.Some(n) {
			t.Errorf("int64 %d narrowed: %v", n, r)
		}
	}
}

func TestAbsentUnitIDs(t *testing.T) {
	f := append(fixture.Actor(model.Actor{}), fixture.Actor(model.Actor{})...)
	block := fixture.Advanced(model.Event{})
	f = append(f, block...)
	f = append(f, nums(1, 1, -1, 1, 0, 0, 0, 0, 0, 0)...)
	events, r := decode(line(t, "SWING_DAMAGE", f...))
	if len(events) != 1 || events[0].Src.ID != "" || events[0].Dst.ID != "" || events[0].InfoUnit != "" || events[0].InfoOwnerID != "" {
		t.Fatal("zero unit sentinel retained", r)
	}
}

func TestEnvironmentalSourceCleared(t *testing.T) {
	// A non-empty source makes the clearing rule observable.
	f := append(common(), "FALLING")
	f = append(f, nums(7, 8, -1, 1, 0, 0, 0, 0, 0, 0)...)
	events, r := decode(line(t, "ENVIRONMENTAL_DAMAGE", f...))
	if len(events) != 1 || events[0].Src != (model.Actor{}) || events[0].Dst != bram {
		t.Fatal("environment inherited a source", r)
	}
}

func TestHeaderRequiresNamedKeys(t *testing.T) {
	for _, key := range []string{"ADVANCED_LOG_ENABLED", "BUILD_VERSION", "PROJECT_ID"} {
		events, r := decode(strings.Replace(header(t, "1"), key, "WRONG_KEY", 1))
		balanced(t, r)
		if len(events) != 0 || r.Header != nil || r.HeadersSeen != 0 || r.Skipped[logfmt.InvalidValue] != 1 {
			t.Errorf("header accepted wrong %s", key)
		}
	}
}

func TestCombatantGearTupleWidth(t *testing.T) {
	for _, gear := range []fixture.Field{"[(800001,60,(),[])]", "[(800001,60,(),[],(),17)]"} {
		f := fixture.Combatant(combatant())
		f[27] = gear
		events, r := decode(line(t, "COMBATANT_INFO", f...))
		balanced(t, r)
		if len(events) != 0 || r.Skipped[logfmt.InvalidValue] != 1 {
			t.Fatal("non-five-field gear accepted")
		}
	}
}

func TestCombatantAuraTriples(t *testing.T) {
	// Balanced lists, but incomplete triples: the list tokenizer cannot mask
	// the semantic arity guard. A missing guard must not index past the list.
	for _, aura := range []fixture.Field{
		fixture.List(fixture.Field(cato.ID)),
		fixture.List(fixture.Field(cato.ID), "900003"),
		fixture.List(fixture.Field(cato.ID), "900003", "1", fixture.Field(alia.ID)),
	} {
		func() {
			defer func() {
				if recover() != nil {
					t.Error("incomplete aura triple panicked")
				}
			}()
			f := fixture.Combatant(combatant())
			f[28] = aura
			events, r := decode(line(t, "COMBATANT_INFO", f...))
			balanced(t, r)
			if len(events) != 0 || r.Skipped[logfmt.InvalidValue] != 1 {
				t.Error("incomplete aura triple accepted")
			}
		}()
	}
}

func TestZoneDifficultyDistinctFromInstance(t *testing.T) {
	events, r := decode(line(t, "ZONE_CHANGE", "555", fixture.Text("Practice Town"), "2"))
	if len(events) != 1 || events[0].Zone == nil || events[0].Zone.InstanceID != 555 || events[0].Zone.Difficulty != 2 {
		t.Fatal("zone difficulty replaced by instance", r)
	}
}

func TestActorNameNewlineEscape(t *testing.T) {
	f := append(common(), spell()...)
	// Wire token is independent of fixture.Text's escape implementation.
	f[1] = `"Alia\nStone-PvP-EU"`
	events, r := decode(line(t, "SPELL_CAST_START", f...))
	if len(events) != 1 || events[0].Src.Name != "Alia\nStone-PvP-EU" {
		t.Fatal("newline escape changed", r)
	}
}

func TestAuraBrokenSpellRejectsShortWidth(t *testing.T) {
	f := append(append(common(), spell()...), "DEBUFF")
	events, r := decode(line(t, "SPELL_AURA_BROKEN_SPELL", f...))
	balanced(t, r)
	if len(events) != 0 || r.Skipped[logfmt.UnknownWidth] != 1 {
		t.Fatal("short aura-broken-spell accepted")
	}
}

func TestTypeOnlyMultiPowerCounted(t *testing.T) {
	f := append(common(), spell()...)
	block := advanced(bram.ID)
	block[10], block[11], block[12] = "3|4", "40", "100"
	events, r := decode(line(t, "SPELL_CAST_SUCCESS", append(f, block...)...))
	balanced(t, r)
	if len(events) != 1 || events[0].PowerType != model.Some(3) || events[0].Power != model.Some(40) || events[0].PowerMax != model.Some(100) || r.MultiPower != 1 {
		t.Fatal("type-only multi-power not counted/kept", r)
	}
}
