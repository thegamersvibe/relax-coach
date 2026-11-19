package main

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Hot items (AH hot items plan, H1). Synthetic data only; item ids are invented.

// hotNow is 12:00 UTC on scan day 2506.
var hotNow = time.Unix(ahpHistDay0+2506*86400+12*3600, 0)

const hotToday = 2506

// series builds an item's history ending on day `last`: prices oldest first,
// 0 = no row that day. The listed count is the same every day.
func series(last int, listed int, prices ...int) []ahpHistRow {
	var rows []ahpHistRow
	first := last - len(prices) + 1
	for i, p := range prices {
		if p > 0 {
			rows = append(rows, ahpHistRow{Day: first + i, High: p, Low: 0, Listed: listed})
		}
	}
	return rows
}

// hotSave is a version 3 save with one house (Realm|Alliance) whose history is hist.
func hotSave(hist map[int][]ahpHistRow) map[string]any {
	H := map[string]any{}
	for id, rows := range hist {
		H[strconv.Itoa(id)] = ahpHistEncode(rows)
	}
	return map[string]any{
		"v":    float64(3),
		"hist": map[string]any{"Realm|Alliance": H},
		"live": map[string]any{"Realm|Alliance": map[string]any{"p": map[string]any{}, "q": map[string]any{}, "t": map[string]any{}, "full": float64(hotNow.Unix() - 3*3600)}},
	}
}

var hotHouses = []FarmHouse{{Key: "Realm|Alliance", Realm: "Realm", Faction: "Alliance"}}

func hotHouse(t *testing.T, db map[string]any) FarmHotHouse {
	t.Helper()
	h := buildHot(db, 3, true, hotHouses, map[int]farmItemRef{}, map[int]bool{}, hotNow)
	if h.Message != "" || len(h.Houses) != 1 {
		t.Fatalf("hot: %+v", h)
	}
	return h.Houses[0]
}

func hotIDs(hh FarmHotHouse) []int {
	var ids []int
	for _, r := range hh.Rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// A steady item: six earlier days at 1000, today 1500 (+50% on both).
func steady(today int) []ahpHistRow {
	return series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1000, today)
}

// Strings written by AH Profit's own histEncode (AHProfit 0.6.5 at 8e7d55e,
// Core.lua, run once in Lua 5.1 for these days) decode to what the addon's
// histDecode returns, and our encoder writes the same strings.
func TestHotDecodeMatchesTheAddonEncoder(t *testing.T) {
	cases := []struct {
		s    string
		rows []ahpHistRow
	}{
		{"cZ2i90o1", []ahpHistRow{{2150, 300, 0, 50}}},
		{"yY2u3a371Y4094/nmU1eV", []ahpHistRow{{2140, 120, 100, 7}, {2141, 130, 0, 9}, {2145, 999999, 1, 1000}}},
		{"m+1WWagggZT//ZgggZTWmorvT3100", []ahpHistRow{{2000, 1e12, 999999999999, 1e9}, {2003, 1, 0, 0}}},
		{"VVUW11W10/VW1WW1/VX1", []ahpHistRow{{31, 31, 30, 32}, {32, 32, 0, 1023}, {64, 1024, 1023, 33}}},
	}
	// What the addon's own histDecode returns for each string as of its last day
	// (same Lua run): the 33-day case keeps only its last day in the 21-day window.
	addonDecoded := [][]ahpHistRow{cases[0].rows, cases[1].rows, cases[2].rows, {{64, 1024, 1023, 33}}}
	for i, c := range cases {
		got, ok := ahpHistDecode(c.s, c.rows[len(c.rows)-1].Day)
		if !ok || !reflect.DeepEqual(got, addonDecoded[i]) {
			t.Errorf("decode %q = %v %v, want %v", c.s, got, ok, addonDecoded[i])
		}
		if all, ok := ahpHistDecode(c.s, 30); i == 3 && ok {
			t.Errorf("as of day 30 its days are after today: the addon rejects the string, got %v", all)
		}
		if enc := ahpHistEncode(c.rows); enc != c.s {
			t.Errorf("encode %v = %q, want the addon's %q", c.rows, enc, c.s)
		}
	}
	// Days older than 21 are dropped, as the addon does.
	got, ok := ahpHistDecode("yY2u3a371Y4094/nmU1eV", 2145+20)
	if !ok || len(got) != 1 || got[0].Day != 2145 {
		t.Errorf("21-day window: %v %v", got, ok)
	}
}

