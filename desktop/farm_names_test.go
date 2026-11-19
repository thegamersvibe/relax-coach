package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Invented names and minimal literal-only shapes; no private addon data.
func nameGame(t *testing.T, save, coach, gear string) (*App, string) {
	t.Helper()
	accountConfigEnv(t)
	game := farmGame(t, "", "")
	path := accountSave(t, game, "A#1", "Alpha")
	body := `ForeverCoachDB={schema=1, itemNames={` + coach + `}, chars={}}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	day := ahpScanDay(time.Now())
	hist := ahpHistEncode(series(day, 20, 1000, 1000, 1000, 1000, 1000, 1000, 1500))
	farmWrite(t, game, "A#1", fmt.Sprintf(`AHProfitDB={v=3,items={%s},hist={ ["Realm|Alliance"]={["900001"]=%q}}, live={ ["Realm|Alliance"]={p={},q={},t={},full=%d}}, bank={ ["Alpha-Realm"]={t=%d,items={{900001,2}}}}}`, save, hist, time.Now().Unix(), time.Now().Unix()))
	if gear != "" {
		nameAddonFile(t, game, "GearQuestForever", "_generated/Data.ForeverAudit.generated.lua", gear)
	}
	a, _ := accountApp(t, game)
	return a, game
}
func nameAddonFile(t *testing.T, game, addon, file, body string) string {
	t.Helper()
	p := filepath.Join(game, "Interface", "AddOns", addon, file)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}
func assertFarmName(t *testing.T, a *App, want string) string {
	t.Helper()
	fv := a.snapshot().Farm
	if len(fv.Hot.Houses) != 1 || len(fv.Hot.Houses[0].Rows) != 1 || len(fv.Sell) != 1 {
		t.Fatalf("missing hot/sell rows: %+v", fv)
	}
	if fv.Hot.Houses[0].Rows[0].Name != want || fv.Sell[0].Name != want {
		t.Fatalf("hot=%q sell=%q want=%q", fv.Hot.Houses[0].Rows[0].Name, fv.Sell[0].Name, want)
	}
	html, err := renderFarmBody(View{Farm: fv})
	if err != nil {
		t.Fatal(err)
	}
	return html
}
func TestFarmNameCoachTOCUpdaterContract(t *testing.T) {
	b, err := os.ReadFile("../addon/ForeverCoach/ForeverCoach.toc")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAddonTOC(b, Version); err != nil {
		t.Fatalf("Farming cache must remain installable by the existing updater: %v", err)
	}
}

func TestFarmNameSourcesInOrder(t *testing.T) {
	for _, tt := range []struct{ name, save, bundle, coach, gear, want string }{
		{"save", `[900001]={"Save label",17}`, `{"Bundled label",900001,1,2,3,1}`, `[900001]={n="Coach label",q=4}`, `[900001]={name="Gear label",quality=3}`, "Save label"},
		{"bundle", "", `{"Bundled label",900001,1,2,3,1}`, `[900001]={n="Coach label",q=4}`, `[900001]={name="Gear label",quality=3}`, "Bundled label"},
		{"coach", "", "", `[900001]={n="Coach label",q=4}`, `[900001]={name="Gear label",quality=3}`, "Coach label"},
		{"gear", "", "", "", `[900001]={name="Gear label",quality=3}`, "Gear label"},
		{"none", "", "", "", "", "item 900001"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, game := nameGame(t, tt.save, tt.coach, "GQ.Data.foreverAudit = {"+tt.gear+"}")
			if tt.bundle != "" {
				nameAddonFile(t, game, "AHProfit", "FarmData.lua", "ns.farmData = {1,{"+tt.bundle+"},{},{}}")
			}
			a.refresh()
			html := assertFarmName(t, a, tt.want)
			if tt.name == "save" || tt.name == "bundle" || tt.name == "none" {
				if strings.Contains(html, `class="q`) {
					t.Fatal("quality must not be guessed from a lower-priority source")
				}
			}
		})
	}
}
func TestFarmNameQualityClasses(t *testing.T) {
	for _, tt := range []struct{ q, class string }{{"0", "q0"}, {"1", "q1"}, {"2", "q2"}, {"3", "q3"}, {"4", "q4"}, {"5", "q5"}, {"6", "q6"}, {"7", "q7"}, {"8", ""}, {"nil", ""}, {"-1", ""}, {"9", ""}, {"2.5", ""}, {`"3"`, ""}} {
		t.Run(tt.q, func(t *testing.T) {
			a, _ := nameGame(t, "", `[900001]={n="Synthetic <blade>",q=`+tt.q+`}`, "")
			html := assertFarmName(t, a, "Synthetic <blade>")
			want := `<span class="` + tt.class + `">Synthetic &lt;blade&gt;</span>`
			if strings.Count(html, want) != 2 {
				t.Fatalf("hot and sell must use validated class %q; HTML lacks %s", tt.class, want)
			}
			if strings.Contains(html, "Synthetic <blade>") {
				t.Fatal("save text was not escaped")
			}
		})
	}
	css, err := assets.ReadFile("native_shell.html")
	if err != nil {
		t.Fatal(err)
	}
	for q, c := range []string{"#9d9d9d", "#ffffff", "#1eff00", "#0070dd", "#a335ee", "#ff8000", "#e6cc80", "#e6cc80"} {
		if !strings.Contains(string(css), fmt.Sprintf(".q%d{color:%s}", q, c)) {
			t.Errorf("missing q%d colour %s", q, c)
		}
	}
}
func TestFarmGearNamesDataOnlyClassesAndBounds(t *testing.T) {
	a, game := nameGame(t, "", "", `local GQ = GearQuest
GQ.Data.foreverAudit = {[900001]={name="Gear <label>",quality=3}}
error("must never execute addon code")`)
	html := assertFarmName(t, a, "Gear <label>")
	if strings.Count(html, `<span class="q3">Gear &lt;label&gt;</span>`) != 2 {
		t.Fatal("GearQuest quality missing on hot/sell")
	}
	file := filepath.Join(game, "Interface", "AddOns", "GearQuestForever", "_generated", "Data.ForeverAudit.generated.lua")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, class := range gearClasses {
		folder := "GearQuestForever_" + class
		table := strings.ToLower(class) + "ItemFacts"
		if class == "PALADIN" {
			table = "itemFacts"
		}
		nameAddonFile(t, game, folder, "Data.Facts.generated.lua", "local GQ = GearQuest\nGQ.Data."+table+` = {[900002]={name="Class label"}}`)
		if got := loadGearNames(game, nil)[900002].Name; got != "Class label" {
			t.Fatalf("class %s not read: %q", class, got)
		}
		if err := os.RemoveAll(filepath.Join(game, "Interface", "AddOns", folder)); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(file)
	if err != nil || string(after) != string(before) {
		t.Fatal("read changed GearQuest bytes")
	}
	for _, body := range []string{`GQ.Data.foreverAudit = {`, `GQ.Data.foreverAudit = os.execute("no")`, `GQ.Data.foreverAudit = {[900001]={name=12}}`, `GQ.Data.foreverAudit = {[900001]={name="bad\nlabel"}}`, `GQ.Data.foreverAudit = {[900001]={name="|Hitem:3|h[x]|h"}}`} {
		nameAddonFile(t, game, "GearQuestForever", "_generated/Data.ForeverAudit.generated.lua", body)
		a.refresh()
		assertFarmName(t, a, "item 900001")
	}
	nameAddonFile(t, game, "GearQuestForever", "_generated/Data.ForeverAudit.generated.lua", string(before))
	t.Run("byte cap", func(t *testing.T) {
		old := gearNameMaxBytes
		t.Cleanup(func() { gearNameMaxBytes = old })
		gearNameMaxBytes = int64(len(before))
		if len(loadGearNames(game, nil)) != 1 {
			t.Fatal("exact byte cap rejected")
		}
		gearNameMaxBytes--
		if len(loadGearNames(game, nil)) != 0 {
			t.Fatal("oversize file admitted")
		}
	})
	t.Run("value cap", func(t *testing.T) {
		old := gearNameMaxValues
		t.Cleanup(func() { gearNameMaxValues = old })
		gearNameMaxValues = 2
		if len(loadGearNames(game, nil)) != 0 {
			t.Fatal("parser value cap ignored")
		}
	})
	t.Run("entry cap", func(t *testing.T) {
		old := gearNameMaxItems
		t.Cleanup(func() { gearNameMaxItems = old })
		gearNameMaxItems = 1
		nameAddonFile(t, game, "GearQuestForever_WARRIOR", "Data.Facts.generated.lua", `GearQuest.Data.warriorItemFacts = {[900002]={name="Another label",quality=2}}`)
		defer os.RemoveAll(filepath.Join(game, "Interface", "AddOns", "GearQuestForever_WARRIOR"))
		if len(loadGearNames(game, nil)) != 0 {
			t.Fatal("aggregate entry cap ignored")
		}
	})
	t.Run("total byte cap", func(t *testing.T) {
		old := gearNameMaxTotalBytes
		t.Cleanup(func() { gearNameMaxTotalBytes = old })
		gearNameMaxTotalBytes = int64(len(before)) - 1
		if len(loadGearNames(game, nil)) != 0 {
			t.Fatal("total bytes cap ignored")
		}
	})
	t.Run("file cap", func(t *testing.T) {
		old := gearNameMaxFiles
		t.Cleanup(func() { gearNameMaxFiles = old })
		gearNameMaxFiles = 1
		nameAddonFile(t, game, "GearQuestForever_WARRIOR", "Data.Facts.generated.lua", `GearQuest.Data.warriorItemFacts = {[900002]={name="Another label"}}`)
		defer os.RemoveAll(filepath.Join(game, "Interface", "AddOns", "GearQuestForever_WARRIOR"))
		if len(loadGearNames(game, nil)) != 0 {
			t.Fatal("file count cap ignored")
		}
	})
}

func TestFarmGearNamesAssignmentBudget(t *testing.T) {
	a, _ := nameGame(t, "", "", strings.Repeat(`GQ.Data.foreverAudit = {[900001]={name="Repeated label"}}`+"\n", 17))
	assertFarmName(t, a, "item 900001")
}

func TestFarmGearNamesSymlinksAndCacheInvalidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges")
	}
	a, game := nameGame(t, "", "", `GQ.Data.foreverAudit = {[900001]={name="First label"}}`)
	assertFarmName(t, a, "First label")
	p := filepath.Join(game, "Interface", "AddOns", "GearQuestForever", "_generated", "Data.ForeverAudit.generated.lua")
	if err := os.WriteFile(p, []byte(`GQ.Data.foreverAudit = {[900001]={name="Changed label",quality=4}}`), 0644); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	assertFarmName(t, a, "Changed label")
	if err := os.Rename(p, p+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("Data.ForeverAudit.generated.lua.real", p); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	assertFarmName(t, a, "item 900001")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(p+".real", p); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(p)
	if err := os.Rename(dir, dir+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("_generated.real", dir); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	assertFarmName(t, a, "item 900001")
}

func TestFarmNameCoachValidationAndCap(t *testing.T) {
	for _, tt := range []struct{ row, want string }{
		{`[900001]={n="",q=3}`, "Gear label"},
		{`[900001]={n="bad\nlabel",q=3}`, "Gear label"},
		{`[900001]={n="|Hitem:3|h[x]|h",q=3}`, "Gear label"},
		{`[900001]={n="` + strings.Repeat("x", 129) + `",q=3}`, "Gear label"},
		{`["0900001"]={n="Malformed ID"}`, "Gear label"},
		{`[900001]={n="` + strings.Repeat("x", 128) + `"}`, strings.Repeat("x", 128)},
	} {
		a, _ := nameGame(t, "", tt.row, `GQ.Data.foreverAudit = {[900001]={name="Gear label"}}`)
		html := assertFarmName(t, a, tt.want)
		if strings.Contains(html, `class="q`) {
			t.Fatal("unknown quality should use default text")
		}
	}
	rows := map[string]any{}
	for i := 1; i <= coachNameMaxItems; i++ {
		rows[strconv.Itoa(i)] = map[string]any{"n": "Synthetic"}
	}
	if len(coachItemNames(map[string]any{"itemNames": rows})) != coachNameMaxItems {
		t.Fatal("exact cap rejected")
	}
	rows["10001"] = map[string]any{"n": "Extra"}
	if len(coachItemNames(map[string]any{"itemNames": rows})) != 0 {
		t.Fatal("oversize Coach cache accepted")
	}
}

func TestFarmNameCoachAccountIsolationAndLastGood(t *testing.T) {
	a, game := nameGame(t, "", `[900001]={n="Account A"}`, "")
	assertFarmName(t, a, "Account A")
	p := filepath.Join(game, "WTF", "Account", "A#1", "SavedVariables", "ForeverCoach.lua")
	if err := os.WriteFile(p, []byte("ForeverCoachDB = {"), 0644); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	assertFarmName(t, a, "Account A")
	accountSave(t, game, "B#2", "Beta")
	ah, err := os.ReadFile(filepath.Join(game, "WTF", "Account", "A#1", "SavedVariables", "AHProfit.lua"))
	if err != nil {
		t.Fatal(err)
	}
	farmWrite(t, game, "B#2", string(ah))
	a.refresh()
	bridge := newNativeBridge(a, true, nativeServices{})
	t.Cleanup(bridge.Close)
	if _, err := bridge.Call("selectAccount", nativeArgs("B#2")); err != nil {
		t.Fatal(err)
	}
	assertFarmName(t, a, "item 900001")
	if _, err := bridge.Call("selectAccount", nativeArgs("A#1")); err != nil {
		t.Fatal(err)
	}
	// A is now corrupt; B's report must not resurrect A's former cache.
	assertFarmName(t, a, "item 900001")
	a.setGame(game)
	a.refresh()
	for _, r := range a.snapshot().Farm.Sell {
		if r.Name == "Account A" {
			t.Fatal("name leaked across folder reset")
		}
	}
}
