package logfmt

import (
	"bufio"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"io"
	"iter"
	"strconv"
	"strings"
	"time"
)

// Decode is lazy. Each yielded event owns its strings. Stopping iteration stops
// reads (apart from bufio's bounded read-ahead); the reader is never closed.
// A final unterminated line is skipped, even if its tokens look complete: the
// file may still be growing. Decode a fresh snapshot to include it once written.
func Decode(r io.Reader, report *ParseReport) iter.Seq[model.Event] {
	return func(yield func(model.Event) bool) {
		if report == nil {
			report = &ParseReport{}
		}
		*report = ParseReport{Skipped: map[SkipReason]int64{}, SkippedLayouts: map[SkippedLayout]int64{}, EventCounts: map[string]int64{}, UnknownEvents: map[string]int64{}}
		br := bufio.NewReader(r)
		var offset int64
		for {
			raw, n, err := readLine(br)
			if n > 0 {
				report.LinesRead++
				if raw == "" {
					report.skip(LineTooLong, "", 0)
				} else {
					e, ok := decodeLine(raw, offset, report)
					if ok && !yield(e) {
						return
					}
				}
				offset += n
			}
			if err != nil {
				if err != io.EOF {
					report.ReadError = err
				}
				return
			}
		}
	}
}

func decodeLine(raw string, offset int64, r *ParseReport) (model.Event, bool) {
	if raw[len(raw)-1] != '\n' {
		r.skip(IncompleteLine, "", 0)
		return model.Event{}, false
	}
	text := strings.TrimSuffix(strings.TrimSuffix(raw, "\n"), "\r")
	stamp, body, found := strings.Cut(text, "  ")
	name, _, _ := strings.Cut(body, ",")
	if name != "" {
		r.EventCounts[name]++
	}
	tm, zone, ok := timestamp(stamp)
	if !found || !ok {
		r.skip(InvalidTime, name, 0)
		return model.Event{}, false
	}
	timePoint := &TimeStamp{tm, zone}
	if r.First == nil {
		r.First = timePoint
	}
	r.Last = timePoint
	fields, ok := split(body)
	if !ok || name == "" {
		r.skip(InvalidFields, name, 0)
		return model.Event{}, false
	}
	e, reason, known, multi := eventPower(fields)
	if reason != "" {
		r.skip(reason, name, len(fields))
		return model.Event{}, false
	}
	if !known {
		r.UnknownEvents[name]++
	}
	if multi {
		r.MultiPower++ // two power types in one field: the first is kept
	}
	if e.Kind == model.Header {
		r.HeadersSeen++
		r.Header = &HeaderInfo{Version: number(fields[1]).V, Advanced: number(fields[3]), Build: strings.Clone(fields[5]), ProjectID: number(fields[7]), Line: r.LinesRead}
	} else {
		r.Parsed++
	}
	if schema, ok := formats[name]; ok {
		for _, present := range schema.widths {
			if present {
				if schema.widths[len(fields)] {
					r.AdvancedOn++
				} else {
					r.AdvancedOff++
				}
				break
			}
		}
	}
	e.TimeMs, e.OffsetH, e.Line, e.Offset, e.Len = tm, zone, r.LinesRead, offset, int64(len(raw))
	return e, true
}

func timestamp(s string) (int64, int8, bool) {
	dot := strings.LastIndexByte(s, '.')
	if dot < 0 || len(s) < dot+5 {
		return 0, 0, false
	}
	fraction := s[dot+1 : dot+4]
	for _, c := range fraction {
		if c < '0' || c > '9' {
			return 0, 0, false
		}
	}
	suffix := s[dot+4:]
	if suffix[0] == '+' {
		return 0, 0, false
	}
	offset, err := strconv.ParseInt(suffix, 10, 8)
	if err != nil || offset < -23 || offset > 23 {
		return 0, 0, false
	}
	at, err := time.Parse("1/2/2006 15:04:05.000", s[:dot+4])
	if err != nil {
		return 0, 0, false
	}
	return at.UnixMilli(), int8(offset), true
}
