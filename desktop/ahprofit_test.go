package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Synthetic AH Profit save shaped like a real one (no private data): two
// realms, Alliance and Horde houses on the same realm, a guild layer, a bank
// per character, gathering nodes and auction counts.
const farmNow = 1794400000

func farmFixture() string {
	return fmt.Sprintf(`AHProfitDB = {
	["cut"] = 5,
	["v"] = 2,
	["live"] = {
		["Stormrage|Alliance"] = { ["p"] = { [2772] = 300, [2589] = 40, [3858] = 1200, [2840] = 50 }, ["q"] = { [2772] = 50, [2589] = 900, [3858] = 7, [2840] = 30 }, ["t"] = { [2772] = %d, [2589] = %d, [3858] = %d, [2840] = %d }, ["full"] = %d },
		["Stormrage|Horde"] = { ["p"] = { [2772] = 999 }, ["q"] = { [2772] = 3 }, ["t"] = { [2772] = %d }, ["full"] = 0 },
		["Other Realm|Alliance"] = { ["p"] = { [2589] = 77 }, ["q"] = { [2589] = 5 }, ["t"] = { [2589] = %d }, ["full"] = 0 },
	},
	["shared"] = {
		["Stormrage|Alliance"] = { ["p"] = { [2589] = 35, [3858] = 1000, [2840] = 45 }, ["q"] = { [2589] = 800, [3858] = 9, [2840] = 99 }, ["t"] = { [2589] = %d, [3858] = %d, [2840] = %d }, ["full"] = %d, ["from"] = { [%d] = "Guildmate" } },
	},
	["bank"] = {
		["Tester-Stormrage"] = { ["t"] = %d, ["items"] = { { 2772, 20, 0 }, { 2589, 100, 0 }, { 2840, 6, 0 }, { 3858, 4, 1 }, { 999999, 2, 0 }, { "bad", 1, 0 }, { 5, 3, 9 } } },
		["Grunt-Stormrage"] = { ["t"] = %d, ["items"] = { { 2772, 2, 0 } } },
		["Alt-Nowhere"] = { ["t"] = %d, ["items"] = { { 2589, 1, 0 } } },
	},
	["nodes"] = {
		[1446] = { { 11, 500, 400, %d, "Tester" }, { 12, 510, 420, %d, "Helper" }, { 11, 100, 100, %d, "Tester" } },
		[1447] = { { 13, 200, 200, %d, "Tester" } },
		["x"] = { { 1, 1, 1, 1, "y" } },
	},
	["auc"] = {
		["stat"] = { ["Iron Ore"] = { 6, 2 }, ["<b>Linen</b>"] = { 1, 3 }, ["Never"] = { 0, 0 } },
		["own"] = { ["Tester-Stormrage"] = { ["t"] = %d, ["list"] = { { 2772, 20, 310, 0 }, { 3858, 1, 1500, 1 } } } },
	},
	["rec"] = { },
}
`, farmNow-600, farmNow-9000, farmNow-9000, farmNow-1800, farmNow-600, // own: iron fresh, linen old, mithril old, copper bar 30 min
		farmNow-60, farmNow-60, // horde, other realm
		farmNow-1200, farmNow-20000, farmNow-300, farmNow-1200, farmNow-1200, // guild: linen newer than own, mithril older, copper bar newer than a fresh own scan
		farmNow-3600, farmNow-3600, farmNow-3600,
		farmNow-100, farmNow-50, farmNow-7000, farmNow-400,
		farmNow-30)
}

func farmChars() []CharView {
	return []CharView{
		{Name: "Tester", Realm: "Stormrage", Faction: "Alliance", Updated: time.Unix(farmNow-100, 0)},
		{Name: "Grunt", Realm: "Stormrage", Faction: "Horde", Updated: time.Unix(farmNow-5000, 0)},
	}
}

func farmRow(fv FarmView, char string, id int) *FarmSellRow {
	for i := range fv.Sell {
		if fv.Sell[i].Char == char && fv.Sell[i].ID == id {
			return &fv.Sell[i]
		}
	}
	return nil
}

