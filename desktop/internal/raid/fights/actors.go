package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"maps"
	"sort"
	"strconv"
	"strings"
)

func player(id string) bool { return strings.HasPrefix(id, "Player-") }

// GUID identity wins over the mutable combat-log type field: mind control
// carries TYPE_PET (bit 12) or TYPE_GUARDIAN (bit 13), not a new character.
func pet(a model.Actor) bool {
	return !player(a.ID) && (strings.HasPrefix(a.ID, "Pet-") || byte(a.Flags>>12)&3 != 0)
}
func creature(id string) bool {
	return strings.HasPrefix(id, "Creature-") || strings.HasPrefix(id, "Vehicle-")
}

// Keep run-wide ownership facts, not run-wide events. Late evidence repairs
// already closed fights; raw events survive only for explicitly selected detail.
type actorIndex struct {
	actors   map[string]model.Actor
	owners   map[string]string
	priority map[string]int
	pets     map[string]bool
	logger   model.Actor
	advanced bool
}

func newActorIndex() *actorIndex {
	return &actorIndex{actors: map[string]model.Actor{}, owners: map[string]string{}, priority: map[string]int{}, pets: map[string]bool{}}
}
func (s *actorIndex) putOwner(id, owner string, rank int) {
	if !player(id) && id != "" && owner != "" && owner != "0" && id != owner && rank > s.priority[id] {
		s.owners[id], s.priority[id], s.pets[id] = owner, rank, true
	}
}
func (s *actorIndex) observe(e model.Event) {
	if e.InfoUnit != "" && (e.HP.Present || e.HPMax.Present || e.Power.Present || e.PowerMax.Present || e.PowerType.Present || e.X.Present || e.Y.Present) {
		s.advanced = true
	}
	for _, a := range []model.Actor{e.Src, e.Dst, e.Caster} {
		if a.ID == "" {
			continue
		}
		if a.Name == "" {
			a.Name = s.actors[a.ID].Name
		}
		s.actors[a.ID] = a
		if pet(a) {
			s.pets[a.ID] = true
		}
		s.putOwner(a.ID, a.OwnerID, 2)
		if player(a.ID) && a.Flags&1 != 0 {
			s.logger = a
		}
	}
	if e.Kind == model.Summon && !player(e.Dst.ID) {
		s.putOwner(e.Dst.ID, e.Src.ID, 1)
		s.pets[e.Dst.ID] = true
	}
	s.putOwner(e.InfoUnit, e.InfoOwnerID, 3)
}
func (s *actorIndex) resolved() map[string]string {
	resolved := map[string]string{}
	for id := range s.owners {
		seen := map[string]bool{id: true}
		owner := s.owners[id]
		for owner != "" && !player(owner) && !seen[owner] {
			seen[owner] = true
			owner = s.owners[owner]
		}
		if player(owner) {
			resolved[id] = owner
		}
	}
	return resolved
}

type fightFacts struct {
	touched      []string
	selected     bool
	bossEvidence []model.Event // distinct creature/source pairs and max HP only
}

func collectFightFacts(f model.Fight) fightFacts {
	touched := map[string]bool{}
	for _, e := range f.Events {
		for _, id := range []string{e.Src.ID, e.Dst.ID, e.InfoUnit, e.Caster.ID} {
			if id != "" {
				touched[id] = true
			}
		}
	}
	return fightFacts{touched: keys(touched)}
}
func (s *actorIndex) project(f *model.Fight, touched []string, resolved map[string]string) {
	f.Owners, f.Players, f.Unowned = map[string]string{}, nil, nil
	participants := map[string]bool{}
	for _, c := range f.Combatants {
		if player(c.PlayerID) {
			participants[c.PlayerID] = true
		}
	}
	for _, id := range touched {
		if player(id) {
			participants[id] = true
		}
		if owner := resolved[id]; owner != "" {
			f.Owners[id] = owner
			participants[owner] = true
		}
		if s.pets[id] && resolved[id] == "" {
			f.Unowned = append(f.Unowned, id)
		}
	}
	sort.Strings(f.Unowned)
	for _, id := range keys(participants) {
		a := s.actors[id]
		a.ID = id
		f.Players = append(f.Players, a)
	}
	if f.Roles == nil {
		f.Roles = map[string]string{}
	}
}
func (s *actorIndex) finish(r *Run, facts []fightFacts, p Params) {
	r.Logger, r.Advanced = s.logger, r.Advanced || s.advanced
	resolved, roster := s.resolved(), map[string]bool{}
	for i := range r.Fights {
		f, fact, d := &r.Fights[i], facts[i], &r.Details[i]
		old := f.Owners
		s.project(f, fact.touched, resolved)
		if !maps.Equal(old, f.Owners) {
			f.BossUnits = nil
			if fact.selected {
				selectBosses(f, p)
				f.Roles = map[string]string{}
				*d = makeDetail(*f, p)
				f.Specs = d.Specs
			} else {
				// Compact selector evidence preserves late owner attribution in index mode.
				f.Events = fact.bossEvidence
				selectBosses(f, p)
				f.Events = nil
			}
		}
		for _, a := range f.Players {
			roster[a.ID] = true
		}
		if f.Partial {
			d.UnscoredReason = "partialFight"
		}
		if r.Kind == model.RunUnknown {
			d.UnscoredReason = "unknownInstance"
		}
	}
	for _, id := range keys(roster) {
		a := s.actors[id]
		a.ID = id
		r.Roster = append(r.Roster, a)
	}
}

