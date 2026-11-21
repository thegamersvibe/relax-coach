package compare

import (
	"bytes"
	"encoding/json"
	"math/big"
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestCompareMatchingEdges(t *testing.T) {
	ns, roster, fights := fixture(4)
	c := config(t)
	roster = append(roster, model.Actor{ID: "case-peer", Name: "alia-stone-pvp-eu"}, model.Actor{ID: "absent-score", Name: "Mira-PvP-EU"}, model.Actor{ID: "unicode", Name: "Élan-PvP-EU"}, roster[0])
	f := OfficerFile{Version: 1, Top: []string{roster[0].Name, "Mira-PvP-EU", "élan-PvP-EU", " Cato-PvP-EU", "player-2"}, Bottom: []string{roster[3].Name}}
	got, err := Compare(ns, roster, c, f, fights)
	if err != nil || len(got.Top) != 5 || got.Top[0].PlayerID != ns[0].PlayerID || got.Top[0].Verdict != "agreement" {
		t.Fatal("exact spelling lost to folded collision or repeated roster entry")
	}
	if got.Top[1].Status != "no rank: 0% measured" || got.Top[1].Verdict != "" {
		t.Fatal("roster-only player is not missing from night")
	}
	for _, e := range got.Top[2:] {
		if e.Status != "not in this night" || e.Verdict != "" {
			t.Fatal("Unicode fold, trim or ID treated as a name")
		}
	}
	f.Top = []string{"ALIA-STONE-PVP-EU"}
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Status != "not in this night" {
		t.Fatal("ambiguous folded name matched")
	}
	roster = append(roster, model.Actor{ID: "duplicate-name", Name: roster[0].Name})
	f.Top = []string{roster[0].Name}
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Status != "not in this night" {
		t.Fatal("ambiguous roster identity compared")
	}
}

func TestCompareAllRoleWeights(t *testing.T) {
	for _, role := range []string{"dps", "healer", "tank"} {
		for name, weight := range map[string]string{"output": "1029/100", "activity": "2071/100", "mechanics": "30", "utility": "24", "preparation": "15"} {
			t.Run(role+"/"+name, func(t *testing.T) {
				ns, roster, fights := fixture(4)
				c := config(t)
				w := cfg.PartWeights{Output: 10.29, Activity: 20.71, Mechanics: 30, Utility: 24, Preparation: 15}
				switch role {
				case "dps":
					c.Scoring.Weights.DPS = w
				case "healer":
					c.Scoring.Weights.Healer = w
				case "tank":
					c.Scoring.Weights.Tank = w
				}
				for i, v := range []int64{800, 600, 601, 400} {
					ns[i].Fights[0].Role = role
					ns[i].Fights[0].Parts = []model.Part{{Name: name, Tenths: model.Some(v)}}
				}
				got, err := Compare(ns, roster, c, officer(roster[3].Name, roster[0].Name), fights)
				want, _ := new(big.Rat).SetString(weight)
				if err != nil || got.Top[0].Part == nil || got.Top[0].Part.Weight.Cmp(want) != 0 || got.Top[0].Part.Role != role {
					t.Fatal("role weight or authored decimal not preserved")
				}
				want.Mul(want, big.NewRat(-401, 20))
				if got.Top[0].Part.Impact.Cmp(want) != 0 {
					t.Fatal("impact is not exact weighted delta")
				}
			})
		}
	}
}

func TestCompareOriginalFightWeighting(t *testing.T) {
	ns, roster, fights := fixture(4)
	c := config(t)
	c.Scoring.Night.KillWeight = 2
	c.Scoring.Night.WipeWeight = .5
	// Both selected roles have a negative delta, so this isolates fight weighting.
	for i, v := range []int64{800, 600, 1000, 400} {
		parts(&ns[i], v, 0)
		ns[i].Fights[0].Parts = ns[i].Fights[0].Parts[:1]
	}
	ns[2].Fights[0].Role = "tank"
	ns[3].Fights = append(ns[3].Fights, model.FightScore{FightID: "trash", Role: "tank", Tenths: model.Some(900), Parts: []model.Part{{Name: "output", Tenths: model.Some(900)}}})
	fights = append(fights, model.Fight{ID: "trash"})
	f := officer(roster[3].Name, roster[0].Name)
	got, err := Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part == nil || got.Top[0].Part.Role != "dps" || got.Top[0].Part.Median.Cmp(big.NewRat(60, 1)) != 0 {
		t.Fatal("kill weight did not select DPS night role")
	}
	fights[0].Success = model.Some(0)
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part == nil || got.Top[0].Part.Role != "tank" || got.Top[0].Part.Score.Cmp(big.NewRat(90, 1)) != 0 || got.Top[0].Part.Median.Cmp(big.NewRat(95, 1)) != 0 {
		t.Fatal("original wipe/trash weights did not reach night summary")
	}
}

func TestComparePurityAndOrder(t *testing.T) {
	ns, roster, fights := fixture(6)
	ns[5].Fights[0].Parts[0].Tenths = model.Some(400) // an explanatory negative delta
	c := config(t)
	f := officer(roster[5].Name, roster[0].Name)
	before, _ := json.Marshal([]any{ns, roster, c, f, fights})
	first, err := Compare(ns, roster, c, f, fights)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Compare(ns, roster, c, f, fights)
	after, _ := json.Marshal([]any{ns, roster, c, f, fights})
	if err != nil || !bytes.Equal(before, after) || !reflect.DeepEqual(first, again) {
		t.Fatal("comparison mutated input or is nondeterministic")
	}
	first.Top[0].Part.Score.SetInt64(0)
	again, err = Compare(ns, roster, c, f, fights)
	if err != nil || again.Top[0].Part.Score.Sign() == 0 {
		t.Fatal("returned rationals alias later results")
	}
	// Rank ties order by logged name, not iteration; both own-five directions.
	for i := range ns {
		ns[i].Rank = model.Some(1)
	}
	f = officer("Missing-PvP-EU", "Absent-PvP-EU")
	a, err := Compare(ns, roster, c, f, fights)
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(ns)-1; i < j; i, j = i+1, j-1 {
		ns[i], ns[j] = ns[j], ns[i]
	}
	b, err := Compare(ns, roster, c, f, fights)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("rank tie order depends on input order")
	}
	for _, rows := range [][]Entry{a.ScoreTop, a.ScoreBottom} {
		for i, e := range rows {
			if e.LoggedName != roster[i].Name || e.Verdict != "" || e.Part != nil || e.PartReason != "" {
				t.Fatal("tie ordering or verdictless score entries")
			}
		}
	}
}