func TestHotDecodeRejectsWhatTheAddonRejects(t *testing.T) {
	good := ahpHistEncode([]ahpHistRow{{2500, 100, 0, 5}, {2501, 120, 0, 5}})
	for name, s := range map[string]string{
		"empty":           "",
		"too long":        strings.Repeat("0", ahpHistLen+1),
		"bad character":   good[:3] + "!" + good[4:],
		"truncated":       good[:len(good)-1],
		"zero step":       ahpHistEncode([]ahpHistRow{{2500, 100, 0, 5}, {2500, 120, 0, 5}}),
		"zero price":      ahpHistEncode([]ahpHistRow{{2500, 0, 0, 5}}),
		"low not below":   ahpHistEncode([]ahpHistRow{{2500, 100, 100, 5}}),
		"price over 1e12": ahpHistEncode([]ahpHistRow{{2500, 1e12 + 1, 0, 5}}),
		"count over 1e9":  ahpHistEncode([]ahpHistRow{{2500, 100, 0, 1e9 + 1}}),
		"after today":     ahpHistEncode([]ahpHistRow{{hotToday + 1, 100, 0, 5}}),
		"endless digits":  strings.Repeat("W", 20),
	} {
		if rows, ok := ahpHistDecode(s, hotToday); ok {
			t.Errorf("%s: decoded %v", name, rows)
		}
	}
}

// A damaged string drops that item only; the page still ranks the rest.
func TestHotDamagedHistoryDropsOnlyThatItem(t *testing.T) {
	db := hotSave(map[int][]ahpHistRow{1001: steady(1500)})
	H := M(M(db["hist"])["Realm|Alliance"])
	H["1002"] = "!!broken"
	H["1003"] = float64(7)
	H["x"] = ahpHistEncode(steady(1500))
	H["01004"] = ahpHistEncode(steady(1500)) // not a key the addon writes
	hh := hotHouse(t, db)
	if !reflect.DeepEqual(hotIDs(hh), []int{1001}) || hh.Damaged != 4 {
		t.Fatalf("rows %v damaged %d", hotIDs(hh), hh.Damaged)
	}
}

// One case per filter: an item that must be kept and one that must be dropped.
func TestHotFilters(t *testing.T) {
	cases := []struct {
		name       string
		keep, drop []ahpHistRow
	}{
		{"recorded days before today >= 5",
			series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1500),
			series(hotToday, 20, 1000, 1000, 1000, 1000, 1500)},
		{"baseline >= 5 silver",
			series(hotToday, 20, 500, 500, 500, 500, 500, 800),
			series(hotToday, 20, 499, 499, 499, 499, 499, 800)},
		{"listed today >= 10",
			append(series(hotToday-1, 10, 1000, 1000, 1000, 1000, 1000), ahpHistRow{hotToday, 1500, 0, 10}),
			append(series(hotToday-1, 10, 1000, 1000, 1000, 1000, 1000), ahpHistRow{hotToday, 1500, 0, 9})},
		{"median listed earlier >= 5",
			append(series(hotToday-1, 5, 1000, 1000, 1000, 1000, 1000), ahpHistRow{hotToday, 1500, 0, 50}),
			append(series(hotToday-1, 4, 1000, 1000, 1000, 1000, 1000), ahpHistRow{hotToday, 1500, 0, 50})},
		{"now at most 10 times the baseline",
			series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 10000),
			series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 10001)},
		{"at least 20% since the previous day (both +30% against usual)",
			series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1080, 1300),
			series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1100, 1300)},
		{"at least 20% against the usual price",
			series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 900, 1200),
			series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 900, 1199)},
	}
	for _, c := range cases {
		hh := hotHouse(t, hotSave(map[int][]ahpHistRow{1: c.keep, 2: c.drop}))
		if !reflect.DeepEqual(hotIDs(hh), []int{1}) || hh.Checked != 2 || hh.Dropped != 1 {
			t.Errorf("%s: rows %v checked %d dropped %d", c.name, hotIDs(hh), hh.Checked, hh.Dropped)
		}
	}
}

