package main

import (
	"os"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestRaidPageDeathLabels(t *testing.T) {
	r := raidPageFixture()
	f := r.Fights[0]
	p := f.Players[1]
	boss := f.Events[0].Dst
	hit := model.Event{TimeMs: f.StartMs + 50000, Kind: model.Damage, Stream: model.SwingLanded, Src: boss, Dst: p, Amount: model.Some(100)}
	end := model.Event{TimeMs: hit.TimeMs, Kind: model.UnitDied, DiedFlag: model.Some(0), Dst: p}
	f.Events = append(f.Events, hit, end)
	d := model.Death{TimeMs: end.TimeMs, Player: p, Cause: model.CauseUnresolved, OwnerKind: model.OwnerUndecided}
	for _, tc := range []struct {
		name, role, cause, suggestion string
		spell                         bool
		want                          string
	}{
		{"ambiguous", "dps", model.CauseUnresolved, "", false, "aggro: puller or tank"},
		{"tank", "tank", model.CauseUnresolved, "", false, "unknown"},
		{"suggested", "dps", model.CauseUnresolved, model.CauseUnhealed, false, "unknown"},
		{"spell", "dps", model.CauseUnresolved, "", true, "unknown"},
		{"known", "dps", model.CauseAvoidable, "", false, model.CauseAvoidable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := f
			x.Roles = map[string]string{p.ID: tc.role}
			x.Events = append([]model.Event{}, f.Events...)
			if tc.spell {
				x.Events[len(x.Events)-2].Stream = model.StreamNone
			}
			dd := d
			dd.Cause = tc.cause
			dd.SuggestedCause = tc.suggestion
			got := raidDeathRows(x, []model.Death{dd}, raidPrepConfig(t))
			if len(got) != 1 || got[0].Cause != tc.want {
				t.Fatalf("got %+v", got)
			}
		})
	}
	// A later hit in the same millisecond cannot explain this death.
	f.Events[len(f.Events)-2], f.Events[len(f.Events)-1] = end, hit
	got := raidDeathRows(f, []model.Death{d}, raidPrepConfig(t))
	if got[0].Cause != "unknown" {
		t.Fatal("post-death hit used")
	}
}
func TestRaidPageWipeWithoutDeath(t *testing.T) {
	r := raidPageFixture()
	r.Fights[0].Success = model.Some(0)
	r.Fights[0].DurationMs = model.Opt{}
	p, err := buildRaidPage(r, logfmt.ParseReport{}, nil, raidPrepConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "No first death recorded") || !strings.Contains(html, "fight duration: unknown duration") {
		t.Fatal("missing death invented")
	}
}
func TestRaidPageWipesAndProblems(t *testing.T) {
	r := raidPageFixture()
	f := &r.Fights[0]
	f.Success = model.Some(0)
	for i := 0; i < 3; i++ {
		f.Events = append(f.Events, model.Event{TimeMs: f.StartMs + 50000 + int64(i)*1000, Kind: model.UnitDied, DiedFlag: model.Some(0), Dst: f.Players[i]})
	}
	f.Events = append(f.Events, model.Event{TimeMs: r.EndMs + 1, Kind: model.UnitDied, DiedFlag: model.Some(0), Dst: f.Players[3]})
	save := raidPrepStore([]int64{1800000000}, []string{"Nobody-OtherRealm"})
	page, err := buildRaidPage(r, logfmt.ParseReport{Skipped: map[logfmt.SkipReason]int64{logfmt.UnknownWidth: 2}}, save, raidPrepConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderRaidBody(page)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1 bosses · 1 wipes", "fight duration: 1m0s", "Wipe 1", "First death", "Then (10s)", "Deaths (3)", "unknown", "Nobody-OtherRealm", "Unmatched prep reports (1)", "Deaths beyond the run end (1): not scored", "incomplete: skipped log lines", `data-death-filter="unknown"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if out := os.Getenv("NATIVE_HTML_OUT"); out != "" {
		if err := os.WriteFile(out+".raid-deaths", []byte(html), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if len(page.Deaths) != 3 || len(page.Wipes) != 1 || len(page.Wipes[0].Following) != 2 {
		t.Fatal("death/wipe accounting changed")
	}
}
