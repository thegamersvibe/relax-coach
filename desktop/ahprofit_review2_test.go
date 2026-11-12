package main

// Review round 2 of the Farming page names (F1, S5, S6, N1, N3).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func review2Bundle(name string, vendor int) string {
	return fmt.Sprintf("ns.farmData = {1,{{%q,2840,100,1,%d,1}},{{\"Iron Deposit\",1,1,1,1,{}}},{{\"Zone\",1,2,0,1,{},{},{}}}}\n", name, vendor)
}

func review2Game(t *testing.T, name string, vendor int, stamp time.Time) string {
	t.Helper()
	g := farmGame(t, review2Bundle(name, vendor), "")
	if err := os.Chtimes(filepath.Join(g, "Interface", "AddOns", "AHProfit", "FarmData.lua"), stamp, stamp); err != nil {
		t.Fatal(err)
	}
	accountSave(t, g, "A#1", "Synthetic")
	farmWrite(t, g, "A#1", `AHProfitDB={v=2,live={["Realm|Alliance"]={p={[2840]=100},t={[2840]=1794399999}}},bank={["Synthetic-Realm"]={t=1794399999,items={{2840,10,0}}}}}`)
	return g
}

// F1: two game folders whose bundled files have the same size and time
// never share names (the reviewer's Alpha/Bravo probe).
func TestFarmRefCacheNeverCrossesGames(t *testing.T) {
	accountConfigEnv(t)
	stamp := time.Unix(1700000000, 0)
	ga := review2Game(t, "AlphaBundle", 20, stamp)
	gb := review2Game(t, "BravoBundle", 80, stamp)
	a, b := accountApp(t, ga)
	accountProjection(t, b)
	if r := a.snapshot().Farm.Sell; len(r) == 0 || r[0].Name != "AlphaBundle" {
		t.Fatalf("game A names: %+v", r)
	}
	if _, err := b.Call("setGame", nativeArgs(gb)); err != nil {
		t.Fatal(err)
	}
	accountProjection(t, b)
	if r := a.snapshot().Farm.Sell; len(r) == 0 || r[0].Name != "BravoBundle" || r[0].Vendor != 80 {
		t.Fatalf("game B shows game A's names: %+v", r)
	}
	// the key itself names the game, so even a shared cache cannot mix them
	var cache farmRefCache
	loadFarmRef(ga, &cache)
	if ref := loadFarmRef(gb, &cache); ref.Items[2840].Name != "BravoBundle" {
		t.Fatalf("shared cache across games: %+v", ref.Items[2840])
	}
	// setGame clears the cached names and the cached parsed save
	a.setGame(ga)
	a.mu.Lock()
	refKey, parseKey := a.farmRef.key, a.farmParse.key
	a.mu.Unlock()
	if refKey != "" || parseKey != "" {
		t.Fatalf("setGame keeps caches: %q %q", refKey, parseKey)
	}
}

// S5: a stack that is all bound is not an auction gain; equal prices are
// not a green "+0c".
func TestFarmBoundAndEqualVendorTexts(t *testing.T) {
	now := time.Unix(farmNow, 0)
	sv := fmt.Sprintf(`AHProfitDB={v=3,items={[1]={"Sword",50000},[2]={"Eq",950},[3]={"Part",500}},live={["R|Alliance"]={p={[1]=100000,[2]=1000,[3]=1000},q={},t={[1]=%d,[2]=%d,[3]=%d}}},bank={["A-R"]={t=1,items={{1,1,1},{2,1,0},{3,3,2}}}}}`, farmNow, farmNow, farmNow)
	fv, err := buildFarm([]byte(sv), []CharView{{Name: "A", Realm: "R", Faction: "Alliance", Updated: now}}, now, farmRef{})
	if err != nil {
		t.Fatal(err)
	}
	if r := farmRow(fv, "A-R", 1); !r.AllBound || r.Gain != 0 || r.Value != 0 {
		t.Fatalf("all bound: %+v", r)
	}
	if r := farmRow(fv, "A-R", 3); r.AllBound || r.Gain != 450 {
		t.Fatalf("partly bound: gain on the unbound one only: %+v", r)
	}
	html, _ := renderFarmBody(View{Farm: fv})
	if strings.Contains(html, "+0c over vendoring") {
		t.Fatal("no green +0c")
	}
	for _, want := range []string{"bound, can't be auctioned", "vendor pays 5g 0s 0c each", "same as a vendor"} {
		if !strings.Contains(html, want) {
			t.Fatalf("render lacks %q", want)
		}
	}
}

