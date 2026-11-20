package score_test

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestMatchPrep(t *testing.T) {
	names := []string{"Alia-Stone-PvP-EU", "alia-stone-pvp-eu", "Bram-PvP-EU", "Äda-PvP-EU"}
	reports := []model.PrepReport{
		mt.Prep(names[0], "MAGE", 60, 1, "test").Clock(9, 10).Build(),
		mt.Prep("bRAM-pvp-eu", "WARRIOR", 60, 1, "test").Build(),
		mt.Prep("Nobody-PvP-EU", "MAGE", 60, 1, "test").Build(),
		mt.Prep("äda-PvP-EU", "MAGE", 60, 1, "test").Build(),
		mt.Prep("Bram", "MAGE", 60, 1, "test").Build(),
	}
	before := append([]model.PrepReport(nil), reports...)
	got, unmatched := score.MatchPrep(reports, names)
	if !reflect.DeepEqual(got[names[0]], reports[:1]) || len(got[names[1]]) != 0 || !reflect.DeepEqual(got[names[2]], reports[1:2]) {
		t.Fatalf("matches=%v", got)
	}
	if !reflect.DeepEqual(unmatched, reports[2:]) {
		t.Fatalf("unmatched=%v", unmatched)
	}
	if !reflect.DeepEqual(before, reports) {
		t.Fatal("mutated reports")
	}
	// A case-fold collision without an exact spelling cannot identify a player.
	ambiguous := mt.Prep("ALIA-STONE-PVP-EU", "", 60, 1, "").Build()
	got, unmatched = score.MatchPrep([]model.PrepReport{ambiguous}, names)
	if len(got) != 0 || len(unmatched) != 1 {
		t.Fatal("ambiguous folded match scores a player")
	}
}