// The rebound (crash yesterday, normal today) is off the list: up a lot since
// yesterday, not at all against its usual price. The slow climb (a little every
// day for weeks) ranks on its small daily change, so ten items that are really
// up push it off the ten rows.
func TestHotReboundAndSlowClimbAreOff(t *testing.T) {
	hist := map[int][]ahpHistRow{
		500: series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1000, 400, 1000),
		501: series(hotToday, 20, 1000, 1030, 1060, 1090, 1120, 1150, 1180, 1210, 1240, 1270, 1300),
	}
	for i := 0; i < 10; i++ {
		hist[600+i] = steady(1200 + 10*i)
	}
	hh := hotHouse(t, hotSave(hist))
	ids := hotIDs(hh)
	if len(ids) != 10 || ids[0] != 609 || ids[9] != 600 {
		t.Fatalf("rows %v", ids)
	}
	for _, id := range ids {
		if id == 500 || id == 501 {
			t.Fatalf("rebound or slow climb on the list: %v", ids)
		}
	}
	// Alone, both are still off: the rebound is not up against usual, the slow climb (+2% a day) is under
	// the 20% minimum rise.
	hh = hotHouse(t, hotSave(map[int][]ahpHistRow{500: hist[500], 501: hist[501]}))
	if len(hh.Rows) != 0 || hh.Checked != 2 || hh.Dropped != 2 {
		t.Fatalf("alone: %v %+v", hotIDs(hh), hh)
	}
}

func TestHotPreviousDayAndSpike(t *testing.T) {
	hist := map[int][]ahpHistRow{
		1: series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1000, 0, 1600),    // previous is 2 days back
		2: series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1000, 0, 0, 1600), // 3 days back: skipped
		3: series(hotToday, 20, 1000, 1000, 2000, 1000, 1000, 1000, 1600),       // +60% but not above an earlier high
	}
	hh := hotHouse(t, hotSave(hist))
	if !reflect.DeepEqual(hotIDs(hh), []int{1, 3}) || hh.Checked != 2 {
		t.Fatalf("rows %v checked %d", hotIDs(hh), hh.Checked)
	}
	r1, r3 := hh.Rows[0], hh.Rows[1]
	if r1.PrevDays != 2 || !r1.Spike || r1.SpikeDays != 8 || r1.ChangePct != 60 || r1.AgainstPct != 60 || r1.Baseline != 1000 || r1.BaseDays != 6 {
		t.Errorf("item 1: %+v", r1)
	}
	if r3.PrevDays != 1 || r3.Spike {
		t.Errorf("item 3 is up but no higher than its earlier 2000: %+v", r3)
	}
	// 49% above the baseline is not a spike.
	hh = hotHouse(t, hotSave(map[int][]ahpHistRow{4: steady(1490)}))
	if len(hh.Rows) != 1 || hh.Rows[0].Spike {
		t.Errorf("49%%: %+v", hh.Rows)
	}
}

// One price per day: the day's lowest (l), or h when l is 0 (they were equal).
// The baseline is the median of the earlier days; an even count averages the middle two.
func TestHotDayPriceAndMedian(t *testing.T) {
	rows := []ahpHistRow{{hotToday - 6, 5000, 1000, 20}, {hotToday - 5, 5000, 1000, 20}, {hotToday - 4, 5000, 1000, 20},
		{hotToday - 3, 5000, 1200, 20}, {hotToday - 2, 5000, 1200, 20}, {hotToday - 1, 5000, 1200, 20}, {hotToday, 9000, 1650, 20}}
	hh := hotHouse(t, hotSave(map[int][]ahpHistRow{1: rows}))
	if len(hh.Rows) != 1 {
		t.Fatalf("rows %+v", hh.Rows)
	}
	if r := hh.Rows[0]; r.Now != 1650 || r.Prev != 1200 || r.Baseline != 1100 || r.AgainstPct != 50 || r.ChangePct != 38 {
		t.Fatalf("lows, not highs, and the median 1100: %+v", r)
	}
}

