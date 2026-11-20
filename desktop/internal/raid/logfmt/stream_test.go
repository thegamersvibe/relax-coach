package logfmt_test

import (
	"errors"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestEveryByteCutAndExactOffsets(t *testing.T) {
	s := header(t, "1") + damage(t, "SWING_DAMAGE", true) + line(t, "COMBATANT_INFO", fixture.Combatant(combatant())...) + line(t, "INVENTED_EVENT", fixture.Text("Alia, Stone-PvP-EU")) + header(t, "0") + strings.ReplaceAll(damage(t, "ENVIRONMENTAL_DAMAGE", false), "\r\n", "\n")
	full, _ := decode(s)
	for cut := 0; cut <= len(s); cut++ {
		events, r := decode(s[:cut])
		balanced(t, r)
		lines := int64(strings.Count(s[:cut], "\n"))
		if cut > 0 && s[cut-1] != '\n' {
			lines++
		}
		if r.LinesRead != lines {
			t.Fatalf("cut=%d lines=%d want=%d", cut, r.LinesRead, lines)
		}
		want := 0
		for _, e := range full {
			if e.Offset+e.Len <= int64(cut) {
				want++
			}
		}
		if len(events) != want {
			t.Fatalf("cut=%d events=%d want=%d", cut, len(events), want)
		}
		for i, e := range events {
			if e.Offset != full[i].Offset || e.Len != full[i].Len || e.Line != full[i].Line || s[e.Offset+e.Len-1] != '\n' {
				t.Fatalf("cut=%d bad byte range", cut)
			}
		}
	}
	// Appending the suffix then taking a fresh snapshot recovers the last row.
	events, r := decode(s[:len(s)-1] + s[len(s)-1:])
	balanced(t, r)
	if len(events) != len(full) {
		t.Fatal(r)
	}
}

type countedReader struct {
	r     io.Reader
	bytes int
}

func (c *countedReader) Read(p []byte) (int, error) { n, e := c.r.Read(p); c.bytes += n; return n, e }

type errorReader struct{ data string }

func (r *errorReader) Read(p []byte) (int, error) {
	if r.data == "" {
		return 0, io.ErrUnexpectedEOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, io.ErrUnexpectedEOF
}
func TestLazyStopNilReportAndReaderFailure(t *testing.T) {
	row := damage(t, "SPELL_DAMAGE", true)
	cr := &countedReader{r: strings.NewReader(strings.Repeat(row, 1000))}
	var report logfmt.ParseReport
	seq := logfmt.Decode(cr, &report)
	if cr.bytes != 0 {
		t.Fatal("eager read")
	}
	for range seq {
		break
	}
	balanced(t, report)
	if report.LinesRead != 1 || cr.bytes >= len(row)*1000 {
		t.Fatal("read past cancellation", report, cr.bytes)
	}
	for range logfmt.Decode(strings.NewReader(row), nil) {
	}
	for range logfmt.Decode(&errorReader{data: row + "partial"}, &report) {
	}
	balanced(t, report)
	if !errors.Is(report.ReadError, io.ErrUnexpectedEOF) || report.LinesRead != 2 || report.Skipped[logfmt.IncompleteLine] != 1 {
		t.Fatal(report)
	}
}

func TestOversizedLineResumesWithoutLosingOffsets(t *testing.T) {
	const capBytes = 1 << 20
	huge := line(t, "INVENTED_EVENT", fixture.Text(strings.Repeat("x", capBytes)))
	next := damage(t, "SWING_DAMAGE_LANDED", true)
	events, r := decode(huge + next)
	balanced(t, r)
	if len(events) != 1 || r.Skipped[logfmt.LineTooLong] != 1 || events[0].Offset != int64(len(huge)) || events[0].Len != int64(len(next)) || events[0].Line != 2 {
		t.Fatal("oversized line not bounded", len(events), r)
	}
}

func TestTypedEventsDoNotRetainWholeLines(t *testing.T) {
	f := append(common(), spell()...)
	f = append(f, advanced(alia.ID)...)
	f = append(f, nums(87, 99, -1, 4, 0, 0, 0, 0, 0, 0)...)
	// The unused damage tag is intentionally large; no typed field needs it.
	f = append(f, fixture.Field(strings.Repeat("x", 256*1024)))
	row := line(t, "SPELL_DAMAGE", f...)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var held []model.Event
	var report logfmt.ParseReport
	for e := range logfmt.Decode(&repeatedReader{row: row, left: int64(len(row)) * 32}, &report) {
		held = append(held, e)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	if after.HeapAlloc > before.HeapAlloc+4*1024*1024 {
		t.Errorf("typed events retained line backing storage: heap before=%d after=%d", before.HeapAlloc, after.HeapAlloc)
	}
	if len(held) != 32 {
		t.Fatal(report)
	}
	runtime.KeepAlive(held)
}

type repeatedReader struct {
	row       string
	left, pos int64
}

func (r *repeatedReader) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.left {
		p = p[:r.left]
	}
	total := 0
	for total < len(p) {
		n := copy(p[total:], r.row[r.pos:])
		total += n
		r.pos = (r.pos + int64(n)) % int64(len(r.row))
	}
	r.left -= int64(total)
	return total, nil
}
func TestGenerated200MiBFileFixedMemory(t *testing.T) {
	row := damage(t, "SPELL_DAMAGE", true)
	rows := int64((200*1024*1024 + len(row) - 1) / len(row))
	size := rows * int64(len(row))
	f, err := os.CreateTemp(t.TempDir(), "synthetic-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if n, err := io.Copy(f, &repeatedReader{row: row, left: size}); err != nil || n != size {
		t.Fatal(n, err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	old := debug.SetMemoryLimit(32 * 1024 * 1024)
	defer debug.SetMemoryLimit(old)
	runtime.GC()
	var m runtime.MemStats
	var peak uint64
	var count int64
	var r logfmt.ParseReport
	for e := range logfmt.Decode(f, &r) {
		count++
		if e.Kind != model.Damage || len(e.Raw) != 0 {
			t.Fatal("bad generated event")
		}
		if count%8192 == 0 {
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > peak {
				peak = m.HeapAlloc
			}
			if m.HeapAlloc > 64*1024*1024 {
				t.Fatalf("heap exceeded fixed 64 MiB ceiling: %d", m.HeapAlloc)
			}
		}
	}
	balanced(t, r)
	if count != rows || r.Parsed != rows || r.ReadError != nil || len(r.Skipped) != 0 {
		t.Fatal(count, r)
	}
	t.Logf("generated bytes=%d rows=%d peakHeapBytes=%d ceilingBytes=%d", size, rows, peak, 64*1024*1024)
}
