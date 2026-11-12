package main

// Farming page, Gold and Sessions: AH Profit's money ledger and session
// archive (saved-data version 3). Read only when AHProfitDB.v is 3; the
// same parsed save and caps as the rest of the page.

import (
	"sort"
	"strconv"
	"time"
)

// ahpLedgerDays and the caps match AH Profit's own (Tools.lua): 180 days, 50
// characters, 100 sessions. Anything past them is not shown.
const (
	ahpLedgerDays  = 180
	ahpLedgerChars = 50
	ahpSessions    = 100
	ahpDay0        = 1577836800 // AH Profit's day 0: 2020-01-01 UTC
)

// ahpSources are AH Profit's ledger sources and how the page names them.
var ahpSources = map[string]string{
	"vendor": "Vendor", "repair": "Repairs", "ah": "Auction house", "mail": "Mail", "trade": "Trade",
	"trainer": "Trainer", "taxi": "Flights", "quest": "Quests", "loot": "Loot", "other": "Other",
}

// FarmGold: earned and spent for today, 7 days, 30 days and everything kept
// (180 days), and the last 30 days by source and by character. Days are AH
// Profit's: UTC days.
type FarmGold struct {
	Periods []FarmGoldPeriod
	Sources []FarmGoldRow // last 30 days
	Chars   []FarmGoldRow // last 30 days, at most ahpLedgerChars
}

type FarmGoldPeriod struct {
	Label   string
	In, Out int
}

func (p FarmGoldPeriod) Net() int { return p.In - p.Out }

type FarmGoldRow struct {
	Name    string
	In, Out int
}

func (r FarmGoldRow) Net() int { return r.In - r.Out }

type FarmSession struct {
	Char, Zone string
	Start      time.Time
	Dur        int // seconds
	Net        int // copper in − out
	PerHour    int // copper per hour; only when HasRate
	HasRate    bool
	Nodes      int
}

// buildGold reads DB.ledger. ok is false when the save has none usable.
func buildGold(db map[string]any, now time.Time) (*FarmGold, bool) {
	ledger, ok := db["ledger"].(map[string]any)
	if !ok {
		return nil, false
	}
	today := int((now.Unix() - ahpDay0) / 86400)
	g := &FarmGold{Periods: []FarmGoldPeriod{{Label: "Today"}, {Label: "7 days"}, {Label: "30 days"}, {Label: "180 days"}}}
	spans := []int{1, 7, 30, ahpLedgerDays}
	bySrc := map[string]*FarmGoldRow{}
	var chars []FarmGoldRow
	for char, rawDays := range ledger {
		days := M(rawDays)
		if char == "" || len(char) > 80 || days == nil {
			continue
		}
		row := FarmGoldRow{Name: char}
		for k, rawSrcs := range days {
			day, err := strconv.Atoi(k)
			if err != nil || day > today || day <= today-ahpLedgerDays {
				continue // a day after today (clock) or past what is kept
			}
			for src, rawE := range M(rawSrcs) {
				name, known := ahpSources[src]
				e := List(rawE)
				if !known || len(e) < 2 {
					continue
				}
				in, ok1 := ahpInt(e[0], 1e12)
				out, ok2 := ahpInt(e[1], 1e12)
				if !ok1 || !ok2 {
					continue
				}
				for i, span := range spans {
					if day > today-span {
						g.Periods[i].In = ahpAdd(g.Periods[i].In, in)
						g.Periods[i].Out = ahpAdd(g.Periods[i].Out, out)
					}
				}
				if day > today-30 {
					s := bySrc[name]
					if s == nil {
						s = &FarmGoldRow{Name: name}
						bySrc[name] = s
					}
					s.In, s.Out = ahpAdd(s.In, in), ahpAdd(s.Out, out)
					row.In, row.Out = ahpAdd(row.In, in), ahpAdd(row.Out, out)
				}
			}
		}
		if row.In > 0 || row.Out > 0 {
			chars = append(chars, row)
		}
	}
	for _, s := range bySrc {
		if s.In > 0 || s.Out > 0 {
			g.Sources = append(g.Sources, *s)
		}
	}
	moved := func(r FarmGoldRow) int { return ahpAdd(r.In, r.Out) }
	byMoved := func(list []FarmGoldRow) {
		sort.Slice(list, func(i, j int) bool {
			if a, b := moved(list[i]), moved(list[j]); a != b {
				return a > b
			}
			return list[i].Name < list[j].Name
		})
	}
	byMoved(g.Sources)
	byMoved(chars)
	if len(chars) > ahpLedgerChars {
		chars = chars[:ahpLedgerChars]
	}
	g.Chars = chars
	return g, true
}

// buildSessions reads DB.sessions, newest first, at most ahpSessions.
func buildSessions(db map[string]any, maps map[int]string) ([]FarmSession, bool) {
	raw, ok := db["sessions"].(map[string]any)
	if !ok {
		return nil, false
	}
	var out []FarmSession
	for _, r := range List(raw) {
		s := M(r)
		start, ok1 := ahpInt(s["start"], 1e10)
		_, ok2 := ahpInt(s["stop"], 1e10)
		dur, ok3 := ahpInt(s["dur"], 1e9)
		inc, okI := s["inc"].(map[string]any)
		exp, okO := s["out"].(map[string]any)
		// A stop before the start (the server clock stepped back) keeps the
		// session: its time comes from dur, which the addon measures itself.
		if !ok1 || !ok2 || !ok3 || !okI || !okO {
			continue
		}
		fs := FarmSession{Start: time.Unix(int64(start), 0), Dur: dur}
		// Name-Realm, as the Gold table shows characters.
		if c, isStr := s["char"].(string); isStr && len(c) <= 64 {
			fs.Char = c
			if r, isStr := s["realm"].(string); isStr && r != "" && len(r) <= 64 {
				fs.Char = c + "-" + r
			}
		}
		if z, isStr := s["zone"].(string); isStr && len(z) <= 80 {
			fs.Zone = z
		}
		if fs.Zone == "" {
			if m, okM := ahpInt(s["map"], 1e7); okM {
				fs.Zone = maps[m]
			}
		}
		fs.Nodes, _ = ahpInt(s["nodes"], 1e6)
		for _, side := range []struct {
			t    map[string]any
			sign int
		}{{inc, 1}, {exp, -1}} {
			for src, v := range side.t {
				if c, okC := ahpInt(v, 1e12); okC && ahpSources[src] != "" {
					fs.Net += side.sign * c
				}
			}
		}
		if dur >= 60 {
			fs.PerHour, fs.HasRate = int(int64(fs.Net)*3600/int64(dur)), true
		}
		out = append(out, fs)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.After(out[j].Start) })
	if len(out) > ahpSessions {
		out = out[:ahpSessions]
	}
	return out, true
}

// ahpDur formats seconds like AH Profit: "1h 5m", "30m".
func ahpDur(s int) string {
	h, m := s/3600, s/60%60
	if h > 0 {
		return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m"
	}
	return strconv.Itoa(m) + "m"
}