// First weeks: no item has 5 earlier days, so the list ranks on the change
// alone, says so, and marks no spike.
func TestHotFirstWeeksChangeOnly(t *testing.T) {
	hh := hotHouse(t, hotSave(map[int][]ahpHistRow{
		1: series(hotToday, 20, 1000, 3000),
		2: series(hotToday, 20, 1000, 1000, 1200),
		3: series(hotToday, 20, 400, 4000),  // under 5 silver
		4: series(hotToday, 20, 1000, 1000), // not rising
	}))
	if !hh.ChangeOnly || !reflect.DeepEqual(hotIDs(hh), []int{1, 2}) || hh.Rows[0].Spike || hh.Dropped != 2 {
		t.Fatalf("change only %t rows %v %+v", hh.ChangeOnly, hotIDs(hh), hh.Rows)
	}
	// Once any item has 5 earlier days, the baseline rules apply to all.
	hh = hotHouse(t, hotSave(map[int][]ahpHistRow{1: series(hotToday, 20, 1000, 3000), 9: steady(1500)}))
	if hh.ChangeOnly || !reflect.DeepEqual(hotIDs(hh), []int{9}) {
		t.Fatalf("baseline mode: %t %v", hh.ChangeOnly, hotIDs(hh))
	}
}

// Without a scan today the house is decided on its last scan day, for every item alike: that day against the
// day before, labelled with the day. Live prices never stand in, so items do not drop one by one as they age.
func TestHotWithoutScanTodayUsesTheLastScanDay(t *testing.T) {
	db := hotSave(map[int][]ahpHistRow{
		1: series(hotToday-1, 20, 1000, 1000, 1000, 1000, 1000, 1000, 1600),
		2: series(hotToday-1, 20, 1000, 1000, 1000, 1000, 1000, 1000, 1500),
		3: series(hotToday-2, 20, 1000, 1000, 1000, 1000, 1000, 1000, 2000), // not seen on that day
	})
	L := M(M(db["live"])["Realm|Alliance"])
	for _, k := range []string{"1", "2", "3"} { // current prices of any age change nothing
		M(L["p"])[k], M(L["q"])[k], M(L["t"])[k] = float64(9000), float64(9), float64(hotNow.Unix()-30*3600)
	}
	M(L["t"])["2"] = float64(hotNow.Unix() - 3600)
	hh := hotHouse(t, db)
	if !reflect.DeepEqual(hotIDs(hh), []int{1, 2}) || hh.Checked != 2 || hh.DaysAgo != 1 {
		t.Fatalf("rows %v checked %d days ago %d", hotIDs(hh), hh.Checked, hh.DaysAgo)
	}
	if hh.Rows[0].Now != 1600 || hh.Rows[1].Now != 1500 || hh.NowDay.Format("2006-01-02") != time.Unix(ahpHistDay0+(hotToday-1)*86400, 0).UTC().Format("2006-01-02") {
		t.Fatalf("yesterday's prices and date: %+v %v", hh.Rows, hh.NowDay)
	}
	if hh.Rows[0].Chart.DotX != hotChartW-2 {
		t.Errorf("the chart ends on the scan day: %+v", hh.Rows[0].Chart)
	}
	// A scan today decides the house on today; yesterday-only items are then not compared.
	H := M(M(db["hist"])["Realm|Alliance"])
	H["4"] = ahpHistEncode(series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1000, 1700))
	hh = hotHouse(t, db)
	if !reflect.DeepEqual(hotIDs(hh), []int{4}) || hh.DaysAgo != 0 || hh.Checked != 1 {
		t.Fatalf("today: rows %v days ago %d checked %d", hotIDs(hh), hh.DaysAgo, hh.Checked)
	}
}

