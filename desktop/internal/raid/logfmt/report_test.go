package logfmt_test

import (
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func header(t *testing.T, enabled string) string {
	return line(t, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", fixture.Field(enabled), "BUILD_VERSION", fixture.Text("fixture.1"), "PROJECT_ID", "99")
}

func TestReportRepeatedHeadersAndFieldCountEvidence(t *testing.T) {
	// Repeated headers are format-reported, not observed in the two live logs.
	start := header(t, "0")
	good := damage(t, "SWING_DAMAGE_LANDED", true)
	unknown := line(t, "UNRECOGNISED_EVENT", "nil")
	bad := line(t, "SPELL_ABSORBED", "1")
	text := start + good + header(t, "1") + damage(t, "SWING_DAMAGE", false) + unknown + bad + "partial"
	events, r := decode(text)
	balanced(t, r)
	if len(events) != 5 || r.HeadersSeen != 2 || r.Parsed != 3 || r.LinesRead != 7 || r.AdvancedOn != 1 || r.AdvancedOff != 1 {
		t.Fatal(r)
	}
	if r.Header == nil || r.Header.Version != 22 || r.Header.Build != "fixture.1" || r.Header.ProjectID != model.Some(99) || r.Header.Advanced != model.Some(1) || r.Header.Line != 3 {
		t.Fatal(r.Header)
	}
	if r.First == nil || r.Last == nil || r.First.TimeMs != at.UnixMilli() || r.First.OffsetH != 2 || r.Last.TimeMs != at.UnixMilli() {
		t.Fatal(r)
	}
	if r.SkippedLayouts[logfmt.SkippedLayout{Name: "SPELL_ABSORBED", Fields: 2, Reason: logfmt.UnknownWidth}] != 1 {
		t.Fatal(r)
	}
	for _, e := range events {
		if e.Kind != model.Other && len(e.Raw) != 0 {
			t.Fatal(e.Kind, "raw retained")
		}
	}
	rendered := r.String()
	for _, want := range []string{"Lines: 7", "Parsed: 3", "Headers: 2", "fixture.1", "Advanced: mixed", "as this logger saw it", "SPELL_ABSORBED fields=2", "UNRECOGNISED_EVENT: 1", "First:", "Last:"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("missing %q in %s", want, rendered)
		}
	}
	if r.String() != rendered {
		t.Fatal("unstable report rendering")
	}
	for _, name := range []string{alia.Name, bram.Name, alia.ID, bram.ID} {
		if strings.Contains(rendered, name) {
			t.Fatal("report leaked actor")
		}
	}
	// During a boundary yield the caller can read that segment's latest header.
	var seen []int64
	var live logfmt.ParseReport
	for e := range logfmt.Decode(strings.NewReader(header(t, "0")+header(t, "1")), &live) {
		if e.Kind == model.Header {
			seen = append(seen, live.Header.Advanced.V)
		}
	}
	if len(seen) != 2 || seen[0] != 0 || seen[1] != 1 {
		t.Fatal(seen)
	}
}

func TestReportHasNoFalseAdvancedClaimWithoutEvidence(t *testing.T) {
	_, r := decode(header(t, "1"))
	if !strings.Contains(r.String(), "Advanced: unknown") {
		t.Fatal(r.String())
	}
	_, r = decode(damage(t, "SPELL_DAMAGE", false))
	if !strings.Contains(r.String(), "Advanced: off") {
		t.Fatal(r.String())
	}
	_, r = decode(damage(t, "SPELL_DAMAGE", true))
	if !strings.Contains(r.String(), "Advanced: on") {
		t.Fatal(r.String())
	}
}
