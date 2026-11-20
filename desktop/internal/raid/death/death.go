// Package death reviews selected fight events. It has no parser, file writer,
// or dependency on fights' private detail representation.
package death

import (
	"sort"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Causes are the model's descriptive values; NIGHT-FILE.md maps them to wire keys.
const (
	Avoidable     = "avoidable ability"
	Unhealed      = "unhealed over time"
	PulledAggro   = "pulled aggro"
	TankLostBoss  = "tank lost the boss"
	Unavoidable   = "unavoidable or one-shot"
	AfterWipeCall = "after the wipe call"
	Unresolved    = "unresolved"
)

type verdict struct {
	cause, owner  string
	owners        []string
	evidence      []int64
	low, terminal bool
}
type entry struct {
	order int
	apply func(*context) verdict
}

var rules []entry

// Register installs a package-local rule during init. New rule_<name>.go files
// supply a unique DESIGN section 8 order. Runtime registration is not supported.
func Register(order int, apply func(*context) verdict) {
	for _, r := range rules {
		if r.order == order {
			panic("death: duplicate rule order")
		}
	}
	rules = append(rules, entry{order, apply})
	sort.Slice(rules, func(i, j int) bool { return rules[i].order < rules[j].order })
}

type context struct {
	view      *fightView
	params    cfg.Death
	mechanics []cfg.Mechanic
	victim    model.Actor
	at        int64
	window    []model.Event
	wipeCall  model.Opt
}

// Review returns deaths in stable time order and summaries of wiped boss fights.
// An index-only fight (no Events) is not judged. Config should be a validated
// cfg snapshot; no tuning values are supplied by this package. Inputs are not
// modified. Evidence holds absolute event times, as modeltest.DeathOf expects.
func Review(f model.Fight, config cfg.Config) ([]model.Death, []model.Wipe) {
	if len(f.Events) == 0 {
		return nil, nil
	}
	snapshot := config.Snapshot()
	v := newView(f)
	encounter, _ := snapshot.Encounter(f.EncounterID)
	call := wipeCall(v, snapshot.Scoring.Death)
	var deaths []model.Death
	for _, e := range v.events {
		p, ok := v.players[e.Dst.ID]
		if !e.IsDeath() || !ok {
			continue
		}
		c := context{view: v, params: snapshot.Scoring.Death, victim: p, at: e.TimeMs, mechanics: encounter.Mechanics, wipeCall: call}
		c.window = v.window(p.ID, c.at-c.params.WindowMs, c.at)
		deaths = append(deaths, classify(&c))
	}
	return deaths, summaries(v, deaths, snapshot.Scoring.Death)
}

func classify(c *context) model.Death {
	d := model.Death{Player: c.victim, FightID: c.view.f.ID, TimeMs: c.at,
		Cause: Unresolved, Confidence: "low", OwnerKind: "undecided", Evidence: recap(c.window, c.victim.ID)}
	var best verdict
	matches := 0
	for _, r := range rules {
		m := r.apply(c)
		if m.cause == "" {
			continue
		}
		if m.low {
			if d.SuggestedCause == "" {
				d.SuggestedCause = m.cause
			}
			continue
		}
		matches++
		if best.cause == "" {
			best = m
		}
		if m.terminal {
			break
		}
	}
	if best.cause != "" {
		d.Cause, d.OwnerKind, d.Owners = best.cause, best.owner, best.owners
		d.SuggestedCause = ""
		d.Confidence = "high"
		if matches > 1 {
			d.Confidence = "medium"
		}
		if best.cause == Unresolved {
			d.Confidence = "low"
		}
		if best.evidence != nil {
			d.Evidence = best.evidence
		}
	}
	return d
}

func recap(window []model.Event, victimID string) []int64 {
	var times []int64
	for _, e := range window {
		if e.Dst.ID == victimID && (damage(e) || e.Kind == model.Heal || e.Kind == model.Absorb) {
			times = append(times, e.TimeMs)
			if len(times) > 10 {
				times = times[1:]
			}
		}
	}
	return times
}
