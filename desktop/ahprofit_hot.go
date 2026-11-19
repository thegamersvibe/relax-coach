package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Hot items on the Farming page (AH hot items plan, H1): items whose price is
// up since the previous recorded day AND against their usual price, from AH
// Profit's daily history of your own scans (AHProfitDB.hist, saved-data
// version 2 and later). Read-only; nothing is written anywhere.

// The thresholds, in one place (plan section 2). Starting values, to be tuned
// once on a real save.
const (
	hotMaxRows           = 10  // rows shown per auction house; never padded
	hotMinDays           = 5   // recorded days before Now for a baseline
	hotMinBaseline       = 500 // copper: 5 silver
	hotMinListed         = 5   // listed on Now's day, and median listed on the earlier days
	hotMaxAgainst        = 10  // Now at most this many times the baseline (and the previous day)
	hotMinChangePct      = 20  // up at least this % since the previous recorded day
	hotMinAgainstPct     = 20  // and at least this % against the usual price (baseline mode)
	hotSpikePct          = 50  // spike: Now at least this % above the baseline and above every earlier high
	hotPrevMaxGap        = 2   // Previous is at most this many days before Now
	hotChartDays         = 21  // the addon keeps 21 days
	hotChartW, hotChartH = 120, 28
)

// The addon's history format (AHProfit Core.lua, "daily history of your own
// auction house scans"): per item one string; per recorded day four numbers
// (day step, h, l, a) as base-32 digits of a 64-character alphabet, a digit of
// 32 or more meaning "more digits follow". The first step is the day itself
// (whole days since 2020-01-01 UTC), later ones the gap to the previous day.
const (
	ahpHistDay0  = 1577836800
	ahpHistDays  = 21
	ahpHistLen   = 1024
	ahpHistAlpha = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz+/"
)

var ahpHistDigit = func() (d [256]int8) {
	for i := range d {
		d[i] = -1
	}
	for i := 0; i < len(ahpHistAlpha); i++ {
		d[ahpHistAlpha[i]] = int8(i)
	}
	return
}()

// ahpScanDay is the addon's scanDay: whole UTC days since 2020-01-01.
func ahpScanDay(t time.Time) int {
	s := t.Unix() - ahpHistDay0
	d := s / 86400
	if s < 0 && s%86400 != 0 {
		d--
	}
	return int(d)
}

type ahpHistRow struct {
	Day, High, Low, Listed int // Low 0 when it equals High
}

// price is the day's one price: its lowest (plan 2: l, or h when they are equal).
func (r ahpHistRow) price() int {
	if r.Low > 0 {
		return r.Low
	}
	return r.High
}

func ahpHistTake(s string, i int) (int, int, bool) {
	v, mult := 0, 1
	for {
		if i >= len(s) || mult > 1099511627776 {
			return 0, i, false
		}
		c := ahpHistDigit[s[i]]
		if c < 0 {
			return 0, i, false
		}
		i++
		if c >= 32 {
			v += int(c-32) * mult
			mult *= 32
		} else {
			return v + int(c)*mult, i, true
		}
	}
}

// ahpHistDecode mirrors the addon's histDecode with its own checks: days
// increasing and not after today, prices 1..1e12, a low below its high (or 0),
// counts up to 1e9. Days older than 21 are dropped. ok=false when anything is
// wrong: the caller then drops that item, never the page.
func ahpHistDecode(s string, today int) ([]ahpHistRow, bool) {
	if len(s) == 0 || len(s) > ahpHistLen {
		return nil, false
	}
	var out []ahpHistRow
	i, day := 0, 0
	for i < len(s) {
		var step, h, l, a int
		var ok bool
		if step, i, ok = ahpHistTake(s, i); !ok || (day > 0 && step < 1) {
			return nil, false
		}
		if h, i, ok = ahpHistTake(s, i); !ok || h < 1 || h > 1e12 {
			return nil, false
		}
		if l, i, ok = ahpHistTake(s, i); !ok || l >= h {
			return nil, false
		}
		if a, i, ok = ahpHistTake(s, i); !ok || a > 1e9 {
			return nil, false
		}
		day += step
		if day > today || day > 1e5 {
			return nil, false
		}
		if day > today-ahpHistDays {
			out = append(out, ahpHistRow{day, h, l, a})
		}
	}
	return out, true
}

// ahpHistEncode is the addon's histEncode (tests and tools only).
func ahpHistEncode(rows []ahpHistRow) string {
	var b strings.Builder
	put := func(v int) {
		for v >= 32 {
			d := v % 32
			b.WriteByte(ahpHistAlpha[d+32])
			v = (v - d) / 32
		}
		b.WriteByte(ahpHistAlpha[v])
	}
	prev := 0
	for _, r := range rows {
		put(r.Day - prev)
		put(r.High)
		put(r.Low)
		put(r.Listed)
		prev = r.Day
	}
	return b.String()
}

