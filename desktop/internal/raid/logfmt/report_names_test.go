package logfmt_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
)

func TestReportRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{"lowercase", "EVENT_1", "EVENT-NAME", " EVENT", "EV\tENT", "EV\x00ENT", "EV\xffENT", "ÉVENT", strings.Repeat("A", 65), strings.Repeat("A", 200000)} {
		for _, badTime := range []bool{false, true} {
			row := line(t, name, "1")
			if badTime {
				row = strings.Replace(row, ".1232  ", ".123+2  ", 1)
			}
			_, r := decode(row)
			balanced(t, r)
			if len(r.EventCounts) != 1 || r.EventCounts["other"] != 1 {
				t.Errorf("unbounded/unsafe event key: input bytes=%d badTime=%v", len(name), badTime)
			}
			if badTime {
				if len(r.UnknownEvents) != 0 || len(r.SkippedLayouts) != 1 || r.SkippedLayouts[logfmt.SkippedLayout{Name: "other", Reason: logfmt.InvalidTime}] != 1 {
					t.Error("skipped name not bucketed")
				}
			} else if len(r.UnknownEvents) != 1 || r.UnknownEvents["other"] != 1 {
				t.Error("unknown name not bucketed")
			}
			if len(r.String()) > 2000 {
				t.Error("unsafe name retained in rendered report")
			}
		}
	}
	// A permitted name at the length boundary is retained, not folded away.
	name := strings.Repeat("Z", 64)
	_, r := decode(line(t, name, "1"))
	if r.UnknownEvents[name] != 1 || r.EventCounts[name] != 1 {
		t.Fatal("64-byte name was not retained")
	}
}

func unknownName(i int) string {
	return fmt.Sprintf("INVENTED_%c%c", 'A'+rune(i/26), 'A'+rune(i%26))
}

func TestReportUnknownNameBudget(t *testing.T) {
	for _, badTime := range []bool{false, true} {
		var b strings.Builder
		for i := 0; i < 96; i++ {
			b.WriteString(line(t, unknownName(i), "1"))
		}
		b.WriteString(line(t, unknownName(0), "2"))
		text := b.String()
		if badTime {
			text = strings.ReplaceAll(text, ".1232  ", ".123+2  ")
		}
		// Known names are outside the unknown-name budget.
		text += damage(t, "SPELL_DAMAGE", false)
		_, r := decode(text)
		balanced(t, r)
		if len(r.EventCounts) != 66 || r.EventCounts["other"] != 32 || r.EventCounts[unknownName(0)] != 2 || r.EventCounts[unknownName(63)] != 1 || r.EventCounts[unknownName(64)] != 0 || r.EventCounts["SPELL_DAMAGE"] != 1 {
			t.Errorf("badTime=%v event name budget: names=%d other=%d first=%d", badTime, len(r.EventCounts), r.EventCounts["other"], r.EventCounts[unknownName(0)])
		}
		if badTime {
			if len(r.UnknownEvents) != 0 || len(r.SkippedLayouts) != 65 || r.SkippedLayouts[logfmt.SkippedLayout{Name: "other", Reason: logfmt.InvalidTime}] != 32 {
				t.Error("skipped names bypassed shared budget")
			}
		} else if len(r.UnknownEvents) != 65 || r.UnknownEvents["other"] != 32 || r.UnknownEvents[unknownName(0)] != 2 {
			t.Error("unknown names bypassed budget")
		}
	}
}

func TestReportNamesDoNotRetainLines(t *testing.T) {
	for _, badTime := range []bool{false, true} {
		var b strings.Builder
		for i := 0; i < 64; i++ {
			b.WriteString(line(t, unknownName(i), fixture.Text(strings.Repeat("x", 128*1024))))
		}
		input := b.String()
		if badTime {
			input = strings.ReplaceAll(input, ".1232  ", ".123+2  ")
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		var r logfmt.ParseReport
		for range logfmt.Decode(strings.NewReader(input), &r) {
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		if after.HeapAlloc > before.HeapAlloc+4*1024*1024 {
			t.Errorf("report retained line storage: badTime=%v before=%d after=%d", badTime, before.HeapAlloc, after.HeapAlloc)
		}
		if len(r.EventCounts) != 64 {
			t.Error("memory test did not exercise distinct name retention")
		}
		runtime.KeepAlive(input)
		runtime.KeepAlive(r)
	}
}