func TestFarmBuildPricesHousesAndSources(t *testing.T) {
	fv, err := buildFarm([]byte(farmFixture()), farmChars(), time.Unix(farmNow, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !fv.Present || fv.Cut != 5 || fv.Note != "" || fv.HasLedger || fv.HasSessions {
		t.Fatalf("header: %+v", fv)
	}
	if len(fv.Houses) != 3 || fv.DefaultHouse != "Stormrage|Alliance" {
		t.Fatalf("houses %+v default %q", fv.Houses, fv.DefaultHouse)
	}
	iron := farmRow(fv, "Tester-Stormrage", 2772)
	if iron == nil || iron.House != "Stormrage|Alliance" || iron.Price != 300 || iron.Source != "scan" || iron.Net != 285 || iron.Value != 285*20 || iron.Listed != 50 {
		t.Fatalf("own fresh scan: %+v", iron)
	}
	linen := farmRow(fv, "Tester-Stormrage", 2589)
	if linen == nil || linen.Price != 35 || linen.Source != "guild" || linen.From != "Guildmate" {
		t.Fatalf("own scan older than 2 h must lose to a newer guild price: %+v", linen)
	}
	mith := farmRow(fv, "Tester-Stormrage", 3858)
	if mith == nil || mith.Price != 1200 || mith.Source != "scan" || mith.Value != 1140*3 || mith.Bound != 1 {
		t.Fatalf("guild price older than own must not win; bound items not valued: %+v", mith)
	}
	bar := farmRow(fv, "Tester-Stormrage", 2840)
	if bar == nil || bar.Price != 50 || bar.Source != "scan" {
		t.Fatalf("an own scan under 2 h wins even over a newer guild price: %+v", bar)
	}
	grunt := farmRow(fv, "Grunt-Stormrage", 2772)
	if grunt == nil || grunt.House != "Stormrage|Horde" || grunt.Price != 999 {
		t.Fatalf("a Horde character uses the Horde house of the same realm: %+v", grunt)
	}
	alt := farmRow(fv, "Alt-Nowhere", 2589)
	if alt == nil || alt.House != "" || !alt.NoPrice {
		t.Fatalf("a character of an unknown realm is never priced from another house: %+v", alt)
	}
	if r := farmRow(fv, "Tester-Stormrage", 999999); r == nil || !r.NoPrice || r.Name != "item 999999" {
		t.Fatalf("unpriced item: %+v", r)
	}
	if farmRow(fv, "Tester-Stormrage", 5) != nil {
		t.Fatal("a row with more bound than held must be dropped")
	}
	if fv.Sell[0].ID != 2772 || fv.Sell[0].Char != "Tester-Stormrage" {
		t.Fatalf("highest value first: %+v", fv.Sell[0])
	}
	if len(fv.Gathering) != 2 || fv.Gathering[0].MapID != 1446 || fv.Gathering[0].Nodes != 3 || fv.Gathering[0].Kinds != 2 ||
		strings.Join(fv.Gathering[0].Who, ",") != "Helper,Tester" || !fv.Gathering[0].Last.Equal(time.Unix(farmNow-50, 0)) {
		t.Fatalf("gathering %+v", fv.Gathering)
	}
	if len(fv.Auctions) != 2 || fv.Auctions[0].Name != "Iron Ore" || fv.Auctions[0].RatePct != 75 || fv.Auctions[1].RatePct != 25 {
		t.Fatalf("auction counts %+v", fv.Auctions)
	}
	if len(fv.Listings) != 1 || fv.Listings[0].Active != 1 || fv.Listings[0].SoldWaiting != 1 || fv.Listings[0].ActiveAfterCut != 294*20 {
		t.Fatalf("listings %+v", fv.Listings)
	}
}

func TestFarmCutAndSchemaVersions(t *testing.T) {
	src := strings.Replace(farmFixture(), `["cut"] = 5`, `["cut"] = 15`, 1)
	fv, err := buildFarm([]byte(src), farmChars(), time.Unix(farmNow, 0))
	if err != nil || fv.Cut != 15 || farmRow(fv, "Tester-Stormrage", 2772).Net != 255 {
		t.Fatalf("15%% cut: %v %+v", err, fv.Cut)
	}
	newer := strings.Replace(farmFixture(), `["v"] = 2`, `["v"] = 3`, 1)
	fv, err = buildFarm([]byte(newer), farmChars(), time.Unix(farmNow, 0))
	if err != nil || !strings.Contains(fv.Note, "newer AH Profit") || len(fv.Sell) == 0 {
		t.Fatalf("a newer save is still read, with a note: %v %q", err, fv.Note)
	}
	old := `AHProfitDB = { ["cut"] = 5, ["live"] = { ["R|Horde"] = { ["p"] = { [1] = 5 }, ["q"] = { [1] = 1 }, ["t"] = { [1] = 1 }, ["full"] = 0 } } }`
	fv, err = buildFarm([]byte(old), nil, time.Unix(farmNow, 0))
	if err != nil || fv.HasLedger || fv.DefaultHouse != "R|Horde" || len(fv.Sell) != 0 || len(fv.Houses) != 1 {
		t.Fatalf("old save without newer fields: %v %+v", err, fv)
	}
	withLedger := strings.Replace(farmFixture(), `["rec"] = { },`, `["rec"] = { }, ["ledger"] = { }, ["sessions"] = { },`, 1)
	fv, err = buildFarm([]byte(withLedger), farmChars(), time.Unix(farmNow, 0))
	if err != nil || !fv.HasLedger || !fv.HasSessions {
		t.Fatalf("ledger/sessions present: %v %+v", err, fv)
	}
	if _, err := buildFarm([]byte(`OtherDB = { }`), nil, time.Now()); err == nil {
		t.Fatal("a file without AHProfitDB must be an error")
	}
	// Two characters with the same name on one realm, both factions, and no
	// Coach data: the house is unknown rather than guessed.
	fv, _ = buildFarm([]byte(farmFixture()), nil, time.Unix(farmNow, 0))
	if r := farmRow(fv, "Tester-Stormrage", 2772); r == nil || r.House != "" || !r.NoPrice || fv.DefaultHouse != "" {
		t.Fatalf("no Coach characters and two houses on the realm: house must stay unknown: %+v default %q", r, fv.DefaultHouse)
	}
}

func TestFarmTruncatedNeverPanics(t *testing.T) {
	src := farmFixture()
	for i := 0; i <= len(src); i++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic at byte %d: %v", i, r)
				}
			}()
			buildFarm([]byte(src[:i]), farmChars(), time.Unix(farmNow, 0))
		}()
	}
}

