package main

// Farming page, Gold and Sessions (AH Profit saved-data version 3: ledger,
// sessions and items). Synthetic fixtures in the addon's format.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// goldDay is AH Profit's day number (UTC days since 2020-01-01).
func goldDay(t time.Time) int { return int((t.Unix() - 1577836800) / 86400) }

func goldFixture(v int, now time.Time) string {
	d := goldDay(now)
	start := now.Add(-3 * time.Hour).Unix()
	return fmt.Sprintf(`AHProfitDB = {
	["v"] = %d,
	["cut"] = 5,
	["ledger"] = {
		["Abdullah-Stormrage"] = {
			[%d] = { ["loot"] = { 5000, 0 }, ["vendor"] = { 12000, 300 }, ["repair"] = { 0, 2500 } },
			[%d] = { ["ah"] = { 90000, 6000 } },
			[%d] = { ["quest"] = { 7000, 0 } },
			[%d] = { ["taxi"] = { 0, 400 } },
			[%d] = { ["loot"] = { 100, 0 } },
		},
		["Ahmed-Stormrage"] = {
			[%d] = { ["trainer"] = { 0, 10000 }, ["mail"] = { 2000, 0 } },
			[%d] = { ["other"] = { 0, 100 } },
			[%d] = { ["other"] = { 0, 77 } },
		},
	},
	["sessions"] = {
		{ ["char"] = "Abdullah", ["realm"] = "Stormrage", ["faction"] = "Alliance", ["level"] = 30, ["map"] = 1429, ["zone"] = "Elwynn Forest",
		  ["start"] = %d, ["stop"] = %d, ["dur"] = 3600, ["inc"] = { ["loot"] = 4000, ["vendor"] = 6000 }, ["out"] = { ["repair"] = 1000 },
		  ["items"] = { { 2589, 12 }, { 2770, -3 } }, ["nodes"] = 4 },
		{ ["char"] = "Ahmed", ["realm"] = "Stormrage", ["faction"] = "Alliance", ["level"] = 20, ["map"] = 1436,
		  ["start"] = %d, ["stop"] = %d, ["dur"] = 1800, ["inc"] = { }, ["out"] = { ["vendor"] = 500 },
		  ["items"] = { }, ["nodes"] = 0 },
	},
	["items"] = { [2589] = { "Linen Cloth", 13, %d } },
}`, v, d, d-3, d-20, d-100, d-7, d, d-30, d-180, start, start+3700, start+4000, start+5800, now.Unix())
}

func goldRender(t *testing.T, sv string, now time.Time) (FarmView, string) {
	t.Helper()
	fv, err := buildFarm([]byte(sv), nil, now, farmRef{Maps: map[int]string{1436: "Westfall"}})
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderFarmBody(View{Farm: fv})
	if err != nil {
		t.Fatal(err)
	}
	return fv, html
}

func goldCard(html, id string) string {
	i := strings.Index(html, `id="`+id+`"`)
	if i < 0 {
		return ""
	}
	j := strings.Index(html[i:], `<div class="card"`)
	if j < 0 {
		return html[i:]
	}
	return html[i : i+j]
}

func TestFarmGoldTotalsByPeriod(t *testing.T) {
	now := time.Unix(farmNow, 0)
	_, html := goldRender(t, goldFixture(3, now), now)
	g := goldCard(html, "farm-gold")
	// today: in 5000+12000+2000 = 19000, out 300+2500+10000 = 12800
	// 7 days (today and the 6 before): + ah 90000 / 6000; 30 days: + quest 7000 and
	// loot 100 at day -7; all: + taxi 400 out (day -100) and other 100 out (day -30,
	// just outside 30 days); other 77 out at day -180 is past what is kept
	for _, want := range []string{
		`<tr><th>Earned</th><td class="num">1g 90s 0c</td><td class="num">10g 90s 0c</td><td class="num">11g 61s 0c</td><td class="num">11g 61s 0c</td></tr>`,
		`<tr><th>Spent</th><td class="num">1g 28s 0c</td><td class="num">1g 88s 0c</td><td class="num">1g 88s 0c</td><td class="num">1g 93s 0c</td></tr>`,
		`<tr><th>Net</th><td class="num">+62s 0c</td><td class="num">+9g 2s 0c</td><td class="num">+9g 73s 0c</td><td class="num">+9g 68s 0c</td></tr>`,
	} {
		if !strings.Contains(g, want) {
			t.Fatalf("missing %s in\n%s", want, g)
		}
	}
	if strings.Contains(g, "Update AH Profit") || strings.Contains(g, "does not show") {
		t.Fatalf("a version 3 save shows numbers, not the update note:\n%s", g)
	}
}

