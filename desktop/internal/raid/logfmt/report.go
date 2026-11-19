// Package logfmt streams Forever combat logs into the shared event model.
// It owns all line-format knowledge. It never opens files or sends data.
package logfmt

import (
	"fmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"sort"
	"strings"
	"unicode/utf8"
)

type SkipReason string

const (
	InvalidTime    SkipReason = "invalid timestamp"
	IncompleteLine SkipReason = "incomplete line"
	InvalidFields  SkipReason = "invalid fields"
	UnknownWidth   SkipReason = "unknown field count"
	InvalidValue   SkipReason = "invalid required value"
	InvalidFlags   SkipReason = "invalid actor flags"
	LineTooLong    SkipReason = "line too long"
)

type TimeStamp struct {
	TimeMs  int64
	OffsetH int8
}

// HeaderInfo is the latest accepted header. It is updated before the Header
// boundary is yielded; glue can pass this metadata to consumers without Raw.
type HeaderInfo struct {
	Version   int64
	Advanced  model.Opt // header claim, not field-count evidence
	Build     string
	ProjectID model.Opt
	Line      int64
}

// SkippedLayout contains no player data; Fields is zero when splitting failed.
type SkippedLayout struct {
	Name   string
	Fields int
	Reason SkipReason
}

// ParseReport is reset when iteration starts. Parsed includes Other, including
// unknown names; it excludes headers. LinesRead = Parsed + HeadersSeen + skips.
// EventCounts counts names even on skipped lines. Unknown names are bounded
// ASCII labels; unsafe/excess names share "other" across all report maps.
// First and Last are the first/last valid timestamps in file order, not
// sorted clock extrema.
// A reader failure is separate from line accounting and never silently ignored.
type ParseReport struct {
	LinesRead, Parsed, HeadersSeen int64
	Skipped                        map[SkipReason]int64
	SkippedLayouts                 map[SkippedLayout]int64
	EventCounts                    map[string]int64
	UnknownEvents                  map[string]int64
	Header                         *HeaderInfo
	First, Last                    *TimeStamp
	AdvancedOn, AdvancedOff        int64 // only layouts with an optional advanced block
	MultiPower                     int64 // lines with two power types in one field (first kept)
	NameNULBytes                   int64 // NUL bytes in recognized actor-name fields
	InvalidUTF8Names               int64 // actor-name occurrences, not unique names
	ReadError                      error
	unknownNames                   map[string]struct{} // shared budget, including skipped names
}

const (
	maxReportNameBytes = 64
	maxUnknownNames    = 64
	otherEventName     = "other"
)

// reportName bounds every report key without changing the event or its Raw.
// Even a short name must own its storage rather than pin the source line.
func (r *ParseReport) reportName(name string) string {
	if name == "" {
		return ""
	}
	if _, known := formats[name]; !known {
		if len(name) > maxReportNameBytes {
			return otherEventName
		}
		for _, c := range name {
			if c != '_' && (c < 'A' || c > 'Z') {
				return otherEventName
			}
		}
		if _, seen := r.unknownNames[name]; !seen {
			if len(r.unknownNames) >= maxUnknownNames {
				return otherEventName
			}
			if r.unknownNames == nil {
				r.unknownNames = make(map[string]struct{})
			}
			r.unknownNames[strings.Clone(name)] = struct{}{}
		}
	}
	return strings.Clone(name)
}

// countActorNames inspects known, split layouts before value validation, so
// invalid flags do not hide name diagnostics. It never normalizes name bytes.
func (r *ParseReport) countActorNames(f []string) {
	schema, known := formats[f[0]]
	if !known {
		return
	}
	if _, supported := schema.widths[len(f)]; !supported {
		return
	}
	switch schema.kind {
	case model.Header, model.EncounterStart, model.EncounterEnd, model.ZoneChange, model.CombatantInfo:
		return
	case model.Other:
		if f[0] == "MAP_CHANGE" {
			return
		}
	}
	indices := []int{2, 6}
	if schema.kind == model.Absorb {
		p := 10
		if len(f) == 22 {
			p += 3
		}
		indices = append(indices, p)
	}
	for _, i := range indices {
		r.NameNULBytes += int64(strings.Count(f[i], "\x00"))
		if !utf8.ValidString(f[i]) {
			r.InvalidUTF8Names++
		}
	}
}

func (r *ParseReport) skip(reason SkipReason, name string, fields int) {
	r.Skipped[reason]++
	if name != "" {
		r.SkippedLayouts[SkippedLayout{name, fields, reason}]++
	}
}

// String is a deterministic text report, not an event dump. A coverage gap
// cannot be inferred from clock silence: a nearby pull may simply be unseen.
func (r ParseReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Lines: %d\nParsed: %d\nHeaders: %d\n", r.LinesRead, r.Parsed, r.HeadersSeen)
	if r.Header != nil {
		fmt.Fprintf(&b, "Header: version=%d build=%s advanced=%v project=%v\n", r.Header.Version, r.Header.Build, r.Header.Advanced, r.Header.ProjectID)
	}
	state := "unknown"
	switch {
	case r.AdvancedOn > 0 && r.AdvancedOff > 0:
		state = "mixed"
	case r.AdvancedOn > 0:
		state = "on"
	case r.AdvancedOff > 0:
		state = "off"
	}
	fmt.Fprintf(&b, "Advanced: %s (on=%d off=%d, by field count)\n", state, r.AdvancedOn, r.AdvancedOff)
	fmt.Fprintf(&b, "Lines with two power types: %d\n", r.MultiPower)
	fmt.Fprintf(&b, "Actor name NUL bytes: %d\nActor names with invalid UTF-8: %d\n", r.NameNULBytes, r.InvalidUTF8Names)
	if r.First != nil {
		fmt.Fprintf(&b, "First: timeMs=%d offsetH=%d\n", r.First.TimeMs, r.First.OffsetH)
	}
	if r.Last != nil {
		fmt.Fprintf(&b, "Last: timeMs=%d offsetH=%d\n", r.Last.TimeMs, r.Last.OffsetH)
	}
	var rows []string
	for reason, n := range r.Skipped {
		rows = append(rows, fmt.Sprintf("Skipped %s: %d", reason, n))
	}
	for key, n := range r.SkippedLayouts {
		rows = append(rows, fmt.Sprintf("Skipped %s fields=%d (%s): %d", key.Name, key.Fields, key.Reason, n))
	}
	for name, n := range r.EventCounts {
		rows = append(rows, fmt.Sprintf("Event %s: %d", name, n))
	}
	for name, n := range r.UnknownEvents {
		rows = append(rows, fmt.Sprintf("Unknown %s: %d", name, n))
	}
	sort.Strings(rows)
	for _, row := range rows {
		b.WriteString(row)
		b.WriteByte('\n')
	}
	if r.ReadError != nil {
		b.WriteString("Reader error: yes\n")
	}
	b.WriteString("Trash is as this logger saw it; range blind spots cannot be inferred from gaps in time.\n")
	return b.String()
}
