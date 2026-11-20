package score_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestDamageTakenOnlyPlayerIsUnmeasured(t *testing.T) {
	for _, role := range []string{"dps", "healer", "tank"} {
		f := fixture(role, role, role)
		a := f.Players[0]
		var es []model.Event
		for _, e := range f.Events {
			if e.Src.ID != a.ID {
				es = append(es, e)
			}
		}
		// A roster entry plus incoming damage and victim health is not an action.
		hit := mt.At(120000).Src(model.Actor{ID: f.BossUnits[0]}).Dst(a).Damage(100, -1).Landed().Build()
		hit.InfoUnit = a.ID
		hit.HP = model.Some(900)
		hit.HPMax = model.Some(1000)
		f.Events = append(es, hit)
		ns := score.Score([]model.Fight{f}, nil, []model.PrepReport{stock(a)}, config(t))
		n := playerScore(t, ns, a)
		if n.Tenths.Present || n.Rank.Present || n.Coverage != 0 {
			t.Errorf("incoming-only %s scored or ranked: %+v", role, n)
		}
		for _, name := range []string{"output", "activity"} {
			p := part(t, n, name)
			if p.Tenths.Present || p.Coverage != 0 || p.Baseline != "" {
				t.Errorf("incoming-only %s %s measured", role, name)
			}
		}
		if part(t, playerScore(t, ns, f.Players[1]), "output").Evidence.Counts["baseline_players"] != 2 {
			t.Error("incoming-only peer entered baseline")
		}
	}
}

func TestUnobservedRosterPlayerIsUnmeasured(t *testing.T) {
	f := fixture("dps", "dps", "dps")
	a := f.Players[0]
	var events []model.Event
	for _, e := range f.Events {
		if e.Src.ID != a.ID {
			events = append(events, e)
		}
	}
	f.Events = events
	ns := score.Score([]model.Fight{f}, nil, nil, config(t))
	n := playerScore(t, ns, a)
	if n.Tenths.Present || n.Rank.Present || n.Coverage != 0 {
		t.Fatalf("unobserved roster player scored as idle: coverage=%d score=%v rank=%v", n.Coverage, n.Tenths, n.Rank)
	}
	for _, p := range n.Fights[0].Parts {
		if p.Tenths.Present || p.Coverage != 0 {
			t.Fatalf("unobserved %s measured", p.Name)
		}
	}
	peer := part(t, playerScore(t, ns, f.Players[1]), "output")
	if peer.Evidence.Counts["baseline_players"] != 2 {
		t.Fatal("unobserved peer entered baseline")
	}
	// Observed unsuccessful action proves a real measured zero, unlike absence.
	f.Events = append(f.Events, mt.At(120000).Src(a).Dst(model.Actor{ID: f.BossUnits[0]}).Spell(81, "Synthetic miss").Miss("MISS").Build())
	n = playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a)
	if p := part(t, n, "output"); p.Tenths != model.Some(0) {
		t.Fatalf("observed zero lost: %+v", p)
	}
}