func TestFarmGoldBySourceAndCharacter(t *testing.T) {
	now := time.Unix(farmNow, 0)
	_, html := goldRender(t, goldFixture(3, now), now)
	g := goldCard(html, "farm-gold")
	for _, want := range []string{
		`<tr><td>Auction house</td><td class="num">9g 0s 0c</td><td class="num">60s 0c</td></tr>`,
		`<tr><td>Trainer</td><td class="num">0c</td><td class="num">1g 0s 0c</td></tr>`,
		`<tr><td>Repairs</td><td class="num">0c</td><td class="num">25s 0c</td></tr>`,
		`<tr><td>Abdullah-Stormrage</td><td class="num">11g 41s 0c</td><td class="num">88s 0c</td><td class="num">+10g 53s 0c</td></tr>`,
		`<tr><td>Ahmed-Stormrage</td><td class="num">20s 0c</td><td class="num">1g 0s 0c</td><td class="num">-80s 0c</td></tr>`,
	} {
		if !strings.Contains(g, want) {
			t.Fatalf("missing %s in\n%s", want, g)
		}
	}
	if strings.Contains(g, "Flights") {
		t.Fatalf("a source only older than 30 days is not in the 30-day breakdown:\n%s", g)
	}
	// the biggest source first
	if strings.Index(g, "Auction house") > strings.Index(g, "Vendor") {
		t.Fatal("sources sorted by money moved, biggest first")
	}
}

func TestFarmSessionsListed(t *testing.T) {
	now := time.Unix(farmNow, 0)
	_, html := goldRender(t, goldFixture(3, now), now)
	s := goldCard(html, "farm-sessions")
	// newest first; net 10000-1000 = 9000 over 1 h; zone from the save, else the map name
	for _, want := range []string{
		`<td>Ahmed</td><td>Westfall</td>`,
		`<td>Abdullah</td><td>Elwynn Forest</td><td>1h 0m</td><td class="num">+90s 0c</td><td class="num">+90s 0c</td><td class="num">4</td>`,
		`<td>30m</td><td class="num">-5s 0c</td><td class="num">-10s 0c</td>`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in\n%s", want, s)
		}
	}
	if strings.Index(s, "Ahmed") > strings.Index(s, "Abdullah") {
		t.Fatal("newest session first")
	}
}

func TestFarmGoldOnlyFromVersion3(t *testing.T) {
	now := time.Unix(farmNow, 0)
	for _, v := range []int{2, 4} {
		fv, html := goldRender(t, goldFixture(v, now), now)
		g, s := goldCard(html, "farm-gold"), goldCard(html, "farm-sessions")
		if strings.Contains(g, "Earned") || strings.Contains(s, "Elwynn") || fv.HasLedger || fv.HasSessions {
			t.Fatalf("v=%d: ledger and sessions are read only from version 3:\n%s\n%s", v, g, s)
		}
		if v == 2 && (!strings.Contains(g, "Update AH Profit") || !strings.Contains(s, "Update AH Profit")) {
			t.Fatalf("v=2: the update note stays:\n%s", g)
		}
		if v == 4 && (strings.Contains(g+s, "Update AH Profit") || !strings.Contains(g, "needs a newer Relax Coach") || !strings.Contains(s, "need a newer Relax Coach")) {
			t.Fatalf("v=4: a newer AH Profit is not told to update itself:\n%s\n%s", g, s)
		}
	}
	// item names from the save only at version 3
	sv := strings.Replace(goldFixture(2, now), `["cut"] = 5,`, `["cut"] = 5, ["bank"] = { ["Abdullah-Stormrage"] = { ["t"] = 1, ["items"] = { { 2589, 3, 0 } } } },`, 1)
	_, html := goldRender(t, sv, now)
	if strings.Contains(html, "Linen Cloth") {
		t.Fatal("v=2: the save's item names are not read")
	}
	_, html = goldRender(t, strings.Replace(sv, `["v"] = 2`, `["v"] = 3`, 1), now)
	if !strings.Contains(html, "Linen Cloth") {
		t.Fatal("v=3: the save's item names are read")
	}
}

