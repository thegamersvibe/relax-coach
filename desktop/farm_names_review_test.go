package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Round-one survivor controls. Fixtures are invented; budgets are independent
// so a later guard cannot mask a missing earlier one.
func TestFarmNameIDZero(t *testing.T) {
	for _, key := range []string{"0", "-1", "01", "1.5", "10000001"} {
		if _, ok := farmNameID(key); ok {
			t.Fatalf("invalid item ID accepted: %q", key)
		}
	}
	for _, key := range []string{"1", "10000000"} {
		if _, ok := farmNameID(key); !ok {
			t.Fatalf("valid boundary rejected: %q", key)
		}
	}
}

func TestFarmGearNamesPerTableBudget(t *testing.T) {
	old := gearNameMaxItems
	t.Cleanup(func() { gearNameMaxItems = old })
	gearNameMaxItems = 1
	game := t.TempDir()
	nameAddonFile(t, game, "GearQuestForever", "Data.Facts.generated.lua", `GQ.Data.foreverAudit = {
		[900001]={name="Synthetic valid"}, [900002]={name=false}}`)
	if len(loadGearNames(game, nil)) != 0 {
		t.Fatal("per-table budget counts invalid rows too, before retaining items")
	}
}

func TestFarmGearNamesFirstWinsAcrossTables(t *testing.T) {
	game := t.TempDir()
	nameAddonFile(t, game, "GearQuestForever", "Data.Facts.generated.lua", `GQ.Data.foreverAudit = {[900001]={name="First synthetic",quality=2}}
GQ.Data.itemFacts = {[900001]={name="Second synthetic",quality=4}}`)
	got := loadGearNames(game, nil)[900001]
	if got.Name != "First synthetic" || got.QualityClass != "q2" {
		t.Fatalf("first table must win deterministically: %+v", got)
	}
}

func TestFarmGearNamesFillFieldsIndependently(t *testing.T) {
	for _, tt := range []struct{ name, first, second, quality string }{
		{"quality then name", `{quality=2}`, `{name="Synthetic name",quality=4}`, "q2"},
		{"name then quality", `{name="Synthetic name"}`, `{quality=4}`, "q4"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game := t.TempDir()
			nameAddonFile(t, game, "GearQuestForever", "Data.Facts.generated.lua",
				"GQ.Data.foreverAudit = {[900001]="+tt.first+"}\nGQ.Data.itemFacts = {[900001]="+tt.second+"}")
			got := loadGearNames(game, nil)[900001]
			if got.Name != "Synthetic name" || got.QualityClass != tt.quality {
				t.Fatalf("first known field must win without hiding the other: %+v", got)
			}
		})
	}
}

func TestFarmGearNamesMtimeInvalidatesSameSize(t *testing.T) {
	game := t.TempDir()
	file := nameAddonFile(t, game, "GearQuestForever", "Data.Facts.generated.lua", `GQ.Data.foreverAudit = {[900001]={name="First synthetic",quality=2}}`)
	first := time.Unix(1700000000, 0)
	if err := os.Chtimes(file, first, first); err != nil {
		t.Fatal(err)
	}
	cache := gearNameCache{}
	if got := loadGearNames(game, &cache)[900001].Name; got != "First synthetic" {
		t.Fatal(got)
	}
	nameAddonFile(t, game, "GearQuestForever", "Data.Facts.generated.lua", `GQ.Data.foreverAudit = {[900001]={name="Later synthetic",quality=4}}`)
	later := first.Add(time.Second)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	got := loadGearNames(game, &cache)[900001]
	if got.Name != "Later synthetic" || got.QualityClass != "q4" {
		t.Fatalf("same bytes count but newer mtime must invalidate: %+v", got)
	}
}

func TestFarmGearNamesGameReset(t *testing.T) {
	a, game := nameGame(t, "", "", `GQ.Data.foreverAudit = {[900001]={name="First synthetic",quality=2}}`)
	assertFarmName(t, a, "First synthetic")
	file := nameAddonFile(t, game, "GearQuestForever", "_generated/Data.ForeverAudit.generated.lua", `GQ.Data.foreverAudit = {[900001]={name="First synthetic",quality=2}}`)
	a.refresh()
	st, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	nameAddonFile(t, game, "GearQuestForever", "_generated/Data.ForeverAudit.generated.lua", `GQ.Data.foreverAudit = {[900001]={name="Later synthetic",quality=4}}`)
	// Explicit folder reset must also discard a same-size, same-mtime cache.
	if err := os.Chtimes(file, st.ModTime(), st.ModTime()); err != nil {
		t.Fatal(err)
	}
	a.setGame(game)
	a.refresh()
	assertFarmName(t, a, "Later synthetic")
}

func TestFarmGearNamesDirectoryBudget(t *testing.T) {
	old := gearNameMaxFiles
	t.Cleanup(func() { gearNameMaxFiles = old })
	gearNameMaxFiles = 2
	game := t.TempDir()
	nameAddonFile(t, game, "GearQuestForever", "Data.Facts.generated.lua", `GQ.Data.foreverAudit = {[900001]={name="Synthetic label"}}`)
	nameAddonFile(t, game, "GearQuestForever", "unrelated-a.txt", "not generated")
	nameAddonFile(t, game, "GearQuestForever", "unrelated-b.txt", "not generated")
	if len(loadGearNames(game, nil)) != 0 {
		t.Fatal("directory enumeration budget includes non-generated files")
	}
}

func TestFarmGearNamesOversizeFileDoesNotSpendTotalBudget(t *testing.T) {
	oldBytes, oldTotal := gearNameMaxBytes, gearNameMaxTotalBytes
	t.Cleanup(func() { gearNameMaxBytes, gearNameMaxTotalBytes = oldBytes, oldTotal })
	body := `GQ.Data.foreverAudit = {[900001]={name="Synthetic label"}}`
	gearNameMaxBytes, gearNameMaxTotalBytes = int64(len(body)), int64(len(body))
	game := t.TempDir()
	// Sorted first; one byte over must be skipped before aggregate charging.
	nameAddonFile(t, game, "GearQuestForever", "a.generated.lua", strings.Repeat(" ", len(body)+1))
	nameAddonFile(t, game, "GearQuestForever", "b.generated.lua", body)
	if got := loadGearNames(game, nil)[900001].Name; got != "Synthetic label" {
		t.Fatalf("oversize file consumed another file's byte budget: %q", got)
	}
}

func TestHotSupplyIsScanDayPeakNotLive(t *testing.T) {
	hist := map[int][]ahpHistRow{
		1: series(hotToday, 100, 1000, 1000, 1000, 1000, 1000, 1000, 1500),
		2: series(hotToday, 100, 1000, 1000, 1000, 1000, 1000, 1000, 500),
	}
	db := hotSave(hist)
	M(M(db["live"])[hotHouses[0].Key])["q"] = map[string]any{"1": float64(2), "2": float64(2)}
	h := buildHot(db, 3, true, hotHouses, nil, nil, hotNow).Houses[0]
	if len(h.Rows) != 1 || len(h.Fallers) != 1 || h.Rows[0].Listed != 100 || h.Fallers[0].Listed != 100 {
		t.Fatalf("low live supply must not replace the house-day peak: %+v", h)
	}
}
