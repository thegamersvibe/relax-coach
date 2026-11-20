package prepread

import (
	"os"
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".lua")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRealNormalReport(t *testing.T) {
	got, report := Parse(fixture(t, "normal"), 1800000000, 1800001000)
	want := modeltest.Prep("Alia-Stone-PvP-EU", "MAGE", 60, 1, "0.2.0").
		Played(123).Reason("join").Clock(1800000000, 1800000000).
		Consumable("potion", 101, 3).Consumable("food", 102, 5).
		Consumable("flask", 103, 1).Consumable("elixir", 104, 2).
		Gear(model.PrepGear{Slot: 2, WornID: 200, BagID: 202}, model.PrepGear{Slot: 3, WornID: 0, BagID: 203}).NotScored(1).Build()
	if !reflect.DeepEqual(got, []model.PrepReport{want}) {
		t.Fatalf("reports = %#v, want %#v", got, want)
	}
	if len(report.Warnings) != 0 || len(report.Skipped) != 0 {
		t.Fatalf("unexpected diagnostic: %+v", report)
	}
}
