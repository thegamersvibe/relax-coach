package logfmt_test

import (
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
)

func TestInvalidFieldsReportNamesBounded(t *testing.T) {
	for _, tc := range []struct {
		label, name, want string
	}{
		{"long", strings.Repeat("A", 200000), "other"},
		{"over-limit", strings.Repeat("A", 65), "other"},
		{"garbage", "EV\x00\xffENT-1", "other"},
		{"lowercase", "event", "other"},
		{"at-limit", strings.Repeat("A", 64), strings.Repeat("A", 64)},
		{"known", "SPELL_DAMAGE", "SPELL_DAMAGE"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			for _, tail := range []fixture.Field{`"unterminated`, "[(1,2]"} {
				events, r := decode(line(t, tc.name, tail))
				balanced(t, r)
				if len(events) != 0 || r.First == nil || r.Skipped[logfmt.InvalidFields] != 1 || len(r.UnknownEvents) != 0 {
					t.Fatal("malformed fields did not reach the InvalidFields skip")
				}
				key := logfmt.SkippedLayout{Name: tc.want, Fields: 0, Reason: logfmt.InvalidFields}
				if len(r.SkippedLayouts) != 1 || r.SkippedLayouts[key] != 1 || len(r.EventCounts) != 1 || r.EventCounts[tc.want] != 1 {
					t.Error("InvalidFields report keys bypassed the bounded name")
				}
				if len(r.String()) > 2000 || (tc.want == "other" && strings.Contains(r.String(), tc.name)) {
					t.Error("unsafe name retained in rendered InvalidFields report")
				}
			}
		})
	}
}

func TestInvalidFieldsShareUnknownNameBudget(t *testing.T) {
	var b strings.Builder
	// Accepted names and split failures must use the same distinct-name budget.
	for i := 0; i < 64; i++ {
		b.WriteString(line(t, unknownName(i), "1"))
	}
	for i := 0; i < 96; i++ {
		b.WriteString(line(t, unknownName(i), `"unterminated`))
	}
	events, r := decode(b.String())
	balanced(t, r)
	if len(events) != 64 || r.Parsed != 64 || r.Skipped[logfmt.InvalidFields] != 96 || len(r.UnknownEvents) != 64 {
		t.Fatal("did not exercise accepted names followed by split failures")
	}
	other := logfmt.SkippedLayout{Name: "other", Fields: 0, Reason: logfmt.InvalidFields}
	if len(r.EventCounts) != 65 || r.EventCounts["other"] != 32 || len(r.SkippedLayouts) != 65 || r.SkippedLayouts[other] != 32 {
		t.Fatal("InvalidFields names bypassed the shared distinct-name cap")
	}
	for i := 0; i < 64; i++ {
		name := unknownName(i)
		key := logfmt.SkippedLayout{Name: name, Fields: 0, Reason: logfmt.InvalidFields}
		if r.EventCounts[name] != 2 || r.UnknownEvents[name] != 1 || r.SkippedLayouts[key] != 1 {
			t.Fatal("a previously admitted name was not reused after the cap")
		}
	}
}
