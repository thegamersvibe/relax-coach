package main

import (
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestRaidPageReportedPreparation(t *testing.T) {
	r := raidPageFixture()
	save := raidPrepStore([]int64{1800000000}, []string{"Bram-Oak-Hill-PvP-EU"})
	p, err := buildRaidPage(r, logfmt.ParseReport{}, save, raidPrepConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Reported stock by category", "Better item in bags", "maximum reported before this fight"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing report display %q", want)
		}
	}
	for _, bad := range []string{"new at 60", "no flask", "flask used"} {
		if strings.Contains(strings.ToLower(html), bad) {
			t.Fatalf("invented claim %q", bad)
		}
	}
	for _, list := range [][]raidPlayerView{p.Ranked, p.Provisional, p.Unscored} {
		for _, player := range list {
			for _, f := range player.Fights {
				for _, part := range f.Parts {
					if part.Name == "deaths" {
						t.Fatal("invented deaths part")
					}
					if part.Name == "mechanics" {
						found := false
						for _, m := range part.Measures {
							found = found || m.Name == "deaths"
						}
						if !found {
							t.Fatal("missing deaths measure")
						}
					}
				}
			}
		}
	}
	// No report means no claimed bag count, even if an unrelated count is zero.
	parts, _ := raidPlayerEvidence(model.NightScore{Fights: []model.FightScore{{FightID: r.Fights[0].ID, Parts: []model.Part{{Name: "preparation"}}}}}, r.Fights, raidPrepConfig(t))
	if len(parts[0].Parts[0].Preparation) != 0 {
		t.Fatal("unreadable/absent report inferred stock")
	}
}
