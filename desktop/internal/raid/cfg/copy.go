package cfg

import (
	"maps"
	"slices"
)

// Snapshot returns a detached copy of the loaded data, including every nested
// map, slice and optional value. Editing it or the public Load view is local to
// that copy; lookups continue to use the original validated snapshot.
func (c *Config) Snapshot() Config { return c.readOnly().clone() }

func (c *Config) readOnly() *Config {
	if c.snapshot != nil {
		return c.snapshot
	}
	return c // Also support caller-constructed Config values, without hidden defaults.
}

func copyPointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func copySpell(s Spell) Spell { s.Source = copyPointer(s.Source); return s }
func copySpells(in []Spell) []Spell {
	out := slices.Clone(in)
	for i, s := range out {
		out[i] = copySpell(s)
	}
	return out
}
func copyProfile(p Profile) Profile {
	p.Roles = slices.Clone(p.Roles)
	p.ResourceExceptions = slices.Clone(p.ResourceExceptions)
	p.Support = slices.Clone(p.Support)
	p.Spells = copySpells(p.Spells)
	p.CastSignatures = copySpells(p.CastSignatures)
	return p
}
func copyEncounter(e EncounterRule) EncounterRule {
	e.BossUnits = slices.Clone(e.BossUnits)
	e.TargetRule = copyPointer(e.TargetRule)
	if e.TargetRule != nil {
		e.TargetRule.Units = slices.Clone(e.TargetRule.Units)
	}
	e.Mechanics = slices.Clone(e.Mechanics)
	for i := range e.Mechanics {
		e.Mechanics[i].Roles = slices.Clone(e.Mechanics[i].Roles)
		e.Mechanics[i].Spell = copySpell(e.Mechanics[i].Spell)
	}
	return e
}
func (c *Config) clone() Config {
	out := *c
	out.snapshot = nil
	s := &out.Scoring
	s.RatioPoints = slices.Clone(s.RatioPoints)
	s.Assignments = slices.Clone(s.Assignments)
	s.NewAt60Seconds = copyPointer(s.NewAt60Seconds)
	s.GCDMs = copyPointer(s.GCDMs)
	s.GCDEnergyMs = copyPointer(s.GCDEnergyMs)
	s.RoundingStep = copyPointer(s.RoundingStep)
	s.Healer.LowHealthFraction = copyPointer(s.Healer.LowHealthFraction)
	s.Death.LostBossGapMs = copyPointer(s.Death.LostBossGapMs)
	s.Death.SummaryWindowMs = copyPointer(s.Death.SummaryWindowMs)
	s.MeasureWeights = maps.Clone(s.MeasureWeights)
	for part, w := range s.MeasureWeights {
		s.MeasureWeights[part] = maps.Clone(w)
	}
	s.CountPenalties = maps.Clone(s.CountPenalties)
	out.Specs.Profiles = slices.Clone(out.Specs.Profiles)
	for i, p := range out.Specs.Profiles {
		out.Specs.Profiles[i] = copyProfile(p)
	}
	out.SpecIDs.Classes = maps.Clone(out.SpecIDs.Classes)
	for class, ids := range out.SpecIDs.Classes {
		out.SpecIDs.Classes[class] = slices.Clone(ids)
	}
	out.Support.Effects = slices.Clone(out.Support.Effects)
	for i := range out.Support.Effects {
		e := &out.Support.Effects[i]
		e.Stacks = copyPointer(e.Stacks)
		e.Spell = copySpell(e.Spell)
	}
	out.Candidates.Encounters = slices.Clone(out.Candidates.Encounters)
	for i := range out.Candidates.Encounters {
		e := &out.Candidates.Encounters[i]
		e.Abilities = copySpells(e.Abilities)
	}
	out.Encounters.Encounters = slices.Clone(out.Encounters.Encounters)
	for i, e := range out.Encounters.Encounters {
		out.Encounters.Encounters[i] = copyEncounter(e)
	}
	out.Consumables.Categories = slices.Clone(out.Consumables.Categories)
	out.Realms.Labels = slices.Clone(out.Realms.Labels)
	out.Instances.Instances = slices.Clone(out.Instances.Instances)
	return out
}
