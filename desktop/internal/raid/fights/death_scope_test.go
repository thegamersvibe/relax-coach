package fights

import (
	"io"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

var deathModes = []string{"Build/detail", "Build/index", "Read/detail", "Read/index"}

func deathResult(t *testing.T, events []model.Event, mode string) Result {
	t.Helper()
	p := params()
	if strings.HasSuffix(mode, "/index") {
		p.DetailsFor = nil
	}
	if strings.HasPrefix(mode, "Build/") {
		return build(t, events, p)
	}
	result, err := Read(strings.NewReader("invented death-scope fixture"), func(raw io.Reader, emit func(model.Event)) error {
		if _, err := io.Copy(io.Discard, raw); err != nil {
			return err
		}
		for _, e := range events {
			emit(e)
		}
		return nil
	}, p)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func scopeDeath(at int64) model.Event { return modeltest.At(at).Dst(alia).Died(0).Build() }

func TestPlayerDeathsOutsideRuns(t *testing.T) {
	start, end := modeltest.Encounter(model.Encounter{ID: 7, InstanceID: model.Some(42)}, 100, 200)
	out := modeltest.At(300).Zone(0, "Invented Outside", 0).Build()
	in := modeltest.At(90).Zone(42, "Invented Halls", 2).Build()
	nextStart, nextEnd := modeltest.Encounter(model.Encounter{ID: 8, InstanceID: model.Some(43)}, 500, 600)
	for _, tc := range []struct {
		name   string
		events []model.Event
		want   int64
		runs   int
	}{
		{"empty", nil, 0, 0},
		{"no-run", []model.Event{scopeDeath(1)}, 1, 0},
		{"before-instance-and-after-exit", []model.Event{scopeDeath(1), in, start, scopeDeath(150), end, out, scopeDeath(301)}, 2, 1},
		{"inclusive-run-edges", []model.Event{in, start, scopeDeath(100), end, scopeDeath(300), out, scopeDeath(300)}, 0, 1},
		{"just-outside-run-edges", []model.Event{in, start, scopeDeath(99), end, out, scopeDeath(301)}, 2, 1},
		{"between-runs", []model.Event{start, end, out, scopeDeath(400), modeltest.At(450).Zone(43, "Invented Raid", 2).Build(), nextStart, scopeDeath(550), nextEnd}, 1, 2},
		{"empty-visit", []model.Event{in, scopeDeath(100), out}, 1, 0},
		{"header-flush", []model.Event{scopeDeath(1), {Kind: model.Header, TimeMs: 50}, start, end, {Kind: model.Header, TimeMs: 300}, scopeDeath(400)}, 2, 1},
		// File order need not be chronological. Use the union of final spans,
		// not the current enabled flag or the first run alone.
		{"later-span", []model.Event{scopeDeath(150), in, start, end, out}, 0, 1},
		{"earlier-span", []model.Event{start, end, out, scopeDeath(150)}, 0, 1},
		{"clock-spike-past-final-end", []model.Event{start, scopeDeath(400), end, out}, 1, 1},
		{"same-time-deaths", []model.Event{scopeDeath(1), scopeDeath(1)}, 2, 0},
	} {
		for _, mode := range deathModes {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				result := deathResult(t, tc.events, mode)
				if len(result.Runs) != tc.runs {
					t.Fatalf("fixture runs=%d, want %d", len(result.Runs), tc.runs)
				}
				if result.PlayerDeathsOutsideRuns != tc.want {
					t.Fatalf("player deaths outside runs=%d, want %d", result.PlayerDeathsOutsideRuns, tc.want)
				}
			})
		}
	}
}