// S6: every house that has rows can be chosen, scanned or not.
func TestFarmHousesWithRowsAreListed(t *testing.T) {
	now := time.Unix(farmNow, 0)
	chars := append(farmChars(), CharView{Name: "Alt", Realm: "Nowhere", Faction: "Horde", Updated: now.Add(-time.Hour)})
	fv, err := buildFarm([]byte(farmFixture()), chars, now, farmRef{})
	if err != nil {
		t.Fatal(err)
	}
	var found *FarmHouse
	for i := range fv.Houses {
		if fv.Houses[i].Key == "Nowhere|Horde" {
			found = &fv.Houses[i]
		}
	}
	if found == nil || found.Items != 0 || found.Realm != "Nowhere" || found.Faction != "Horde" {
		t.Fatalf("an unscanned house with rows is listed: %+v", fv.Houses)
	}
	html, _ := renderFarmBody(View{Farm: fv})
	if !strings.Contains(html, `<option value="Nowhere|Horde">Nowhere · Horde (not scanned)</option>`) {
		t.Fatal("render lists it as not scanned")
	}
	if r := farmRow(fv, "Alt-Nowhere", 2589); r == nil || r.House != "Nowhere|Horde" || !r.NoPrice {
		t.Fatalf("its row: %+v", r)
	}
}

// N1: name and vendor price are taken per field: the save first, then the
// bundled files, independently.
func TestFarmNameAndVendorPerField(t *testing.T) {
	ref := loadFarmRef(farmGame(t, farmDataFixture, historyFixture), nil)
	items := map[int]farmItemRef{}
	for id, it := range ref.Items {
		items[id] = it
	}
	saveItemRefs(map[string]any{"items": map[string]any{
		"2840": map[string]any{"1": "Copper Bar (save)"},                // name only: bundled vendor 10 stays
		"2772": map[string]any{"1": "Iron Ore (save)", "2": float64(7)}, // both
		"3858": map[string]any{"1": float64(5), "2": float64(40)},       // vendor only (bad name): bundled name stays
		"2589": map[string]any{"1": "Linen (save)", "2": "40"},          // bad vendor: History gives none
	}}, items)
	if it := items[2840]; it.Name != "Copper Bar (save)" || !it.VendorKnown || it.Vendor != 10 {
		t.Fatalf("name only: %+v", it)
	}
	if it := items[2772]; it.Name != "Iron Ore (save)" || !it.VendorKnown || it.Vendor != 7 {
		t.Fatalf("both: %+v", it)
	}
	if it := items[3858]; it.Name != "Mithril Ore" || !it.VendorKnown || it.Vendor != 40 {
		t.Fatalf("vendor only: %+v", it)
	}
	if it := items[2589]; it.Name != "Linen (save)" || it.VendorKnown {
		t.Fatalf("bad vendor: %+v", it)
	}
}

// N3: if the first data row of a bundled file has an unexpected shape, no
// bundled names are used at all.
func TestFarmRefShapeCheck(t *testing.T) {
	if ref := loadFarmRef(farmGame(t, farmDataFixture, historyFixture), nil); len(ref.Items) == 0 || len(ref.Nodes) == 0 || len(ref.Maps) == 0 {
		t.Fatal("the expected shape is read")
	}
	for name, c := range map[string][2]string{
		"FarmData item column inserted": {strings.Replace(farmDataFixture, `{"Iron Ore",2772,308,10733,0,1}`, `{"Iron Ore",2772,308,10733,0,0,1}`, 1), historyFixture},
		"FarmData item id as text":      {strings.Replace(farmDataFixture, `{"Iron Ore",2772,`, `{"Iron Ore","2772",`, 1), historyFixture},
		"FarmData arrays reordered":     {strings.Replace(farmDataFixture, `ns.farmData = {1794385517,{`, `ns.farmData = {1794385517,{{"Arathi Highlands",30,40,0,1417,{},{},{}}},{`, 1), historyFixture},
		"FarmData zone mapID as text":   {strings.Replace(farmDataFixture, `{"Arathi Highlands",30,40,0,1417,`, `{"Arathi Highlands",30,40,0,"1417",`, 1), historyFixture},
		"FarmData node name as number":  {strings.Replace(farmDataFixture, `{"Iron Deposit",1,125`, `{7,1,125`, 1), historyFixture},
		"History column dropped":        {farmDataFixture, strings.Replace(historyFixture, `{"Linen Cloth",2589,20768,21,21}`, `{"Linen Cloth",2589,20768,21}`, 1)},
		"History name and id swapped":   {farmDataFixture, strings.Replace(historyFixture, `{"Linen Cloth",2589,`, `{2589,"Linen Cloth",`, 1)},
	} {
		ref := loadFarmRef(farmGame(t, c[0], c[1]), nil)
		if len(ref.Items)+len(ref.Nodes)+len(ref.Maps) != 0 {
			t.Fatalf("%s: no bundled names at all, got %d items %d nodes %d maps", name, len(ref.Items), len(ref.Nodes), len(ref.Maps))
		}
	}
	// node rows only need their name: extra or fewer node columns are fine
	short := strings.Replace(farmDataFixture, `{"Iron Deposit",1,125,1,1,{2772,2}}`, `{"Iron Deposit"}`, 1)
	if ref := loadFarmRef(farmGame(t, short, historyFixture), nil); ref.Nodes[185701] != "Iron Deposit" || len(ref.Items) == 0 {
		t.Fatal("node rows with only a name are read")
	}
	// a missing History.lua is not a shape failure
	if ref := loadFarmRef(farmGame(t, farmDataFixture, ""), nil); len(ref.Items) == 0 {
		t.Fatal("FarmData alone still names items")
	}
}
