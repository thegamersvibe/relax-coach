package compare

import (
	"fmt"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func config(t *testing.T) cfg.Config {
	t.Helper()
	c, err := cfg.Load("../../../../rules/raid")
	if err != nil {
		t.Fatal(err)
	}
	return c.Snapshot()
}

func fixture(n int) ([]model.NightScore, []model.Actor, []model.Fight) {
	names := []string{"Alia-Stone-PvP-EU", "Bram-Oak-Hill-PvP-EU", "Cato-PvP-EU", "Dara-PvP-EU", "Edda-PvP-EU", "Fenn-PvP-EU", "Gala-PvP-EU", "Hale-PvP-EU", "Iona-PvP-EU", "Jori-PvP-EU", "Kora-PvP-EU", "Leto-PvP-EU"}
	var scores []model.NightScore
	var roster []model.Actor
	for i := 0; i < n; i++ {
		a := mt.Player(names[i])
		// Deliberately not names: only the roster translates IDs to logged names.
		a.ID = fmt.Sprintf("player-%d", i)
		roster = append(roster, a)
		scores = append(scores, model.NightScore{PlayerID: a.ID, Tenths: model.Some(int64(900 - i*10)), Rank: model.Some(int64(i + 1)), Coverage: 74,
			Fights: []model.FightScore{{FightID: "boss", PlayerID: a.ID, Role: "dps", Tenths: model.Some(700), Parts: []model.Part{{Name: "output", Tenths: model.Some(700)}}}},
		})
	}
	return scores, roster, []model.Fight{{ID: "boss", Boss: true, Success: model.Some(1)}}
}

func officer(top, bottom string) OfficerFile {
	return OfficerFile{Version: 1, Top: []string{top}, Bottom: []string{bottom}}
}

func TestCompareRanksAndNames(t *testing.T) {
	ns, roster, fights := fixture(6)
	ns[5].Rank = model.Opt{}
	ns[5].Coverage = 27
	c := config(t)
	f := OfficerFile{Version: 1, Officer: "prediction", Top: []string{"dara-pvp-eu", roster[2].Name, "Cato", roster[5].Name}, Bottom: []string{roster[0].Name, roster[4].Name}}
	got, err := Compare(ns, roster, c, f, fights)
	if err != nil || got.Ranked != 5 || got.Officer != f.Officer || len(got.Top) != 4 || len(got.Bottom) != 2 {
		t.Fatalf("rows: %+v %v", got, err)
	}
	if got.Top[0].Verdict != "disagreement" || got.Top[0].LoggedName != roster[3].Name || got.Top[0].PlayerID != roster[3].ID || got.Top[0].Rank != model.Some(4) || got.Top[0].Name != f.Top[0] {
		t.Fatal("lower-half top or roster matching")
	}
	if got.Top[1].Verdict != "agreement" || got.Top[2].Status != "not in this night" || got.Top[2].Verdict != "" {
		t.Fatal("odd upper-half boundary or full-name-only matching")
	}
	if got.Top[3].Status != "no rank: 27% measured" || got.Top[3].Verdict != "" || got.Top[3].Rank.Present {
		t.Fatal("provisional verdict")
	}
	if got.Bottom[0].Verdict != "disagreement" || got.Bottom[1].Verdict != "agreement" {
		t.Fatal("bottom half reversed")
	}
	// Rank, not input order or coverage, determines halves; tied ranks stay tied.
	ns[0].Rank = model.Some(3)
	ns[0].Coverage = 1
	ns[5].Tenths = model.Opt{}
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Bottom[0].Verdict != "disagreement" || got.Top[3].Status != "no rank: 27% measured" {
		t.Fatal("rank recomputed or unscored name changed")
	}
	// Even boundary: rank 3 is lower half among four ranked players.
	ns[4].Rank = model.Opt{}
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Ranked != 4 || got.Top[1].Verdict != "disagreement" || got.Bottom[0].Verdict != "agreement" {
		t.Fatal("even half boundary")
	}
}

func TestCompareOwnFives(t *testing.T) {
	ns, roster, fights := fixture(12)
	// Scramble source order to expose accidental re-ranking or reliance on order.
	ns[0], ns[11] = ns[11], ns[0]
	f := officer("alia-stone-pvp-eu", roster[11].Name)
	got, err := Compare(ns, roster, config(t), f, fights)
	if err != nil || len(got.ScoreTop) != 4 || len(got.ScoreBottom) != 4 {
		t.Fatalf("own five should be filtered, not backfilled: %+v %v", got, err)
	}
	for i, e := range got.ScoreTop {
		if e.PlayerID != roster[i+1].ID || e.Verdict != "" {
			t.Fatal("score top order/verdict/exclusion")
		}
	}
	for i, e := range got.ScoreBottom {
		if e.PlayerID != roster[10-i].ID || e.Verdict != "" {
			t.Fatal("score bottom order/verdict/exclusion")
		}
	}
	f = officer(roster[11].Name, "alia-stone-pvp-eu")
	got, err = Compare(ns, roster, config(t), f, fights)
	if err != nil || len(got.ScoreTop) != 4 || len(got.ScoreBottom) != 4 {
		t.Fatal("names in either list must be excluded from both own fives")
	}
	for i := range ns {
		if ns[i].PlayerID == roster[1].ID || ns[i].PlayerID == roster[10].ID {
			ns[i].Rank = model.Opt{}
		}
	}
	got, err = Compare(ns, roster, config(t), f, fights)
	if err != nil {
		t.Fatal(err)
	}
	for _, rows := range [][]Entry{got.ScoreTop, got.ScoreBottom} {
		for _, e := range rows {
			if !e.Rank.Present || e.PlayerID == roster[1].ID || e.PlayerID == roster[10].ID {
				t.Fatal("own five included an unranked player")
			}
		}
	}
}

func TestCompareMinimumAndInvalid(t *testing.T) {
	ns, roster, fights := fixture(4)
	f := officer(roster[0].Name, roster[3].Name)
	c := config(t)
	for _, n := range []int{0, 1, 2, 3, 4} {
		got, err := Compare(ns[:n], roster, c, f, fights)
		if err != nil || got.Ranked != n {
			t.Fatalf("threshold: %+v %v", got, err)
		}
		if n < 4 && (got.NoReportReason != "fewer than 4 ranked players" || len(got.Top)+len(got.Bottom)+len(got.ScoreTop)+len(got.ScoreBottom) != 0) {
			t.Fatal("small night produced a report")
		}
		if n == 4 && got.NoReportReason != "" {
			t.Fatal("four ranked players suppressed")
		}
	}
	got, err := Compare(ns, roster, c, OfficerFile{}, fights)
	if err == nil || got.Ranked != 0 || len(got.Top)+len(got.Bottom) != 0 {
		t.Fatal("invalid file compared")
	}
}
