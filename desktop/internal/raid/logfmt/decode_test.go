package logfmt_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

var at = time.Date(2026, 6, 7, 8, 9, 10, 123000000, time.UTC)

func line(t *testing.T, name string, f ...fixture.Field) string {
	t.Helper()
	var b bytes.Buffer
	if err := fixture.Write(&b, at, 2, name, f...); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
func decode(s string) ([]model.Event, logfmt.ParseReport) {
	var out []model.Event
	var r logfmt.ParseReport
	for e := range logfmt.Decode(strings.NewReader(s), &r) {
		out = append(out, e)
	}
	return out, r
}
func balanced(t *testing.T, r logfmt.ParseReport) {
	t.Helper()
	sum := r.Parsed + r.HeadersSeen
	for _, n := range r.Skipped {
		sum += n
	}
	if sum != r.LinesRead {
		t.Fatalf("accounting: %+v", r)
	}
}

func TestUnknownReportedEscapingAndTimeOffsets(t *testing.T) {
	// Comma/quote escaping: format reported, not observed in live logs.
	for _, offset := range []int8{2, -4, 10} {
		var b bytes.Buffer
		if err := fixture.Write(&b, at, offset, "INVENTED_EVENT", fixture.Text("Bram, \"Oak\"\\Hill-PvP-EU"), fixture.List(fixture.Tuple(fixture.Number(1), fixture.Number(2)))); err != nil {
			t.Fatal(err)
		}
		events, r := decode(b.String())
		balanced(t, r)
		if len(events) != 1 {
			t.Fatalf("events=%d report=%+v", len(events), r)
		}
		e := events[0]
		if e.Kind != model.Other || e.TimeMs != at.UnixMilli() || e.OffsetH != offset || e.Line != 1 || e.Offset != 0 || e.Len != int64(b.Len()) || len(e.Raw) != 3 || e.Raw[1] != "Bram, \"Oak\"\\Hill-PvP-EU" || e.Raw[2] != "[(1,2)]" {
			t.Fatalf("%+v", e)
		}
		if r.LinesRead != 1 || r.Parsed != 1 || r.EventCounts["INVENTED_EVENT"] != 1 || r.UnknownEvents["INVENTED_EVENT"] != 1 {
			t.Fatal(r)
		}
	}
}

func TestTruncatedAndInvalidTimestampAccounting(t *testing.T) {
	good := line(t, "INVENTED_EVENT", fixture.Number(3))
	bad := strings.Replace(good, ".1232  ", ".123  ", 1)
	events, r := decode(good + bad + strings.TrimSuffix(good, "\r\n"))
	balanced(t, r)
	if len(events) != 1 || r.LinesRead != 3 || r.Skipped[logfmt.InvalidTime] != 1 || r.Skipped[logfmt.IncompleteLine] != 1 {
		t.Fatal(r)
	}
}
