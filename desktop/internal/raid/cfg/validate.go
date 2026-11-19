package cfg

import (
	"fmt"
	"math"
	"strings"
)

func fraction(v float64) bool   { return v >= 0 && v <= 1 }
func scoreValue(v float64) bool { return v >= 0 && v <= 100 }
func nonblank(v string) bool    { return strings.TrimSpace(v) != "" }
func oneOf(v string, options ...string) bool {
	for _, o := range options {
		if v == o {
			return true
		}
	}
	return false
}
func same(a, b float64) bool         { return math.Abs(a-b) < 1e-9 }
func positiveOptional(v *int64) bool { return v == nil || *v > 0 }

func (s Scoring) validate() error {
	for role, w := range map[string]PartWeights{"dps": s.Weights.DPS, "healer": s.Weights.Healer, "tank": s.Weights.Tank} {
		if !scoreValue(w.Output) || !scoreValue(w.Activity) || !scoreValue(w.Mechanics) || !scoreValue(w.Utility) || !scoreValue(w.Preparation) || !same(w.Output+w.Activity+w.Mechanics+w.Utility+w.Preparation, 100) {
			return fmt.Errorf("weights.%s must be nonnegative and sum to 100", role)
		}
	}
	n := s.Night
	if !fraction(n.TrashShare) || !fraction(n.MaxTrashShare) || !fraction(n.MinBossShare) || n.TrashShare > n.MaxTrashShare || !same(n.MaxTrashShare+n.MinBossShare, 1) || n.KillWeight <= 0 || n.WipeWeight < 0 || n.WipeWeight >= n.KillWeight {
		return fmt.Errorf("invalid night shares or kill/wipe weights")
	}
	c := s.Confidence
	if !fraction(c.MinCoverage) || c.LowConfidenceFightMs <= 0 || c.ShortFightWeight <= 0 || c.ShortFightWeight > 1 || c.MinMeasurablePlayers < 2 || c.MinBaselinePlayers < c.MinMeasurablePlayers {
		return fmt.Errorf("invalid confidence thresholds or cohort minimums")
	}
	f := s.Fights
	if f.PullGapMs <= 0 || f.TailMs < 0 || f.TankMinBossSwingShare <= 0 || !fraction(f.TankMinBossSwingShare) || f.TankMinTopRatio <= 0 || !fraction(f.TankMinTopRatio) || f.ReentryGapMs <= 0 || f.HealerMinHealingDamageRatio <= 0 || f.BossUnitSelection != "highest_max_health_damaged_creature" {
		return fmt.Errorf("invalid fights timing, role rule or boss-unit selection")
	}
	if len(s.RatioPoints) < 2 {
		return fmt.Errorf("ratio_points needs at least two points")
	}
	for i, p := range s.RatioPoints {
		if p.Ratio < 0 || !scoreValue(p.Score) || (i > 0 && (p.Ratio <= s.RatioPoints[i-1].Ratio || p.Score < s.RatioPoints[i-1].Score)) {
			return fmt.Errorf("ratio_points must increase by ratio with nondecreasing scores in [0,100]")
		}
	}
	h := s.Healer
	if !scoreValue(s.CountStartScore) || !fraction(h.TeamShare) || !fraction(h.IndividualShare) || !same(h.TeamShare+h.IndividualShare, 1) || !scoreValue(h.UnhealedTeamScore) || !scoreValue(h.UnhealedOutputCap) || (h.LowHealthFraction != nil && (*h.LowHealthFraction <= 0 || !fraction(*h.LowHealthFraction))) {
		return fmt.Errorf("invalid count/healer parameters")
	}
	d := s.Death
	if d.WindowMs <= 0 || d.OneShotWindowMs <= 0 || d.UnhealedMinMs <= 0 || d.OneShotWindowMs > d.WindowMs || d.UnhealedMinMs > d.WindowMs || d.WipeDeadShare <= 0 || !fraction(d.WipeDeadShare) || !positiveOptional(d.LostBossGapMs) || !positiveOptional(d.SummaryWindowMs) {
		return fmt.Errorf("invalid death windows or wipe threshold")
	}
	if !positiveOptional(s.NewAt60Seconds) || !positiveOptional(s.GCDMs) || (s.RoundingStep != nil && (*s.RoundingStep <= 0 || !scoreValue(*s.RoundingStep))) {
		return fmt.Errorf("invalid optional threshold")
	}
	seen := map[string]bool{}
	for _, a := range s.Assignments {
		if !nonblank(a.Job) || seen[a.Job] || (a.Role != "" && !oneOf(a.Role, "dps", "healer", "tank")) || !scoreValue(a.OutputToUtility) {
			return fmt.Errorf("invalid/duplicate assignment %q", a.Job)
		}
		seen[a.Job] = true
	}
	for part, weights := range s.MeasureWeights {
		if !oneOf(part, "output", "activity", "mechanics", "utility", "preparation") || len(weights) == 0 {
			return fmt.Errorf("invalid measure part %q", part)
		}
		total := 0.0
		for id, v := range weights {
			if !nonblank(id) || v < 0 {
				return fmt.Errorf("invalid measure weight %q", id)
			}
			total += v
		}
		if total <= 0 {
			return fmt.Errorf("measure weights for %s are all zero", part)
		}
	}
	for id, v := range s.CountPenalties {
		if !nonblank(id) || !scoreValue(v) {
			return fmt.Errorf("invalid count penalty %q", id)
		}
	}
	return nil
}

