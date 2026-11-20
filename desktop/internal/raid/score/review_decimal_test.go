package score_test

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

// Tune real JSON and re-load: no mutation of a loaded public view is mistaken
// for changing the private validated snapshot.
func tunedScoring(t *testing.T, edit func(map[string]any)) cfg.Config {
	t.Helper()
	dir := t.TempDir()
	files, err := filepath.Glob("../../../../rules/raid/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(file) == "scoring.json" {
			var m map[string]any
			if err = json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			edit(m)
			b, err = json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
		}
		if err = os.WriteFile(filepath.Join(dir, filepath.Base(file)), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	c, err := cfg.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return c.Snapshot()
}

func decimalHealerConfig(t *testing.T, team, individual, output float64) cfg.Config {
	return tunedScoring(t, func(m map[string]any) {
		h := m["healer"].(map[string]any)
		h["team_share"] = team
		h["individual_share"] = individual
		m["weights"].(map[string]any)["healer"] = map[string]any{"output": output, "activity": 0, "mechanics": 0, "utility": 100 - output, "preparation": 0}
	})
}

func TestSecondHealerDecimalCoverage(t *testing.T) {
	for _, x := range []struct {
		team, individual float64
		want             int
	}{{.29, .71, 29}, {.57, .43, 57}, {.58, .42, 58}} {
		c := decimalHealerConfig(t, x.team, x.individual, 100)
		n := score.Score([]model.Fight{fixture("healer")}, nil, nil, c)[0]
		p := part(t, n, "output")
		if p.Coverage != x.want || n.Fights[0].Coverage != x.want || n.Coverage != x.want {
			t.Errorf("share=%g coverage part/fight/night=%d/%d/%d want=%d", x.team, p.Coverage, n.Fights[0].Coverage, n.Coverage, x.want)
		}
	}
}

func TestSecondHealerDecimalScore(t *testing.T) {
	c := decimalHealerConfig(t, .29, .71, 100)
	f := fixture("healer", "healer", "tank")
	a, b, target := f.Players[0], f.Players[1], f.Players[2]
	f.Events = append(f.Events, model.Event{TimeMs: 115000, InfoUnit: target.ID, HP: model.Some(1), HPMax: model.Some(100)}, mt.At(116000).Src(a).Dst(target).Heal(1843, 0).Build(), mt.At(117000).Src(b).Dst(target).Heal(2133, 0).Build())
	n := playerScore(t, score.Score([]model.Fight{f}, nil, nil, c), a)
	p := part(t, n, "output")
	// Independent integer oracle: individual ratio=3686/3976, points=140*r-70;
	// exact weighted part=71.45, half away from zero gives 715 tenths.
	raw := new(big.Rat).Mul(big.NewRat(3686, 3976), big.NewRat(140, 1))
	raw.Sub(raw, big.NewRat(70, 1))
	raw.Mul(raw, big.NewRat(71, 100))
	raw.Add(raw, big.NewRat(29, 1))
	if raw.Cmp(big.NewRat(7145, 100)) != 0 {
		t.Fatal("invalid oracle")
	}
	if p.Tenths != model.Some(715) || p.Coverage != 100 {
		t.Fatalf("exact=%s got=%v coverage=%d", raw, p.Tenths, p.Coverage)
	}
}

func TestSecondHealerIndividualDecimalShare(t *testing.T) {
	c := decimalHealerConfig(t, .71, .29, 100)
	f := healingFixture()
	p := part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, c), f.Players[0]), "output")
	if p.Coverage != 100 {
		t.Fatalf("individual decimal share corrupted coverage: %d", p.Coverage)
	}
}

func TestSecondHealerExactThresholds(t *testing.T) {
	for _, w := range []float64{15, 60} {
		c := decimalHealerConfig(t, .29, .71, w)
		f := healingFixture()
		n := playerScore(t, score.Score([]model.Fight{f}, nil, nil, c), f.Players[0])
		if part(t, n, "output").Coverage != 100 || n.Fights[0].Coverage != int(w) || n.Coverage != int(w) || !n.Fights[0].Tenths.Present || !n.Tenths.Present || n.Rank.Present != (w == 60) {
			t.Fatalf("threshold=%g part/fight/night=%d/%d/%d score=%v rank=%v", w, part(t, n, "output").Coverage, n.Fights[0].Coverage, n.Coverage, n.Tenths, n.Rank)
		}
	}
}
