package prepread

import (
	"math"
	"sort"
)

type storedBody struct {
	body, key string
	day       int64
	index     int
	receipts  []int64
}

func integer(v any, low, high int64) (int64, bool) {
	f, ok := v.(float64)
	if !ok || f < float64(low) || f > float64(high) || f != math.Trunc(f) {
		return 0, false
	}
	return int64(f), true
}

func array(v any, cap int) ([]any, bool) {
	t, ok := v.(table)
	if !ok || len(t) > cap {
		return nil, false
	}
	out := make([]any, len(t))
	for i := range out {
		item, exists := t[float64(i+1)]
		if !exists || item == nil {
			return nil, false
		}
		out[i] = item
	}
	return out, true
}

func keys(t table) ([]string, bool) {
	out := make([]string, 0, len(t))
	for key := range t {
		s, ok := key.(string)
		if !ok || nameKey(s) != s || !stringsQualified(s) {
			return nil, false
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out, true
}

func stringsQualified(s string) bool {
	for i := range len(s) {
		if s[i] == '-' {
			return true
		}
	}
	return false
}

// scanStore refuses structural corruption rather than performing the addon's
// destructive load-time salvage. It checks all retained nights, even when the
// requested fight interval excludes them. Bodies are decoded after references
// are proven valid and all byte budgets have been accounted exactly once.
func scanStore(root table) ([]storedBody, string) {
	// Store.lua's newer latch precedes validation, including fractional and
	// very large numeric future versions. No future layout is interpreted.
	if v, ok := root["version"].(float64); ok && v > 1 {
		return nil, "newer store"
	}
	prep, prepOK := root["prep"].(table)
	if prepOK {
		if v, ok := prep["version"].(float64); ok && v > 2 {
			return nil, "newer store"
		}
	}
	rootVersion, ok := integer(root["version"], 1, 1)
	if !ok || rootVersion != 1 {
		return nil, "account version"
	}
	if !prepOK {
		return nil, "store type"
	}
	version, ok := integer(prep["version"], 2, 2)
	if !ok || version != 2 {
		return nil, "store version"
	}
	nights, ok := array(prep["nights"], maxNights)
	if !ok {
		return nil, "nights shape or cap"
	}
	days := map[int64]bool{}
	var out []storedBody
	for _, value := range nights {
		n, ok := value.(table)
		if !ok {
			return nil, "night type"
		}
		day, ok := integer(n["day"], -1, limits.MaxDay)
		if !ok || days[day] {
			return nil, "night day"
		}
		days[day] = true
		windows, ok := array(n["windows"], maxWindows)
		if !ok || len(windows) == 0 {
			return nil, "windows shape or cap"
		}
		reports, ok := n["reports"].(table)
		if !ok || len(reports) > maxWindows*maxPlayers {
			return nil, "reports shape or cap"
		}
		names, ok := keys(reports)
		if !ok {
			return nil, "character key"
		}
		byName := map[string][]*storedBody{}
		total := 0
		for _, key := range names {
			bodies, ok := array(reports[key], maxWindows)
			if !ok || len(bodies) == 0 {
				return nil, "bodies shape or cap"
			}
			own := 0
			for i, value := range bodies {
				body, ok := value.(string)
				if !ok {
					return nil, "body type"
				}
				own += len(body)
				byName[key] = append(byName[key], &storedBody{body: body, key: key, day: day, index: i + 1})
			}
			if own > limits.MaxCharacterBytes {
				return nil, "character byte budget"
			}
			total += own
		}
		if total > limits.MaxNightBytes {
			return nil, "night byte budget"
		}
		for _, value := range windows {
			w, ok := value.(table)
			if !ok {
				return nil, "window type"
			}
			if _, ok := integer(w["openedAt"], 0, limits.MaxStamp); !ok {
				return nil, "window clock"
			}
			rows, ok := w["reports"].(table)
			if !ok || len(rows) == 0 || len(rows) > maxPlayers {
				return nil, "characters shape or cap"
			}
			names, ok := keys(rows)
			if !ok {
				return nil, "character key"
			}
			for _, key := range names {
				row, ok := rows[key].(table)
				if !ok {
					return nil, "reference type"
				}
				stamp, ok := integer(row["receivedAt"], 0, limits.MaxStamp)
				if !ok {
					return nil, "reference clock"
				}
				index, ok := integer(row["report"], 1, int64(len(byName[key])))
				if !ok {
					return nil, "dangling reference"
				}
				b := byName[key][int(index)-1]
				b.receipts = append(b.receipts, stamp)
			}
		}
		for _, key := range names {
			for _, b := range byName[key] {
				if len(b.receipts) == 0 {
					return nil, "unreferenced body"
				}
				out = append(out, *b)
			}
		}
	}
	return out, ""
}