func FuzzBuildFarm(f *testing.F) {
	f.Add(farmFixture())
	f.Add(`AHProfitDB = { ["bank"] = { ["a-b"] = { ["items"] = { { 1e308, -1, 2 } } } }, ["nodes"] = { [1] = { { 1, 2, 3, 1e300, 5 } } } }`)
	f.Add(`AHProfitDB = { ["auc"] = { ["own"] = { ["x"] = { ["list"] = { { 1, 1e9, 1e12, 0 } } } } } }`)
	f.Fuzz(func(t *testing.T, s string) {
		fv, err := buildFarm([]byte(s), farmChars(), time.Unix(farmNow, 0))
		if err == nil {
			if _, rerr := renderFarmBody(View{Farm: fv}); rerr != nil {
				t.Fatal(rerr)
			}
		}
	})
}

func farmWrite(t *testing.T, game, account, body string) string {
	t.Helper()
	p := filepath.Join(game, "WTF", "Account", account, "SavedVariables", "AHProfit.lua")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFarmSameAccountOnlyAndRetention(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	accountSave(t, game, "B#2", "Beta")
	farmWrite(t, game, "B#2", farmFixture())
	a, b := accountApp(t, game)
	if err := a.selectAccount("A#1"); err != nil {
		t.Fatal(err)
	}
	p := accountProjection(t, b)
	if p["HasFarm"] != false || p["FarmHTML"] != "" || a.snapshot().Farm.Present {
		t.Fatal("an account without AHProfit.lua must not show another account's farm data")
	}
	if err := a.selectAccount("B#2"); err != nil {
		t.Fatal(err)
	}
	p = accountProjection(t, b)
	html, _ := p["FarmHTML"].(string)
	if p["HasFarm"] != true || !strings.Contains(html, "Sell now") || !strings.Contains(html, "&lt;b&gt;Linen&lt;/b&gt;") || strings.Contains(html, "<b>Linen</b>") {
		t.Fatalf("farm page for the selected account, escaped: has=%v", p["HasFarm"])
	}
	before := a.snapshot().Farm
	farmWrite(t, game, "B#2", "AHProfitDB = {")
	p = accountProjection(t, b)
	after := a.snapshot().Farm
	if p["HasFarm"] != true || !strings.Contains(after.Warning, "previous one") || len(after.Sell) != len(before.Sell) {
		t.Fatalf("a half-written save keeps the last good farm data with a warning: %q", after.Warning)
	}
	farmWrite(t, game, "A#1", "AHProfitDB = {")
	if err := a.selectAccount("A#1"); err != nil {
		t.Fatal(err)
	}
	accountProjection(t, b)
	if f := a.snapshot().Farm; len(f.Sell) != 0 || f.Error == "" || f.Warning != "" {
		t.Fatalf("last-good farm data must never cross to another account: %d rows, warning %q", len(f.Sell), f.Warning)
	}
	os.Remove(filepath.Join(game, "WTF", "Account", "A#1", "SavedVariables", "AHProfit.lua"))
	accountProjection(t, b)
	if a.snapshot().Farm.Present {
		t.Fatal("an account without AHProfit.lua has no farm page")
	}
	os.Remove(filepath.Join(game, "WTF", "Account", "B#2", "SavedVariables", "AHProfit.lua"))
	if err := a.selectAccount("B#2"); err != nil {
		t.Fatal(err)
	}
	if p = accountProjection(t, b); p["HasFarm"] != false {
		t.Fatal("removing AHProfit.lua hides the page")
	}
}

// The same rule as the Coach save: a selected identity that is not in the
// current enumeration (its ForeverCoach.lua is gone, or on Windows only a
// differently cased folder exists) is never read under that name.
func TestFarmSelectedIdentityMustBeEnumerated(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	coach := accountSave(t, game, "A#1", "Alpha")
	farmWrite(t, game, "A#1", farmFixture())
	a, b := accountApp(t, game)
	if p := accountProjection(t, b); p["HasFarm"] != true {
		t.Fatal("farm page for the selected account")
	}
	if err := os.Remove(coach); err != nil {
		t.Fatal(err)
	}
	if p := accountProjection(t, b); p["HasFarm"] != false || a.snapshot().Farm.Present {
		t.Fatal("AHProfit.lua read for an identity that is no longer enumerated")
	}
}

func TestFarmBrokenFirstReadAndSizeCap(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	farmWrite(t, game, "A#1", "AHProfitDB = { [\"live\"] = ")
	a, b := accountApp(t, game)
	p := accountProjection(t, b)
	if p["HasFarm"] != true || !strings.Contains(p["FarmHTML"].(string), "could not be read") || a.snapshot().Farm.Error == "" {
		t.Fatal("a broken save with nothing earlier says so instead of hiding the page")
	}
	old := ahpMaxBytes
	ahpMaxBytes = 64
	t.Cleanup(func() { ahpMaxBytes = old })
	farmWrite(t, game, "A#1", farmFixture())
	accountProjection(t, b)
	if f := a.snapshot().Farm; !strings.Contains(f.Error, "larger than") {
		t.Fatalf("size cap: %+v", f.Error)
	}
	if a.snapshot().Error != "" {
		t.Fatal("an AH Profit problem must not touch the Coach report")
	}
}

func TestFarmNeverWrites(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	p := farmWrite(t, game, "A#1", farmFixture())
	st0, _ := os.Stat(p)
	_, b := accountApp(t, game)
	for i := 0; i < 3; i++ {
		accountProjection(t, b)
	}
	st1, _ := os.Stat(p)
	data, _ := os.ReadFile(p)
	if !st1.ModTime().Equal(st0.ModTime()) || string(data) != farmFixture() {
		t.Fatal("AHProfit.lua changed")
	}
}

func TestFarmCoin(t *testing.T) {
	for in, want := range map[int]string{0: "0c", 7: "7c", 100: "1s 0c", 12345: "1g 23s 45c", 123456789: "12,345g 67s 89c", -150: "-1s 50c"} {
		if got := coin(in); got != want {
			t.Fatalf("coin(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestFarmRenderForUITests writes the Farming fragment of the synthetic save
// for the jsdom tests (native_farm_ui_test.cjs) when FARM_HTML_OUT is set.
func TestFarmRenderForUITests(t *testing.T) {
	fv, err := buildFarm([]byte(farmFixture()), farmChars(), time.Unix(farmNow, 0))
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderFarmBody(View{Farm: fv})
	if err != nil || !strings.Contains(html, `id="farm-house"`) || !strings.Contains(html, `data-house="Stormrage|Horde"`) {
		t.Fatalf("render: %v", err)
	}
	if out := os.Getenv("FARM_HTML_OUT"); out != "" {
		if err := os.WriteFile(out, []byte(html), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