func TestFarmGoldIgnoresBadEntries(t *testing.T) {
	now := time.Unix(farmNow, 0)
	d := goldDay(now)
	sv := fmt.Sprintf(`AHProfitDB = { ["v"] = 3, ["ledger"] = {
		["Good-R"] = { [%d] = { ["loot"] = { 100, 0 }, ["nope"] = { 5, 5 }, ["vendor"] = { -1, 0 }, ["ah"] = { 1.5, 0 }, ["mail"] = { "9", 0 }, ["quest"] = { 50 }, ["trade"] = { 30, -1 } } },
		["Future-R"] = { [%d] = { ["loot"] = { 777, 0 } } },
		["Bad-R"] = "x",
	}, ["sessions"] = { "x", { ["start"] = 1 }, { ["char"] = "<b>X</b>", ["start"] = %d, ["stop"] = %d, ["dur"] = -5, ["inc"] = { }, ["out"] = { } } } }`,
		d, d+5, now.Unix()-100, now.Unix())
	_, html := goldRender(t, sv, now)
	g, s := goldCard(html, "farm-gold"), goldCard(html, "farm-sessions")
	if !strings.Contains(g, `<tr><th>Earned</th><td class="num">1s 0c</td>`) || strings.Contains(g, "7s 77c") {
		t.Fatalf("only valid entries count; days after today are ignored:\n%s", g)
	}
	if strings.Contains(s, "<b>X</b>") || strings.Contains(s, "&lt;b&gt;") {
		t.Fatalf("broken sessions (negative duration, missing fields) are skipped:\n%s", s)
	}
	if !strings.Contains(s, "No sessions") {
		t.Fatalf("with no valid session, an empty note:\n%s", s)
	}
}

func TestFarmGoldCaps(t *testing.T) {
	now := time.Unix(farmNow, 0)
	d := goldDay(now)
	var b strings.Builder
	b.WriteString(`AHProfitDB = { ["v"] = 3, ["ledger"] = {`)
	for c := 0; c < 70; c++ {
		fmt.Fprintf(&b, `["C%02d-R"] = { [%d] = { ["loot"] = { %d, 0 } } },`, c, d, 100+c)
	}
	b.WriteString(`}, ["sessions"] = {`)
	for i := 0; i < 130; i++ {
		fmt.Fprintf(&b, `{ ["char"] = "S%03d", ["start"] = %d, ["stop"] = %d, ["dur"] = 60, ["inc"] = { ["loot"] = 1 }, ["out"] = { }, ["items"] = { } },`, i, now.Unix()-int64(10000-i*10), now.Unix()-int64(9000-i*10))
	}
	b.WriteString(`} }`)
	_, html := goldRender(t, b.String(), now)
	g, s := goldCard(html, "farm-gold"), goldCard(html, "farm-sessions")
	if n := strings.Count(g, `-R</td>`); n != 50 {
		t.Fatalf("at most 50 characters listed, got %d", n)
	}
	if n := strings.Count(s, "<td>S"); n != 100 || !strings.Contains(s, "<td>S129</td>") || strings.Contains(s, "<td>S029</td>") {
		t.Fatalf("at most 100 sessions, the newest, got %d", n)
	}
}

// testdata/ahprofit_v3_from_addon.lua was written by AH Profit's own code
// (buiden/AHProfit #6, cf9070e, run under Lua 5.1 with synthetic events: a
// vendor sale, Repair All, an auction invoice from the mailbox, a stopped
// session, a trainer on a second character, a quest three days later). It
// checks that the page reads what the addon really writes.
func TestFarmGoldFromAddonOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/ahprofit_v3_from_addon.lua")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1794661911, 0)
	fv, html := goldRender(t, string(data), now)
	g, s := goldCard(html, "farm-gold"), goldCard(html, "farm-sessions")
	for _, want := range []string{
		`<tr><th>Earned</th><td class="num">7s 0c</td><td class="num">1g 44s 0c</td>`,
		`<tr><th>Spent</th><td class="num">0c</td><td class="num">50s 0c</td>`,
		`<tr><td>Auction house</td><td class="num">90s 0c</td><td class="num">0c</td></tr>`,
		`<tr><td>Trainer</td><td class="num">0c</td><td class="num">40s 0c</td></tr>`,
		`<tr><td>Ahmed-Realm</td>`,
	} {
		if !strings.Contains(g, want) {
			t.Fatalf("missing %s in\n%s", want, g)
		}
	}
	if !strings.Contains(s, `<td>Abdullah</td><td>Elwynn Forest</td><td>45m</td><td class="num">+30s 0c</td><td class="num">+40s 0c</td><td class="num">5</td>`) {
		t.Fatalf("the addon's session:\n%s", s)
	}
	if !fv.HasLedger || !fv.HasSessions || fv.Note != "" {
		t.Fatalf("a version 3 save from the addon is fully read: %+v", fv)
	}
}