func stringsUnique(values []string) bool {
	seen := map[string]bool{}
	for _, v := range values {
		if !nonblank(v) || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}
func rolesValid(values []string) bool {
	if len(values) == 0 || !stringsUnique(values) {
		return false
	}
	for _, v := range values {
		if !oneOf(v, "dps", "healer", "tank") {
			return false
		}
	}
	return true
}
func (s Spell) validate() error {
	if !nonblank(s.Name) || s.ID < 0 {
		return fmt.Errorf("spell needs a name and a nonnegative optional id")
	}
	if s.ID != 0 && s.Source == nil {
		return fmt.Errorf("spell id for %q needs its own provenance", s.Name)
	}
	if s.Source != nil && (!oneOf(s.Source.Kind, "real_log", "forever_source") || !nonblank(s.Source.Note)) {
		return fmt.Errorf("spell %q source must identify a real log or Forever source", s.Name)
	}
	return nil
}
func validateSpells(spells []Spell) error {
	seen := map[string]bool{}
	for _, s := range spells {
		if err := s.validate(); err != nil {
			return err
		}
		key := fmt.Sprintf("%s/%d", s.Name, s.ID)
		if seen[key] {
			return fmt.Errorf("duplicate spell %q", s.Name)
		}
		seen[key] = true
	}
	return nil
}

func (c *Config) validate() error {
	if err := c.Scoring.validate(); err != nil {
		return fmt.Errorf("scoring.json: %w", err)
	}
	expected := map[string]bool{}
	for class, ids := range c.SpecIDs.Classes {
		if !nonblank(class) || class != strings.ToUpper(class) || len(ids) == 0 {
			return fmt.Errorf("spec-ids.json: empty/invalid class %q", class)
		}
		for _, id := range ids {
			key := class + "." + id.ID
			if !nonblank(id.ID) || strings.Contains(id.ID, ".") || !nonblank(id.Label) || expected[key] {
				return fmt.Errorf("spec-ids.json: invalid/duplicate id %q", key)
			}
			expected[key] = true
		}
	}
	if len(expected) == 0 {
		return fmt.Errorf("spec-ids.json: no ids")
	}
	effects := map[string]bool{}
	if c.Support.Overlap != "split_equally" {
		return fmt.Errorf("support.json: unsupported overlap policy")
	}
	for _, e := range c.Support.Effects {
		if !nonblank(e.ID) || effects[e.ID] || !nonblank(e.Source) || !nonblank(e.Note) || !oneOf(e.Scope, "raid", "party", "class", "single", "enemy", "personal", "unknown") || (oneOf(e.Scope, "personal", "unknown") && e.Credit) {
			return fmt.Errorf("support.json: invalid/duplicate effect %q", e.ID)
		}
		if err := e.Spell.validate(); err != nil {
			return fmt.Errorf("support.json: %w", err)
		}
		effects[e.ID] = true
	}
	seen := map[string]bool{}
	for _, p := range c.Specs.Profiles {
		if !expected[p.ID] || seen[p.ID] || p.ID != p.ClassName+"."+p.Spec {
			return fmt.Errorf("specs.json: unexpected/duplicate/mismatched profile %q", p.ID)
		}
		seen[p.ID] = true
		if !rolesValid(p.Roles) || !oneOf(p.Pets, "none", "optional", "required", "totems") || !nonblank(p.Source) || !nonblank(p.Comparison) || !nonblank(p.CalibrationNote) || len(p.Spells) == 0 || !stringsUnique(p.ResourceExceptions) || !stringsUnique(p.Support) {
			return fmt.Errorf("specs.json: incomplete/invalid profile %s", p.ID)
		}
		if err := validateSpells(p.Spells); err != nil {
			return fmt.Errorf("specs.json: %s: %w", p.ID, err)
		}
		if err := validateSpells(p.CastSignatures); err != nil {
			return fmt.Errorf("specs.json: %s cast signatures: %w", p.ID, err)
		}
		for _, id := range p.Support {
			if !effects[id] {
				return fmt.Errorf("specs.json: %s references unknown support %q", p.ID, id)
			}
		}
	}
	for id := range expected {
		if !seen[id] {
			return fmt.Errorf("specs.json: missing profile %s from spec-ids.json", id)
		}
	}
	if err := c.validateEncounters(); err != nil {
		return err
	}
	if len(c.Consumables.Categories) == 0 || !stringsUnique(c.Consumables.Categories) {
		return fmt.Errorf("consumables.json: empty/duplicate categories")
	}
	if len(c.Realms.Labels) == 0 || !stringsUnique(c.Realms.Labels) {
		return fmt.Errorf("realm-labels.json: empty/duplicate labels")
	}
	if c.Instances.RaidGroupSizeAbove < 1 {
		return fmt.Errorf("instances.json: invalid raid group threshold")
	}
	instanceIDs := map[int64]bool{}
	for _, i := range c.Instances.Instances {
		if i.ID <= 0 || instanceIDs[i.ID] || !nonblank(i.Name) || !nonblank(i.Source) || !oneOf(i.Kind, "raid", "dungeon") {
			return fmt.Errorf("instances.json: invalid/duplicate instance %d", i.ID)
		}
		instanceIDs[i.ID] = true
	}
	return nil
}

func selectorsValid(units []CreatureSelector) bool {
	seen := map[string]bool{}
	for _, u := range units {
		key := fmt.Sprintf("%d/%s", u.ID, u.Name)
		if u.ID < 0 || (u.ID == 0 && !nonblank(u.Name)) || !nonblank(u.Source) || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}
func (c *Config) validateEncounters() error {
	candidates := map[string]bool{}
	for _, e := range c.Candidates.Encounters {
		key := e.Raid + "/" + e.Name
		if !nonblank(e.Name) || !nonblank(e.Raid) || !nonblank(e.Source) || !nonblank(e.Note) || candidates[key] {
			return fmt.Errorf("encounters-candidate.json: invalid/duplicate candidate %q", key)
		}
		candidates[key] = true
		if err := validateSpells(e.Abilities); err != nil {
			return fmt.Errorf("encounters-candidate.json: %w", err)
		}
	}
	if c.Encounters.DefaultTargetRule != "boss_only" {
		return fmt.Errorf("encounters-enabled.json: default target rule must be boss_only")
	}
	ids := map[int64]bool{}
	for _, e := range c.Encounters.Encounters {
		if e.ID <= 0 || ids[e.ID] || !nonblank(e.Name) || !nonblank(e.Source) || !selectorsValid(e.BossUnits) {
			return fmt.Errorf("encounters-enabled.json: invalid/duplicate encounter %d or boss override", e.ID)
		}
		ids[e.ID] = true
		if t := e.TargetRule; t != nil {
			if !oneOf(t.Mode, "boss_only", "boss_and_units") || !selectorsValid(t.Units) || (t.Mode == "boss_and_units" && len(t.Units) == 0) || (t.Mode == "boss_only" && len(t.Units) != 0) {
				return fmt.Errorf("encounters-enabled.json: invalid target rule for %d", e.ID)
			}
		}
		mechanics := map[string]bool{}
		for _, m := range e.Mechanics {
			if !nonblank(m.ID) || mechanics[m.ID] || !rolesValid(m.Roles) || !nonblank(m.AffectedPlayers) || !nonblank(m.ValidResponse) || m.EvidenceSource.Kind != "real_log" || !nonblank(m.EvidenceSource.Note) {
				return fmt.Errorf("encounters-enabled.json: mechanic %q needs identified players, response and real-log evidence", m.ID)
			}
			mechanics[m.ID] = true
			if err := m.Spell.validate(); err != nil {
				return fmt.Errorf("encounters-enabled.json: %w", err)
			}
		}
	}
	return nil
}