// Index-only fights need no timeline events. Retain one damage candidate per
// source/creature pair, one actor per creature and one maximum per InfoUnit.
func bossEvidence(f model.Fight) []model.Event {
	if !f.Boss {
		return nil
	}
	pairs := map[[2]string]model.Event{}
	units := map[string]model.Actor{}
	hp := map[string]int64{}
	for _, e := range f.Events {
		for _, a := range []model.Actor{e.Src, e.Dst} {
			if creature(a.ID) {
				units[a.ID] = a
			}
		}
		if e.Kind == model.Damage && creature(e.Dst.ID) {
			pairs[[2]string{e.Src.ID, e.Dst.ID}] = model.Event{Kind: model.Damage, Src: e.Src, Dst: e.Dst}
		}
		if e.HPMax.Present {
			hp[e.InfoUnit] = max(hp[e.InfoUnit], e.HPMax.V)
		}
	}
	var out []model.Event
	for _, e := range pairs {
		out = append(out, e)
	}
	for _, a := range units {
		out = append(out, model.Event{Src: a})
	}
	for id, h := range hp {
		out = append(out, model.Event{InfoUnit: id, HPMax: model.Some(h)})
	}
	return out
}
func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func selectBosses(f *model.Fight, p Params) {
	if !f.Boss {
		return
	}
	candidates := map[string]model.Actor{}
	hp := map[string]int64{}
	for _, e := range f.Events {
		if e.Kind == model.Damage && creature(e.Dst.ID) && (player(e.Src.ID) || f.Owners[e.Src.ID] != "") {
			candidates[e.Dst.ID] = e.Dst
		}
		if e.HPMax.Present && e.HPMax.V > hp[e.InfoUnit] {
			hp[e.InfoUnit] = e.HPMax.V
		}
	}
	var rules []CreatureSelector
	if p.BossUnits != nil {
		rules = p.BossUnits(f.EncounterID)
	}
	if len(rules) > 0 {
		// An override can name a boss with no player damage (e.g. a separate phase).
		for _, e := range f.Events {
			for _, a := range []model.Actor{e.Src, e.Dst} {
				if creature(a.ID) {
					candidates[a.ID] = a
				}
			}
		}
		for id, a := range candidates {
			for _, rule := range rules {
				if matches(a, rule) {
					f.BossUnits = append(f.BossUnits, id)
					break
				}
			}
		}
		sort.Strings(f.BossUnits)
		return
	}
	if p.BossUnitSelection != "highest_max_health_damaged_creature" {
		return
	}
	best := ""
	for id := range candidates {
		if hp[id] > 0 && (best == "" || hp[id] > hp[best] || hp[id] == hp[best] && id < best) {
			best = id
		}
	}
	if best != "" {
		f.BossUnits = []string{best}
	}
}
func matches(a model.Actor, r CreatureSelector) bool {
	if r.ID == 0 && r.Name == "" {
		return false
	}
	if r.Name != "" && r.Name != a.Name {
		return false
	}
	if r.ID != 0 {
		parts := strings.Split(a.ID, "-")
		if len(parts) < 7 {
			return false
		}
		id, err := strconv.ParseInt(parts[5], 10, 64)
		if err != nil || id != r.ID {
			return false
		}
	}
	return true
}
