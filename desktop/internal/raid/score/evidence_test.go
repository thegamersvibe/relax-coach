package score_test

import (
	"encoding/json"
	"fmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"math"
	"testing"
)

func evidenceCheck(ns []model.NightScore, c cfg.Config) error {
	quantize := func(v float64) int64 {
		return int64(math.Round(math.Max(0, math.Min(100, math.Round(v / *c.Scoring.RoundingStep)**c.Scoring.RoundingStep)) * 10))
	}
	ratio := func(r float64) float64 {
		ps := c.Scoring.RatioPoints
		if r <= ps[0].Ratio {
			return ps[0].Score
		}
		for i := 1; i < len(ps); i++ {
			if r <= ps[i].Ratio {
				a, b := ps[i-1], ps[i]
				return a.Score + (b.Score-a.Score)*(r-a.Ratio)/(b.Ratio-a.Ratio)
			}
		}
		return ps[len(ps)-1].Score
	}
	for _, n := range ns {
		if n.Tenths.Present && (n.Tenths.V < 0 || n.Tenths.V > 1000) {
			return fmt.Errorf("night bounds")
		}
		for _, f := range n.Fights {
			var fw cfg.PartWeights
			switch f.Role {
			case "dps":
				fw = c.Scoring.Weights.DPS
			case "healer":
				fw = c.Scoring.Weights.Healer
			case "tank":
				fw = c.Scoring.Weights.Tank
			}
			weights := map[string]float64{"output": fw.Output, "activity": fw.Activity, "mechanics": fw.Mechanics, "utility": fw.Utility, "preparation": fw.Preparation}
			fightSum, fightWeight := 0.0, 0.0
			for _, p := range f.Parts {
				x := p.Evidence.Counts
				raw := map[string]float64{}
				value := func(k string) float64 { return float64(x[k]) }
				if x["measured.boss_damage"] == 1 {
					raw["boss_damage"] = ratio(value("boss_damage") / value("alive_ms") / (value("baseline_damage_num") / value("baseline_damage_den")))
				}
				if x["measured.active_time"] == 1 {
					raw["active_time"] = ratio(value("active_ms") / value("alive_ms") / (value("baseline_active_num") / value("baseline_active_den")))
				}
				if x["measured.healer_team"] == 1 {
					raw["healer_team"] = c.Scoring.CountStartScore
					if x["unhealed_deaths"] > 0 {
						raw["healer_team"] = c.Scoring.Healer.UnhealedTeamScore
					}
				}
				if x["measured.healer_individual"] == 1 {
					raw["healer_individual"] = ratio(value("low_health_effective") * value("baseline_players") / value("team_low_health_effective"))
				}
				if x["measured.deaths"] == 1 {
					raw["deaths"] = c.Scoring.CountStartScore - c.Scoring.CountPenalties["owned_death"]*value("death_severity_num")/value("death_severity_den")
				}
				if x["measured.boss_swings"] == 1 {
					raw["boss_swings"] = c.Scoring.CountStartScore - c.Scoring.CountPenalties["boss_swing_taken"]*value("boss_swings_taken")
				}
				if x["measured.stock"] == 1 {
					raw["stock"] = 0
					if x["potion_count"] > 0 {
						raw["stock"] += 50
					}
					if x["food_count"] > 0 {
						raw["stock"] += 50
					}
				}
				if x["measured.bag_gear"] == 1 {
					raw["bag_gear"] = c.Scoring.CountStartScore - c.Scoring.CountPenalties["better_item_in_bags"]*value("better_gear_slots")
				}
				inner := c.Scoring.MeasureWeights[p.Name]
				if p.Name == "output" && f.Role == "healer" {
					inner = map[string]float64{"healer_team": c.Scoring.Healer.TeamShare * 100, "healer_individual": c.Scoring.Healer.IndividualShare * 100}
				}
				sum, w := 0.0, 0.0
				for key, v := range raw {
					expected := quantize(v)
					if x["points_tenths."+key] != expected {
						return fmt.Errorf("raw evidence %s: got %d want %d", key, x["points_tenths."+key], expected)
					}
					w += inner[key]
					sum += math.Max(0, math.Min(100, v)) * inner[key]
				}
				if p.Tenths.Present {
					if w == 0 || p.Coverage != int(math.Floor(w)) {
						return fmt.Errorf("part coverage")
					}
					want := quantize(sum / w)
					if p.Name == "output" && f.Role == "healer" && x["unhealed_deaths"] > 0 {
						want = min(want, quantize(c.Scoring.Healer.UnhealedOutputCap))
					}
					if p.Tenths.V != want {
						return fmt.Errorf("part recomputation %s: got %d want %d", p.Name, p.Tenths.V, want)
					}
					weight := weights[p.Name] * w / 100
					fightWeight += weight
					fightSum += weight * float64(p.Tenths.V) / 10
				}
				if len(p.Evidence.Text) > 200 || len(p.Evidence.AtMs) > 20 {
					return fmt.Errorf("evidence bound")
				}
			}
			if f.Coverage != int(math.Floor(fightWeight)) {
				return fmt.Errorf("fight coverage")
			}
			if f.Tenths.Present && f.Tenths.V != quantize(fightSum/fightWeight) {
				return fmt.Errorf("fight recomputation")
			}
		}
	}
	return nil
}
func TestEvidenceRecomputationAndAuraEdge(t *testing.T) {
	c := config(t)
	for _, f := range []model.Fight{healingFixture(), fixture("dps", "dps", "tank", "tank"), fixture("healer"), fixture("tank")} {
		var rs []model.PrepReport
		for _, a := range f.Players {
			rs = append(rs, stock(a))
		}
		ns := score.Score([]model.Fight{f}, nil, rs, c)
		if err := evidenceCheck(ns, c); err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(ns)
		f.Events = append(f.Events, model.Event{TimeMs: f.EndMs, Kind: model.AuraRemoved, Src: f.Players[0]})
		after, _ := json.Marshal(score.Score([]model.Fight{f}, nil, rs, c))
		if string(before) != string(after) {
			t.Fatal("aura edge changed scores")
		}
		f.Events[len(f.Events)-1].TimeMs++
		after, _ = json.Marshal(score.Score([]model.Fight{f}, nil, rs, c))
		if string(before) != string(after) {
			t.Fatal("aura past edge changed scores")
		}
	}
}
func TestCoverageWeightedNight(t *testing.T) {
	c := config(t)
	a := fixture("tank", "tank")
	a.ID = "high"
	b := fixture("tank")
	b.ID = "low"
	ns := score.Score([]model.Fight{a, b}, nil, nil, c)
	n := playerScore(t, ns, a.Players[0])
	// Exact coverage is 47.5 and 17.5 before storage floors it to 47 and 17.
	// Aggregation must not use those displayed integers as weights.
	x, y := n.Fights[0], n.Fights[1]
	want := int64(math.Round((float64(x.Tenths.V)*47.5 + float64(y.Tenths.V)*17.5) / 65))
	if n.Tenths != model.Some(want) || n.Coverage != 32 {
		t.Fatalf("night coverage weighting got %v/%d want %d/32", n.Tenths, n.Coverage, want)
	}
}
