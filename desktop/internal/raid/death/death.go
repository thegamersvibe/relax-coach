// Package death reviews selected fight events. It has no parser, file writer,
// or dependency on fights' private detail representation.
package death

import (
	"fmt"
	"sort"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Causes are the model's descriptive values; NIGHT-FILE.md maps them to wire keys.
const (
	Avoidable     = model.CauseAvoidable
	Unhealed      = model.CauseUnhealed
	PulledAggro   = model.CausePulledAggro
	TankLostBoss  = model.CauseLostBoss
	Unavoidable   = model.CauseUnavoidable
	AfterWipeCall = model.CauseAfterWipe
	Unresolved    = model.CauseUnresolved
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
	view       *fightView
	params     cfg.Death
	mechanics  []cfg.Mechanic
	victim     model.Actor
	at         int64
	start, end int // absolute event indices; end identifies this UNIT_DIED line
	window     []model.Event
	wipeCall   model.Opt
	wipeLine   int
}

// Review returns deaths in stable time order, summaries of wiped boss fights,
// the count of real player deaths stamped after the final runEndMs, and an error.
// A nonpositive runEndMs is invalid, including for an index-only fight.
// Callers pass the final run end, never the encounter end (valid tails can exceed it).
// An index-only fight (no Events) is not judged. Config should be a validated
// cfg snapshot; no tuning values are supplied by this package. Inputs are not
// modified. Evidence holds absolute event times, as modeltest.DeathOf expects.
func Review(f model.Fight, runEndMs int64, config cfg.Config) ([]model.Death, []model.Wipe, int, error) {
	if runEndMs <= 0 {
		return nil, nil, 0, fmt.Errorf("death: runEndMs must be positive")
	}
	if len(f.Events) == 0 {
		return nil, nil, 0, nil
	}
	snapshot := config.Snapshot()
	v := newView(f)
	encounter, _ := snapshot.Encounter(f.EncounterID)
	call, callLine := wipeCall(v, snapshot.Scoring.Death)
	var deaths []model.Death
	excluded := 0
	for i, e := range v.events {
		p, ok := v.players[e.Dst.ID]
		if !e.IsDeath() || !ok {
			continue
		}
		if e.TimeMs > runEndMs {
			excluded++
			continue
		}
		c := context{view: v, params: snapshot.Scoring.Death, victim: p, at: e.TimeMs, end: i, mechanics: encounter.Mechanics, wipeCall: call, wipeLine: callLine}
		c.start, c.window = v.deathWindow(p.ID, c.at-c.params.WindowMs, i)
		deaths = append(deaths, classify(&c))
	}
	// Presentation order is separate from every causal event position.
	sort.SliceStable(deaths, func(i, j int) bool { return deaths[i].TimeMs < deaths[j].TimeMs })
	return deaths, summaries(v, deaths, snapshot.Scoring.Death), excluded, nil
}

func classify(c *context) model.Death {
	d := model.Death{Player: c.victim, FightID: c.view.f.ID, TimeMs: c.at,
		Cause: Unresolved, Confidence: "low", OwnerKind: model.OwnerUndecided, Evidence: recap(c.window, c.victim.ID)}
	// One backward clock step anywhere in the fight's log order declines every
	// rule for every death in it: no rule runs and nothing falls through to a
	// later rule. The whole fight is the range because the raid-wide rule reads
	// survival and same-spell hits across it, including lines after the death.
	// A miss is preferred to a false charge (see the package README).
	if !c.view.clock {
		return d
	}
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
		if m.cause != Unresolved {
			matches++
		}
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