type FarmHot struct {
	Shown     bool   // the section is on the page
	Message   string // the whole section: e.g. "Update AH Profit to see hot items."
	GuildOnly bool   // guild prices for a house with no own scan: shown on the unknown-house choice
	Houses    []FarmHotHouse
}

type FarmHotHouse struct {
	House      string
	Source     string    // "From your own scans"
	LastScan   time.Time // newest own scan of this house
	Message    string    // instead of rows: no scan today, guild only, nothing up
	Checked    int       // items with a Now and a Previous
	Dropped    int       // of those, left out by the filters
	Damaged    int       // history strings that did not decode (items dropped)
	ChangeOnly bool      // first weeks: no item has hotMinDays earlier days yet
	NowDay     time.Time // the recorded day the list is for (UTC)
	DaysAgo    int       // 0: today; 1: yesterday's scan; more: an older scan
	Rows       []FarmHotRow
}

type FarmHotRow struct {
	ID, Now, Prev, Baseline int
	Name                    string
	Gatherable              bool
	ChangePct, AgainstPct   int
	PrevDays                int // 1 "since yesterday", 2 "since 2 days ago"
	BaseDays                int // earlier days in the median
	Spike                   bool
	SpikeDays               int // "highest in N days"
	Chart                   FarmHotChart
	rank                    float64
}

type FarmHotChart struct {
	W, H   int
	Path   string // SVG path; a missing day breaks the line (a new M), never a zero
	DotX   int    // today's point, highlighted
	DotY   int
	HasDot bool
}

func median(v []int) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]int(nil), v...)
	sort.Ints(s)
	n := len(s)
	if n%2 == 1 {
		return float64(s[n/2])
	}
	return (float64(s[n/2-1]) + float64(s[n/2])) / 2
}

func pct(now, base float64) float64 { return (now - base) / base * 100 }

// hotItem is one item's history and its Now, before the house-wide mode is known.
type hotItem struct {
	id          int
	rows        []ahpHistRow
	now, listed int
	nowDay      int
	earlier     []ahpHistRow // rows before nowDay, oldest first
	prev        ahpHistRow
}

// buildHot ranks one save. ver is AHProfitDB.v; gather marks items the Farming
// page lists as gatherable (AH Profit's bundled farm data).
func buildHot(db map[string]any, ver int, verOK bool, houses []FarmHouse, items map[int]farmItemRef, gather map[int]bool, now time.Time) FarmHot {
	if !verOK || ver < 2 {
		return FarmHot{Shown: true, Message: "Update AH Profit to see hot items."}
	}
	if ver > ahpKnownVersion {
		return FarmHot{Shown: true, Message: "Hot items from this newer AH Profit need a newer Relax Coach."}
	}
	hist, live, shared := M(db["hist"]), M(db["live"]), M(db["shared"])
	today := ahpScanDay(now)
	out := FarmHot{Shown: true}
	known := map[string]bool{}
	for _, h := range houses {
		known[h.Key] = true
	}
	// Guild prices for a house this account never scanned itself: that member gets the guild-only line,
	// not "pick an auction house" (the picker lists only scanned houses).
	for k, raw := range shared {
		if !known[k] && len(M(M(raw)["p"])) > 0 {
			out.GuildOnly = true
		}
	}
	for _, h := range houses {
		out.Houses = append(out.Houses, buildHotHouse(h.Key, M(hist[h.Key]), M(live[h.Key]), M(shared[h.Key]), items, gather, today, now))
	}
	return out
}