// The rank is the smaller of the two rises: an order by either one alone differs.
func TestHotRankIsTheSmallerRise(t *testing.T) {
	hh := hotHouse(t, hotSave(map[int][]ahpHistRow{
		1: series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 2300, 3000), // since +30%, against +200%: rank 30
		2: series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 750, 1500),  // since +100%, against +50%: rank 50
		3: series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 350, 1400),  // since +300%, against +40%: rank 40
	}))
	if !reflect.DeepEqual(hotIDs(hh), []int{2, 3, 1}) {
		t.Fatalf("by min(since, against): %v %+v", hotIDs(hh), hh.Rows)
	}
}

// Guild prices and no own history anywhere: the no-house choice says guild-only. With any own house it
// does not, whichever house the guild prices are for.
func TestHotGuildOnlyMember(t *testing.T) {
	db := hotSave(map[int][]ahpHistRow{})
	db["hist"] = map[string]any{}
	db["shared"] = map[string]any{"Other|Horde": map[string]any{"p": map[string]any{"1": float64(100)}}}
	h := buildHot(db, 3, true, nil, nil, nil, hotNow)
	if !h.GuildOnly {
		t.Fatal("guild prices, no own scans")
	}
	html, err := renderFarmBody(View{Farm: FarmView{Present: true, Hot: h}})
	if err != nil || !strings.Contains(html, `<tr data-house=""><td colspan="8" class="subtle">Hot items need your own scans for now.`) {
		t.Fatalf("render: %v", err)
	}
	own := hotSave(map[int][]ahpHistRow{1: steady(1500)})
	own["shared"] = map[string]any{"Other|Horde": map[string]any{"p": map[string]any{"1": float64(100)}}}
	if buildHot(own, 3, true, hotHouses, nil, nil, hotNow).GuildOnly {
		t.Error("an own house plus guild prices for another house is not guild-only")
	}
}

// The house's day is the last FULL scan's day: one search today (a few items) does not empty the list.
// The real-save shape: a full day yesterday, a tiny today.
func TestHotPartialTodayKeepsTheFullScanDay(t *testing.T) {
	y := hotToday - 1
	hist := map[int][]ahpHistRow{}
	for id := 1; id <= 20; id++ { // yesterday's full scan: 20 items, items 1-3 up
		p := 1000
		if id <= 3 {
			p = 1500 + 100*id
		}
		hist[id] = series(y, 20, 1000, 1000, 1000, 1000, 1000, 1000, p)
	}
	hist[1] = append(hist[1], ahpHistRow{Day: hotToday, High: 1000, Listed: 20}) // searched again today, back to normal
	hist[99] = series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1000, 0, 3000) // a search today only
	db := hotSave(hist)
	L := M(M(db["live"])["Realm|Alliance"])
	L["full"] = float64(hotNow.Unix() - 20*3600) // the last full scan: yesterday 16:00
	check := func(label string) {
		t.Helper()
		hh := hotHouse(t, db)
		if !reflect.DeepEqual(hotIDs(hh), []int{3, 2, 1}) || hh.DaysAgo != 1 || hh.Checked != 20 {
			t.Fatalf("%s: rows %v days ago %d checked %d", label, hotIDs(hh), hh.DaysAgo, hh.Checked)
		}
		if hh.Rows[2].Now != 1600 || hh.Rows[2].Chart.DotX != hotChartW-2 {
			t.Errorf("%s: item 1 is yesterday's price, its chart ends yesterday: %+v", label, hh.Rows[2])
		}
		html, _ := renderFarmBody(View{Farm: FarmView{Present: true, Houses: hotHouses, Hot: buildHot(db, 3, true, hotHouses, nil, nil, hotNow)}})
		if !strings.Contains(html, "From yesterday&#39;s scan") && !strings.Contains(html, "From yesterday's scan") {
			t.Errorf("%s: labelled from yesterday's scan", label)
		}
	}
	check("full-scan time")
	L["full"] = float64(0) // no full-scan time: the busiest-day rule picks yesterday too
	check("busiest day")
	// A full scan today makes today the day.
	for id := 2; id <= 20; id++ {
		hist[id] = append(hist[id], ahpHistRow{Day: hotToday, High: 1000, Listed: 20})
	}
	db = hotSave(hist)
	if hh := hotHouse(t, db); hh.DaysAgo != 0 {
		t.Errorf("a full scan today: days ago %d", hh.DaysAgo)
	}
	if d := hotHouseDay(map[int]int{10: 100, 11: 49, 12: 5}, nil, 12); d != 10 {
		t.Errorf("49%% of the busiest day is not a full day: %d", d)
	}
	if d := hotHouseDay(map[int]int{10: 100, 11: 50, 12: 5}, nil, 12); d != 11 {
		t.Errorf("50%% is: %d", d)
	}
}