func TestPlayerDeathsInRunsWithoutFight(t *testing.T) {
	start, end := modeltest.Encounter(model.Encounter{ID: 7, InstanceID: model.Some(42)}, 100, 200)
	out := modeltest.At(300).Zone(0, "Invented Outside", 0).Build()
	back := modeltest.At(400).Zone(42, "Invented Return", 2).Build()
	nextStart, nextEnd := modeltest.Encounter(model.Encounter{ID: 8, InstanceID: model.Some(42)}, 500, 600)
	for _, tc := range []struct {
		name             string
		events           []model.Event
		missing, outside int64
	}{
		{"inside-boss", []model.Event{start, scopeDeath(150), end}, 0, 0},
		{"boss-start-and-end", []model.Event{start, scopeDeath(100), end, scopeDeath(200)}, 0, 0},
		{"inside-pull", []model.Event{hit(100), scopeDeath(101)}, 0, 0},
		// Fight membership includes the tail; EndMs still belongs to END.
		{"tail-equality", []model.Event{start, end, scopeDeath(2700), {TimeMs: 2800}}, 0, 0},
		{"after-tail", []model.Event{start, end, scopeDeath(2701)}, 1, 0},
		{"between-fights", []model.Event{start, end, scopeDeath(3000), hit(4000)}, 1, 0},
		{"before-first-fight", []model.Event{{TimeMs: 0}, scopeDeath(1), start, end}, 1, 0},
		{"no-return", []model.Event{start, end, out, scopeDeath(350)}, 0, 1},
		// The ruling uses the whole inclusive run span, including a re-entry
		// gap. It is diagnostic only: outside lines still enter no fight.
		{"resumed-span", []model.Event{start, end, out, scopeDeath(350), back, nextStart, nextEnd}, 1, 0},
		{"same-time-different-membership", []model.Event{start, end, scopeDeath(300), out, scopeDeath(300)}, 1, 0},
		{"empty-visit", []model.Event{scopeDeath(1)}, 0, 1},
		{"earlier-span", []model.Event{start, end, out, scopeDeath(150)}, 1, 0},
		{"clock-spike-outside-run", []model.Event{start, scopeDeath(400), end, out}, 0, 1},
		{"two-missing-lines", []model.Event{start, end, scopeDeath(3000), scopeDeath(3000)}, 2, 0},
	} {
		for _, mode := range deathModes {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				result := deathResult(t, tc.events, mode)
				if result.PlayerDeathsInRunsWithoutFight != tc.missing || result.PlayerDeathsOutsideRuns != tc.outside {
					t.Fatalf("in-run-no-fight=%d outside-runs=%d, want %d / %d", result.PlayerDeathsInRunsWithoutFight, result.PlayerDeathsOutsideRuns, tc.missing, tc.outside)
				}
			})
		}
	}
}

func TestPlayerDeathScopeUsesModelDeathAndPlayerGUID(t *testing.T) {
	charmed := alia
	charmed.Flags = 0x1248
	pet := modeltest.Pet("Invented Ward", alia)
	for _, tc := range []struct {
		name  string
		event model.Event
		want  int64
	}{
		{"real", scopeDeath(1), 1},
		{"charmed-player", modeltest.At(1).Dst(charmed).Died(0).Build(), 1},
		{"feign", modeltest.At(1).Dst(alia).Died(1).Build(), 0},
		{"other-flag", modeltest.At(1).Dst(alia).Died(2).Build(), 0},
		{"absent-flag", model.Event{TimeMs: 1, Kind: model.UnitDied, Dst: alia}, 0},
		{"party-kill", model.Event{TimeMs: 1, Kind: model.PartyKill, Dst: alia, DiedFlag: model.Some(0)}, 0},
		{"npc", modeltest.At(1).Dst(giant).Died(0).Build(), 0},
		{"owned-pet", modeltest.At(1).Dst(pet).Died(0).Build(), 0},
		{"player-source-only", modeltest.At(1).Src(alia).Dst(giant).Died(0).Build(), 0},
		{"no-destination", modeltest.At(1).Died(0).Build(), 0},
	} {
		for _, mode := range deathModes {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				result := deathResult(t, []model.Event{tc.event}, mode)
				if result.PlayerDeathsOutsideRuns != tc.want {
					t.Fatalf("player deaths outside runs=%d, want %d", result.PlayerDeathsOutsideRuns, tc.want)
				}
				// The same filter applies to unassigned deaths inside a run.
				es := append([]model.Event{{TimeMs: 0}, tc.event}, boss(7, 100)...)
				inside := deathResult(t, es, mode)
				if inside.PlayerDeathsOutsideRuns != 0 || inside.PlayerDeathsInRunsWithoutFight != tc.want {
					t.Fatalf("inside-run filter: outside=%d missing=%d, want 0 / %d", inside.PlayerDeathsOutsideRuns, inside.PlayerDeathsInRunsWithoutFight, tc.want)
				}
			})
		}
	}
}
