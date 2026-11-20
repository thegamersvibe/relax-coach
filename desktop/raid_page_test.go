package main

import (
	"fmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"os"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func raidPageFixture() fights.Run {
	f := raidPrepFixture(1800000000000, 1800000060000, 0)
	f.Name = "Training Colossus"
	for _, name := range []string{"Bram-Oak-Hill-PvP-EU", "Cato-PvP-EU", "Dara-PvP-EU", "Elen-PvP-EU"} {
		a := mt.Player(name)
		f.Players = append(f.Players, a)
		f.Roles[a.ID] = "dps"
		f.Events = append(f.Events, model.Event{TimeMs: f.StartMs, Kind: model.CastSuccess, Src: a, SpellID: 123, SpellName: "Training Bolt"})
		f.Events = append(f.Events, model.Event{TimeMs: f.StartMs, Kind: model.Damage, Src: a, Dst: mt.Creature("Training Colossus", 42), Amount: model.Some(100)})
	}
	return fights.Run{Kind: model.RunRaid, InstanceID: 99, StartMs: f.StartMs, EndMs: f.EndMs, Logger: f.Players[0], Roster: f.Players, Fights: []model.Fight{f}}
}
func TestRaidPageDiagnostics(t *testing.T) {
	r := raidPageFixture()
	r.Partial = true
	parsed := logfmt.ParseReport{LinesRead: 9, Parsed: 7, HeadersSeen: 1, Skipped: map[logfmt.SkipReason]int64{logfmt.UnknownWidth: 1}}
	p, err := buildRaidPage(r, parsed, []byte(`RelaxRaidDB={version=1,prep={version=1}}`), raidPrepConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Parse report", "Lines read: 9", "Parsed events: 7", "Headers: 1", "unknown field count: 1", "Prep diagnostics"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
}
func TestRaidPageEvidence(t *testing.T) {
	r := raidPageFixture()
	c := raidPrepConfig(t)
	r.Roster[0].Name = "<img src=x onerror=alert(1)>"
	save := raidPrepStore([]int64{1800000000}, []string{"Bram-Oak-Hill-PvP-EU"})
	p, err := buildRaidPage(r, logfmt.ParseReport{}, save, c)
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-raid-player="1"`, `id="raid-player-1"`, "Part", "Weight", "Against", "Evidence", "output", "activity", "mechanics", "utility", "preparation", "not measured", "35%", "25%", "boss_damage", "alive_ms", "measured.used", "prep_stamped_s", "prep_sender_clock_s", "fight_end_utc_s", "1800000000", "Training Colossus", "uncalibratedSpec"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing evidence %q", want)
		}
	}
	if strings.Contains(html, "<img src=x") || !strings.Contains(html, "&lt;img") {
		t.Fatal("unsafe name")
	}
	// Engine already rounded; the page must preserve every fight/part tenth.
	scores := score.Score(r.Fights, nil, []model.PrepReport{}, c)
	for _, n := range scores {
		for _, f := range n.Fights {
			for _, part := range f.Parts {
				if part.Tenths.Present {
					want := fmt.Sprintf("%d.%d", part.Tenths.V/10, part.Tenths.V%10)
					if !strings.Contains(html, want) {
						t.Fatalf("lost engine tenth %s", want)
					}
				}
			}
		}
	}
	if out := os.Getenv("NATIVE_HTML_OUT"); out != "" {
		if err := os.WriteFile(out+".raid-body", []byte(html), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestRaidPageHeaderAndLists(t *testing.T) {
	r := raidPageFixture()
	r.Roster = append(r.Roster, mt.Player("Faye-PvP-EU"))
	c := raidPrepConfig(t)
	page, err := buildRaidPage(r, logfmt.ParseReport{}, nil, c)
	if err != nil {
		t.Fatal(err)
	}
	if page.Bosses != 1 || page.Trash != 0 || len(page.Ranked) != 4 || len(page.Provisional) != 1 || len(page.Unscored) != 1 {
		t.Fatalf("header/list counts: %+v", page)
	}
	html, err := renderRaidBody(page)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Log: complete", "Ranked (4)", "Provisional (1)", "Not enough data (1)", "Scoring " + c.Scoring.Version, "Unlisted instance 99", "same role, same fight", "17% measured", "0% measured"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	for _, row := range page.Ranked {
		if row.Rank == "" {
			t.Fatal("ranked row lost engine rank")
		}
	}
	for _, row := range page.Provisional {
		if row.Rank != "" || row.Score == "" {
			t.Fatal("provisional lost score or got rank")
		}
	}
	for _, row := range page.Unscored {
		if row.Rank != "" || row.Score != "not enough data" {
			t.Fatal("unscored invented a score")
		}
	}
	r.Kind = model.RunDungeon
	page, err = buildRaidPage(r, logfmt.ParseReport{}, nil, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range [][]raidPlayerView{page.Ranked, page.Provisional, page.Unscored} {
		for _, row := range list {
			if row.Rank != "" {
				t.Fatal("dungeon ranked")
			}
		}
	}
	html, err = renderRaidBody(page)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Dungeon run: no ranks") {
		t.Fatal("dungeon no-rank explanation missing")
	}
}
