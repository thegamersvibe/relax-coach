package logfmt_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func nameCounts(t *testing.T, r logfmt.ParseReport, nulBytes, invalidNames int) {
	t.Helper()
	if r.NameNULBytes != int64(nulBytes) || r.InvalidUTF8Names != int64(invalidNames) {
		t.Errorf("name counters: NUL bytes=%d invalid UTF-8 names=%d", r.NameNULBytes, r.InvalidUTF8Names)
	}
	for _, want := range []string{
		fmt.Sprintf("Actor name NUL bytes: %d\n", nulBytes),
		fmt.Sprintf("Actor names with invalid UTF-8: %d\n", invalidNames),
	} {
		if !strings.Contains(r.String(), want) {
			t.Errorf("missing name counter %q", want)
		}
	}
}

func TestActorNameAnomaliesPreserved(t *testing.T) {
	src := "Alia-\x00\x00\xff-Stone-PvP-EU"
	dst := "Bram-\xfe-Oak-Hill-PvP-EU"
	caster := "Cato-\x00-PvP-EU"
	for _, withSpell := range []bool{false, true} {
		f := absorbFields(withSpell)
		f[1], f[5] = fixture.Text(src), fixture.Text(dst)
		i := 9
		if withSpell {
			i += 3
		}
		f[i] = fixture.Text(caster)
		events, r := decode(line(t, "SPELL_ABSORBED", f...))
		balanced(t, r)
		if len(events) != 1 || len(r.Skipped) != 0 {
			t.Fatalf("name bytes dropped event: %v", r)
		}
		e := events[0]
		if e.Src.Name != src || e.Dst.Name != dst || e.Caster.Name != caster {
			t.Error("actor name bytes were altered")
		}
		nameCounts(t, r, 3, 2)
		if strings.ContainsAny(r.String(), "\x00\xff\xfe") {
			t.Error("name bytes leaked into report")
		}
	}
	// Valid multibyte text, including a literal replacement character, is not
	// invalid UTF-8. A repeated anomalous name counts once per actor occurrence.
	f := append(common(), spell()...)
	f[1], f[5] = fixture.Text("Alia-آليا-�-PvP-EU"), fixture.Text("Bram-É-PvP-EU")
	events, r := decode(line(t, "SPELL_CAST_START", f...))
	if len(events) != 1 || events[0].Src.Name != "Alia-آليا-�-PvP-EU" {
		t.Fatal("valid UTF-8 name changed")
	}
	nameCounts(t, r, 0, 0)
	f[1], f[5] = fixture.Text(src), fixture.Text(src)
	_, r = decode(line(t, "SPELL_CAST_START", f...))
	nameCounts(t, r, 4, 2)
}

func TestNameCountersCoverKnownActorLayouts(t *testing.T) {
	for _, name := range []string{"SPELL_CAST_START", "SPELL_CREATE", "ENCHANT_APPLIED", "ENCHANT_REMOVED"} {
		f := append(common(), spell()...)
		f[1] = fixture.Text("Alia-\x00\xff-Stone-PvP-EU")
		events, r := decode(line(t, name, f...))
		if len(events) != 1 {
			t.Fatalf("%s not exercised", name)
		}
		nameCounts(t, r, 1, 1)
		if events[0].Kind == model.Other && events[0].Raw[2] != "Alia-\x00\xff-Stone-PvP-EU" {
			t.Error("opaque name bytes changed")
		}
		// Readable actor names are still counted when flags invalidate the row.
		f[2] = "abc"
		_, r = decode(line(t, name, f...))
		if r.Skipped[logfmt.InvalidFlags] != 1 {
			t.Fatal("invalid-flags precondition")
		}
		nameCounts(t, r, 1, 1)
	}
	for _, text := range []string{
		line(t, "INVENTED_EVENT", fixture.Text("not-an-actor\x00\xff")),
		line(t, "ZONE_CHANGE", "555", fixture.Text("Practice Town\x00\xff"), "2"),
		line(t, "SPELL_CAST_START", fixture.Text("unsupported-width\x00\xff")),
	} {
		_, r := decode(text)
		nameCounts(t, r, 0, 0)
	}
}