// "Not enough history yet" only when the house lacks the days; a mature house whose chosen-day items have
// no earlier row reads "Nothing is clearly up". One item reads "1 item".
func TestHotMatureHouseNothingUp(t *testing.T) {
	db := hotSave(map[int][]ahpHistRow{
		1: series(hotToday-1, 20, 1000, 1000, 1000, 1000, 1000, 1000, 1000), // a mature item, not seen today
		2: series(hotToday, 20, 1000),                                       // new today, no earlier row
		3: series(hotToday, 20, 1000, 1000),                                 // seen yesterday and today, flat
	})
	hh := hotHouse(t, db)
	if hh.ChangeOnly || len(hh.Rows) != 0 || hh.Checked != 1 {
		t.Fatalf("mature house: %+v", hh)
	}
	html, _ := renderFarmBody(View{Farm: FarmView{Present: true, Houses: hotHouses, Hot: buildHot(db, 3, true, hotHouses, nil, nil, hotNow)}})
	if strings.Contains(html, "Not enough history yet") || !strings.Contains(html, "Nothing is clearly up.") || !strings.Contains(html, "1 item checked") {
		t.Error("mature house: nothing up, singular count")
	}
	young := hotSave(map[int][]ahpHistRow{3: series(hotToday, 20, 1000, 1000)})
	if hh := hotHouse(t, young); !hh.ChangeOnly {
		t.Error("a house without the days is in first-weeks mode")
	}
}

// A missing day is a gap in the line (a new M), not a zero; today is last and marked.
func TestHotChartGap(t *testing.T) {
	c := hotChart(series(hotToday, 20, 1000, 1000, 0, 1000, 1500), hotToday, 1500)
	if strings.Count(c.Path, "M") != 2 || strings.Count(c.Path, "L") != 2 || !c.HasDot {
		t.Fatalf("path %q", c.Path)
	}
	last := strings.Fields(c.Path)
	if x, _ := strconv.Atoi(strings.TrimLeft(last[len(last)-2], "ML")); x != c.DotX || c.DotX != hotChartW-2 || c.DotY != 2 {
		t.Fatalf("today's point: path %q dot %d,%d", c.Path, c.DotX, c.DotY)
	}
	for _, f := range strings.Fields(c.Path) {
		if y, err := strconv.Atoi(f); err == nil && y > hotChartH {
			t.Fatalf("point outside the chart: %q", c.Path)
		}
	}
}

// The day boundary is 00:00 UTC, as the addon counts days.
func TestHotDayBoundary(t *testing.T) {
	midnight := time.Unix(ahpHistDay0+hotToday*86400, 0)
	if ahpScanDay(midnight.Add(-time.Minute)) != hotToday-1 || ahpScanDay(midnight.Add(time.Minute)) != hotToday || ahpScanDay(midnight) != hotToday {
		t.Fatal("23:59 and 00:01 UTC must fall on different days")
	}
	if d := ahpScanDay(time.Unix(ahpHistDay0-1, 0)); d != -1 {
		t.Fatalf("before 2020: %d", d)
	}
}