func buildHotHouse(key string, H, L, S map[string]any, items map[int]farmItemRef, gather map[int]bool, today int, now time.Time) FarmHotHouse {
	hh := FarmHotHouse{House: key, Source: "From your own scans"}
	lt := M(L["t"])
	hh.LastScan = ahpTime(L["full"])
	for _, v := range lt {
		if t := ahpTime(v); t.After(hh.LastScan) && !t.After(now) {
			hh.LastScan = t
		}
	}
	if len(H) == 0 {
		if len(M(S["p"])) > 0 {
			hh.Message = "Hot items need your own scans for now. Guild price history is coming."
		} else {
			hh.Message = "No scan today. Scan the auction house to see what moved."
		}
		return hh
	}
	// Decide the day for the whole house, not per item: the newest recorded day. Without a scan today the
	// list is that day's against its previous day, for every item alike, labelled with the day, until a
	// new scan. (A live price never stands in for a day: items would drop one by one as they pass 24 h.)
	type decoded struct {
		id   int
		rows []ahpHistRow
	}
	var all []decoded
	newest := -1
	for k, raw := range H {
		id, err := strconv.Atoi(k)
		s, isStr := raw.(string)
		if err != nil || id < 1 || id > 1e7 || strconv.Itoa(id) != k || !isStr {
			hh.Damaged++
			continue
		}
		rows, ok := ahpHistDecode(s, today)
		if !ok {
			hh.Damaged++
			continue
		}
		if n := len(rows); n > 0 {
			newest = max(newest, rows[n-1].Day)
			all = append(all, decoded{id, rows})
		}
	}
	if newest < 0 {
		hh.Message = "No scan today. Scan the auction house to see what moved."
		return hh
	}
	hh.NowDay = time.Unix(ahpHistDay0+int64(newest)*86400, 0).UTC()
	hh.DaysAgo = today - newest
	var cands []hotItem
	anyBaseline := false
	for _, d := range all {
		last := d.rows[len(d.rows)-1]
		if last.Day != newest {
			continue // not seen on that day
		}
		it := hotItem{id: d.id, rows: d.rows, now: last.price(), listed: last.Listed, nowDay: newest}
		it.earlier = d.rows[:len(d.rows)-1]
		e := it.earlier
		if len(e) == 0 || it.nowDay-e[len(e)-1].Day > hotPrevMaxGap {
			continue // no Previous within 2 days: skipped
		}
		it.prev = e[len(e)-1]
		if len(e) >= hotMinDays {
			anyBaseline = true
		}
		cands = append(cands, it)
	}
	hh.ChangeOnly = !anyBaseline
	for _, it := range cands {
		hh.Checked++
		row, keep := hotRank(it, hh.ChangeOnly)
		if !keep {
			hh.Dropped++
			continue
		}
		row.ID = it.id
		row.Name = items[it.id].Name
		if row.Name == "" {
			row.Name = fmt.Sprintf("item %d", it.id)
		}
		row.Gatherable = gather[it.id]
		row.Chart = hotChart(it.rows, it.nowDay, it.now)
		hh.Rows = append(hh.Rows, row)
	}
	sort.Slice(hh.Rows, func(i, j int) bool {
		if hh.Rows[i].rank != hh.Rows[j].rank {
			return hh.Rows[i].rank > hh.Rows[j].rank
		}
		return hh.Rows[i].ID < hh.Rows[j].ID
	})
	if len(hh.Rows) > hotMaxRows {
		hh.Rows = hh.Rows[:hotMaxRows]
	}
	return hh
}

// hotRank applies the filters and the rank value (plan 2) to one item.
func hotRank(it hotItem, changeOnly bool) (FarmHotRow, bool) {
	var prices, listed []int
	high := 0
	for _, r := range it.earlier {
		prices = append(prices, r.price())
		listed = append(listed, r.Listed)
		high = max(high, r.High)
	}
	base := median(prices)
	now, prev := float64(it.now), float64(it.prev.price())
	change := pct(now, prev)
	against := pct(now, base)
	row := FarmHotRow{Now: it.now, Prev: it.prev.price(), Baseline: int(base + 0.5), PrevDays: it.nowDay - it.prev.Day, BaseDays: len(prices)}
	row.ChangePct, row.AgainstPct = int(change+0.5), int(against+0.5)
	switch {
	case !changeOnly && len(prices) < hotMinDays:
		return row, false
	case base < hotMinBaseline:
		return row, false
	case it.listed < hotMinListed || median(listed) < hotMinListed:
		return row, false
	case now > hotMaxAgainst*base || now > hotMaxAgainst*prev:
		return row, false
	case change < hotMinChangePct:
		return row, false
	}
	if changeOnly {
		row.rank = change
		return row, true
	}
	if against < hotMinAgainstPct { // up against both: a rebound to normal and a slow climb drop out
		return row, false
	}
	row.rank = min(change, against)
	if now >= base*(1+hotSpikePct/100.0) && it.now > high {
		row.Spike, row.SpikeDays = true, it.nowDay-it.earlier[0].Day+1
	}
	return row, true
}

// hotChart: one point per recorded day of the last 21, Now's day last and
// highlighted. A missing day is a gap in the line, not a zero.
func hotChart(rows []ahpHistRow, nowDay, now int) FarmHotChart {
	c := FarmHotChart{W: hotChartW, H: hotChartH}
	first := nowDay - hotChartDays + 1
	price := map[int]int{}
	for _, r := range rows {
		if r.Day >= first && r.Day <= nowDay {
			price[r.Day] = r.price()
		}
	}
	price[nowDay] = now
	lo, hi := now, now
	for _, p := range price {
		lo, hi = min(lo, p), max(hi, p)
	}
	x := func(d int) int { return 2 + (d-first)*(hotChartW-4)/(hotChartDays-1) }
	y := func(p int) int {
		if hi == lo {
			return hotChartH / 2
		}
		return hotChartH - 2 - (p-lo)*(hotChartH-4)/(hi-lo)
	}
	var b strings.Builder
	drawing := false
	for d := first; d <= nowDay; d++ {
		p, ok := price[d]
		if !ok {
			drawing = false
			continue
		}
		cmd := "L"
		if !drawing {
			cmd = "M"
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%s%d %d", cmd, x(d), y(p))
		drawing = true
	}
	c.Path = b.String()
	c.DotX, c.DotY, c.HasDot = x(nowDay), y(now), true
	return c
}