func TestHotNoDataMessages(t *testing.T) {
	msg := func(db map[string]any, ver int) string {
		h := buildHot(db, ver, true, hotHouses, nil, nil, hotNow)
		if h.Message != "" {
			return h.Message
		}
		return h.Houses[0].Message
	}
	if m := msg(hotSave(nil), 1); m != "Update AH Profit to see hot items." {
		t.Errorf("version 1: %q", m)
	}
	if h := buildHot(hotSave(nil), 0, false, hotHouses, nil, nil, hotNow); h.Message != "Update AH Profit to see hot items." {
		t.Errorf("no version: %q", h.Message)
	}
	if m := msg(hotSave(nil), 4); !strings.Contains(m, "newer Relax Coach") {
		t.Errorf("newer save: %q", m)
	}
	guild := hotSave(nil)
	delete(M(guild["hist"]), "Realm|Alliance")
	guild["shared"] = map[string]any{"Realm|Alliance": map[string]any{"p": map[string]any{"1": float64(100)}}}
	if m := msg(guild, 3); m != "Hot items need your own scans for now. Guild price history is coming." {
		t.Errorf("guild only: %q", m)
	}
	if m := msg(hotSave(map[int][]ahpHistRow{}), 3); m != "No scan today. Scan the auction house to see what moved." {
		t.Errorf("no scan today: %q", m)
	}
	if m := msg(hotSave(map[int][]ahpHistRow{1: series(hotToday-1, 20, 1000, 1000, 1000, 1000, 1000, 1500)}), 3); m != "" {
		t.Errorf("a scan yesterday is shown, labelled, not as no scan: %q", m)
	}
	flat := hotSave(map[int][]ahpHistRow{1: steady(1000), 2: steady(900)})
	h := buildHot(flat, 3, true, hotHouses, nil, nil, hotNow)
	if hh := h.Houses[0]; hh.Message != "" || len(hh.Rows) != 0 || hh.Checked != 2 || hh.Dropped != 2 {
		t.Errorf("nothing up: %+v", hh)
	}
	html, _ := renderFarmBody(View{Farm: FarmView{Present: true, Houses: hotHouses, Hot: h}})
	if !strings.Contains(html, "Nothing is clearly up.") || !strings.Contains(html, "2 items checked, 2 left out by the filters") {
		t.Error("nothing up: the page says so with the counts")
	}
	empty := buildHot(hotSave(map[int][]ahpHistRow{}), 3, true, hotHouses, nil, nil, hotNow)
	html, _ = renderFarmBody(View{Farm: FarmView{Present: true, Houses: hotHouses, Hot: empty}})
	if !strings.Contains(html, "No scan today.") || strings.Contains(html, ", last scan ") {
		t.Error("no scan today: no contradicting last-scan line")
	}
}

// Through the real page: a save with history renders the section for its
// house, names from the save, the gather mark, the spike and the chart; save
// text stays escaped.
func TestHotRendersOnTheFarmingPage(t *testing.T) {
	rows := steady(1600)
	sv := fmt.Sprintf(`AHProfitDB = {
	["v"] = 3,
	["live"] = { ["Realm|Alliance"] = { ["p"] = { }, ["q"] = { }, ["t"] = { }, ["full"] = %d } },
	["hist"] = { ["Realm|Alliance"] = { [7001] = %q, [7002] = %q } },
	["items"] = { [7001] = { "<b>Fake Ore</b>", 5 } },
}`, hotNow.Unix()-3*3600, ahpHistEncode(rows), ahpHistEncode(series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 1000, 1300)))
	ref := farmRef{Items: map[int]farmItemRef{}, Gather: map[int]bool{7001: true}}
	fv, err := buildFarm([]byte(sv), nil, hotNow, ref)
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderFarmBody(View{Farm: fv})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="farm-hot"`, `&lt;b&gt;Fake Ore&lt;/b&gt;`, `(gather)`, `+60%`, `spike`, `highest in 7 days`, `median of 6 days`, `item 7002`, `<path d="M`, `From your own scans, last scan`, `data-house=""`} {
		if !strings.Contains(html, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(html, "<b>Fake Ore</b>") {
		t.Error("save text must stay escaped")
	}
	if i, j := strings.Index(html, "Fake Ore"), strings.Index(html, "item 7002"); i > j {
		t.Error("+60% ranks above +30%")
	}
}
