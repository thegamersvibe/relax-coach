package cfg

import (
	"fmt"
	"math"
	"strings"
)

func fraction(v float64) bool    { return v >= 0 && v <= 1 }
func scoreValue(v float64) bool  { return v >= 0 && v <= 100 }
func normalized(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
func nonblank(v string) bool     { return strings.TrimSpace(v) != "" }
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

type fieldCheck struct {
	field   string
	invalid bool
}

func checkFields(prefix string, checks ...fieldCheck) error {
	for _, c := range checks {
		if c.invalid {
			return fmt.Errorf("%s%s: invalid value", prefix, c.field)
		}
	}
	return nil
}

func (s Scoring) validate() error {
	for role, w := range map[string]PartWeights{"dps": s.Weights.DPS, "healer": s.Weights.Healer, "tank": s.Weights.Tank} {
		if err := checkFields("weights."+role+".",
			fieldCheck{"output", !scoreValue(w.Output)}, fieldCheck{"activity", !scoreValue(w.Activity)},
			fieldCheck{"mechanics", !scoreValue(w.Mechanics)}, fieldCheck{"utility", !scoreValue(w.Utility)},
			fieldCheck{"preparation", !scoreValue(w.Preparation)}); err != nil {
			return err
		}
		if !same(w.Output+w.Activity+w.Mechanics+w.Utility+w.Preparation, 100) {
			return fmt.Errorf("weights.%s: parts must sum to 100", role)
		}
	}
	n, c, f, h, d := s.Night, s.Confidence, s.Fights, s.Healer, s.Death
	if err := checkFields("",
		fieldCheck{"night.max_trash_share", !fraction(n.MaxTrashShare)},
		fieldCheck{"night.trash_share", !fraction(n.TrashShare) || n.TrashShare > n.MaxTrashShare},
		fieldCheck{"night.min_boss_share", !fraction(n.MinBossShare) || !same(n.MaxTrashShare+n.MinBossShare, 1)},
		fieldCheck{"night.kill_weight", n.KillWeight <= 0 || n.KillWeight > 1},
		fieldCheck{"night.wipe_weight", n.WipeWeight < 0 || n.WipeWeight >= n.KillWeight},
		fieldCheck{"confidence.min_coverage", !fraction(c.MinCoverage)},
		fieldCheck{"confidence.provisional_min_coverage", c.ProvisionalMinCoverage <= 0 || c.ProvisionalMinCoverage >= c.MinCoverage},
		fieldCheck{"confidence.low_confidence_fight_ms", c.LowConfidenceFightMs <= 0},
		fieldCheck{"confidence.short_fight_weight", c.ShortFightWeight <= 0 || c.ShortFightWeight > 1},
		fieldCheck{"confidence.min_measurable_players", c.MinMeasurablePlayers < 2},
		fieldCheck{"confidence.min_baseline_players", c.MinBaselinePlayers < c.MinMeasurablePlayers},
		fieldCheck{"fights.pull_gap_ms", f.PullGapMs <= 0},
		fieldCheck{"fights.tail_ms", f.TailMs < 0 || f.TailMs > f.PullGapMs},
		fieldCheck{"fights.tank_min_boss_swing_share", f.TankMinBossSwingShare <= 0 || !fraction(f.TankMinBossSwingShare)},
		fieldCheck{"fights.tank_min_top_ratio", f.TankMinTopRatio <= 0 || !fraction(f.TankMinTopRatio)},
		fieldCheck{"fights.reentry_gap_ms", f.ReentryGapMs <= 0},
		fieldCheck{"fights.healer_min_healing_damage_ratio", f.HealerMinHealingDamageRatio <= 0},
		fieldCheck{"fights.boss_unit_selection", f.BossUnitSelection != "highest_max_health_damaged_creature"},
		fieldCheck{"count_start_score", !scoreValue(s.CountStartScore)},
		fieldCheck{"healer.team_share", !fraction(h.TeamShare)},
		fieldCheck{"healer.individual_share", !fraction(h.IndividualShare) || !same(h.TeamShare+h.IndividualShare, 1)},
		fieldCheck{"healer.unhealed_team_score", !scoreValue(h.UnhealedTeamScore)},
		fieldCheck{"healer.unhealed_output_cap", !scoreValue(h.UnhealedOutputCap)},
		fieldCheck{"healer.low_health_fraction", h.LowHealthFraction != nil && (*h.LowHealthFraction <= 0 || !fraction(*h.LowHealthFraction))},
		fieldCheck{"death.full_health_fraction", d.FullHealthFraction <= 0 || !fraction(d.FullHealthFraction)},
		fieldCheck{"death.min_health_samples", d.MinHealthSamples < 1},
		fieldCheck{"death.raidwide_share", d.RaidwideShare <= 0 || !fraction(d.RaidwideShare)},
		fieldCheck{"death.raidwide_window_ms", d.RaidwideWindowMs <= 0},
		fieldCheck{"death.window_ms", d.WindowMs <= 0},
		fieldCheck{"death.one_shot_window_ms", d.OneShotWindowMs <= 0 || d.OneShotWindowMs > d.WindowMs},
		fieldCheck{"death.unhealed_min_ms", d.UnhealedMinMs <= 0 || d.UnhealedMinMs > d.WindowMs},
		fieldCheck{"death.wipe_dead_share", d.WipeDeadShare <= 0 || !fraction(d.WipeDeadShare)},
		fieldCheck{"death.lost_boss_gap_ms", !positiveOptional(d.LostBossGapMs)},
		fieldCheck{"death.summary_window_ms", !positiveOptional(d.SummaryWindowMs)},
		fieldCheck{"new_at_60_seconds", !positiveOptional(s.NewAt60Seconds)},
		fieldCheck{"gcd_ms", !positiveOptional(s.GCDMs)},
		fieldCheck{"gcd_energy_ms", !positiveOptional(s.GCDEnergyMs)},
		fieldCheck{"rounding_step", s.RoundingStep != nil && (*s.RoundingStep <= 0 || !scoreValue(*s.RoundingStep))},
	); err != nil {
		return err
	}
	if len(s.RatioPoints) < 2 {
		return fmt.Errorf("ratio_points: needs at least two points")
	}
	for i, p := range s.RatioPoints {
		if err := checkFields(fmt.Sprintf("ratio_points[%d].", i),
			fieldCheck{"ratio", p.Ratio < 0 || (i > 0 && p.Ratio <= s.RatioPoints[i-1].Ratio)},
			fieldCheck{"score", !scoreValue(p.Score) || (i > 0 && p.Score < s.RatioPoints[i-1].Score)}); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for i, a := range s.Assignments {
		if err := checkFields(fmt.Sprintf("assignments[%d].", i),
			fieldCheck{"job", !nonblank(a.Job) || seen[normalized(a.Job)]},
			fieldCheck{"role", a.Role != "" && !oneOf(a.Role, "dps", "healer", "tank")},
			fieldCheck{"output_to_utility", !scoreValue(a.OutputToUtility)}); err != nil {
			return err
		}
		seen[normalized(a.Job)] = true
	}
	for part, weights := range s.MeasureWeights {
		if !oneOf(part, "output", "activity", "mechanics", "utility", "preparation") || len(weights) == 0 {
			return fmt.Errorf("measure_weights.%s: invalid/empty part", part)
		}
		total := 0.0
		for id, v := range weights {
			if !nonblank(id) || !scoreValue(v) {
				return fmt.Errorf("measure_weights.%s.%s: weight must be in [0,100]", part, id)
			}
			total += v
		}
		if !same(total, 100) {
			return fmt.Errorf("measure_weights.%s: weights must sum to 100", part)
		}
	}
	for id, v := range s.CountPenalties {
		if !nonblank(id) || !scoreValue(v) {
			return fmt.Errorf("count_penalties.%s: penalty must be in [0,100]", id)
		}
	}
	return nil
}

func stringsUnique(values []string) bool {
	seen := map[string]bool{}
	for _, v := range values {
		if !nonblank(v) || seen[normalized(v)] {
			return false
		}
		seen[normalized(v)] = true
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
	if err := checkFields("", fieldCheck{"name", !nonblank(s.Name)}, fieldCheck{"id", s.ID < 0}); err != nil {
		return err
	}
	if s.ID != 0 && s.Source == nil {
		return fmt.Errorf("source: spell id for %q needs its own provenance", s.Name)
	}
	if s.Source != nil {
		return checkFields("source.", fieldCheck{"kind", !oneOf(s.Source.Kind, "real_log", "forever_source")}, fieldCheck{"note", !nonblank(s.Source.Note)})
	}
	return nil
}
func validateSpells(spells []Spell) error {
	seen := map[string]bool{}
	for i, s := range spells {
		if err := s.validate(); err != nil {
			return fmt.Errorf("[%d].%w", i, err)
		}
		key := normalized(s.Name)
		if seen[key] {
			return fmt.Errorf("[%d].name: duplicate spell %q", i, s.Name)
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
	specKeys := map[string]bool{}
	for class, ids := range c.SpecIDs.Classes {
		path := fmt.Sprintf("spec-ids.json: classes[%q]", class)
		if !nonblank(class) || class != strings.ToUpper(class) || len(ids) == 0 {
			return fmt.Errorf("%s: empty/invalid class", path)
		}
		for i, id := range ids {
			key := class + "." + id.ID
			normalizedKey := normalized(class) + "." + normalized(id.ID)
			if err := checkFields(fmt.Sprintf("%s[%d].", path, i),
				fieldCheck{"id", !nonblank(id.ID) || strings.Contains(id.ID, ".") || specKeys[normalizedKey]},
				fieldCheck{"label", !nonblank(id.Label)}); err != nil {
				return err
			}
			expected[key] = true
			specKeys[normalizedKey] = true
		}
	}
	if len(expected) == 0 {
		return fmt.Errorf("spec-ids.json: classes: no ids")
	}
	effects := map[string]bool{}
	effectIDs := map[string]bool{}
	if c.Support.Overlap != "split_equally" {
		return fmt.Errorf("support.json: overlap: unsupported policy")
	}
	for i, e := range c.Support.Effects {
		path := fmt.Sprintf("support.json: effects[%d].", i)
		if e.Credit && !e.Verified {
			return fmt.Errorf("%scredit requires verified=true", path)
		}
		if e.Credit && e.Stacks == nil {
			return fmt.Errorf("%sstacks: credit requires an explicit stacking rule", path)
		}
		if err := checkFields(path,
			fieldCheck{"id", !nonblank(e.ID) || effectIDs[normalized(e.ID)]},
			fieldCheck{"source", !nonblank(e.Source)}, fieldCheck{"note", !nonblank(e.Note)},
			fieldCheck{"scope", !oneOf(e.Scope, "raid", "party", "class", "single", "enemy", "personal", "unknown")},
			fieldCheck{"credit", oneOf(e.Scope, "personal", "unknown") && e.Credit}); err != nil {
			return err
		}
		if err := e.Spell.validate(); err != nil {
			return fmt.Errorf("%sspell.%w", path, err)
		}
		effects[e.ID] = true
		effectIDs[normalized(e.ID)] = true
	}
	seen := map[string]bool{}
	for i, p := range c.Specs.Profiles {
		path := fmt.Sprintf("specs.json: profiles[%d].", i)
		if !expected[p.ID] || seen[normalized(p.ID)] || p.ID != p.ClassName+"."+p.Spec {
			return fmt.Errorf("%sid: unexpected/duplicate/mismatched profile %q (class_name/spec)", path, p.ID)
		}
		seen[normalized(p.ID)] = true
		if err := checkFields(path,
			fieldCheck{"roles", !rolesValid(p.Roles)}, fieldCheck{"pets", !oneOf(p.Pets, "none", "optional", "required", "totems")},
			fieldCheck{"source", !nonblank(p.Source)}, fieldCheck{"comparison", !nonblank(p.Comparison)},
			fieldCheck{"calibration_note", !nonblank(p.CalibrationNote)}, fieldCheck{"spells", len(p.Spells) == 0},
			fieldCheck{"resource_exceptions", !stringsUnique(p.ResourceExceptions)}, fieldCheck{"support", !stringsUnique(p.Support)}); err != nil {
			return err
		}
		if err := validateSpells(p.Spells); err != nil {
			return fmt.Errorf("%sspells%w", path, err)
		}
		if err := validateSpells(p.CastSignatures); err != nil {
			return fmt.Errorf("%scast_signatures%w", path, err)
		}
		for j, id := range p.Support {
			if !effects[id] {
				return fmt.Errorf("%ssupport[%d]: unknown support %q", path, j, id)
			}
		}
	}
	for id := range expected {
		if !seen[normalized(id)] {
			return fmt.Errorf("specs.json: profiles: missing profile %s from spec-ids.json", id)
		}
	}
	if err := c.validateEncounters(); err != nil {
		return err
	}
	if len(c.Consumables.Categories) == 0 || !stringsUnique(c.Consumables.Categories) {
		return fmt.Errorf("consumables.json: categories: empty/duplicate values")
	}
	if len(c.Realms.Labels) == 0 || !stringsUnique(c.Realms.Labels) {
		return fmt.Errorf("realm-labels.json: labels: empty/duplicate values")
	}
	if c.Instances.RaidGroupSizeAbove < 5 {
		return fmt.Errorf("instances.json: raid_group_size_above must be at least 5")
	}
	instanceIDs := map[int64]bool{}
	for index, i := range c.Instances.Instances {
		if err := checkFields(fmt.Sprintf("instances.json: instances[%d].", index),
			fieldCheck{"id", i.ID <= 0 || instanceIDs[i.ID]}, fieldCheck{"name", !nonblank(i.Name)},
			fieldCheck{"source", !nonblank(i.Source)}, fieldCheck{"kind", !oneOf(i.Kind, "raid", "dungeon")}); err != nil {
			return err
		}
		instanceIDs[i.ID] = true
	}
	return c.validateSpellIdentities()
}

func validateSelectors(units []CreatureSelector, path string) error {
	seen := map[int64]bool{}
	for i, u := range units {
		if err := checkFields(fmt.Sprintf("%s[%d].", path, i),
			fieldCheck{"id", u.ID <= 0 || seen[u.ID]}, fieldCheck{"source", !nonblank(u.Source)}); err != nil {
			return err
		}
		seen[u.ID] = true
	}
	return nil
}

func (c *Config) validateEncounters() error {
	candidates := map[string]bool{}
	for i, e := range c.Candidates.Encounters {
		key := normalized(e.Raid) + "/" + normalized(e.Name)
		path := fmt.Sprintf("encounters-candidate.json: encounters[%d].", i)
		if err := checkFields(path,
			fieldCheck{"name", !nonblank(e.Name)}, fieldCheck{"raid", !nonblank(e.Raid)},
			fieldCheck{"source", !nonblank(e.Source)}, fieldCheck{"note", !nonblank(e.Note)}); err != nil {
			return err
		}
		if candidates[key] {
			return fmt.Errorf("%sname/raid: duplicate candidate %q", path, key)
		}
		candidates[key] = true
		if err := validateSpells(e.Abilities); err != nil {
			return fmt.Errorf("%sabilities%w", path, err)
		}
	}
	if c.Encounters.DefaultTargetRule != "boss_only" {
		return fmt.Errorf("encounters-enabled.json: default_target_rule must be boss_only")
	}
	ids := map[int64]bool{}
	for i, e := range c.Encounters.Encounters {
		path := fmt.Sprintf("encounters-enabled.json: encounters[%d].", i)
		if err := checkFields(path,
			fieldCheck{"id", e.ID <= 0 || ids[e.ID]}, fieldCheck{"name", !nonblank(e.Name)}, fieldCheck{"source", !nonblank(e.Source)}); err != nil {
			return err
		}
		if err := validateSelectors(e.BossUnits, path+"boss_units"); err != nil {
			return err
		}
		ids[e.ID] = true
		if t := e.TargetRule; t != nil {
			if !oneOf(t.Mode, "boss_only", "boss_and_units") {
				return fmt.Errorf("%starget_rule.mode: invalid mode", path)
			}
			if err := validateSelectors(t.Units, path+"target_rule.units"); err != nil {
				return err
			}
			if (t.Mode == "boss_and_units" && len(t.Units) == 0) || (t.Mode == "boss_only" && len(t.Units) != 0) {
				return fmt.Errorf("%starget_rule.units: inconsistent with mode", path)
			}
		}
		mechanics := map[string]bool{}
		for j, m := range e.Mechanics {
			mp := fmt.Sprintf("%smechanics[%d].", path, j)
			if err := checkFields(mp,
				fieldCheck{"id", !nonblank(m.ID) || mechanics[normalized(m.ID)]}, fieldCheck{"roles", len(m.Roles) != 0 && !rolesValid(m.Roles)},
				fieldCheck{"affected_players", !nonblank(m.AffectedPlayers)}, fieldCheck{"valid_response", !nonblank(m.ValidResponse)},
				fieldCheck{"evidence_source.kind", m.EvidenceSource.Kind != "real_log"}, fieldCheck{"evidence_source.note", !nonblank(m.EvidenceSource.Note)}); err != nil {
				return err
			}
			mechanics[normalized(m.ID)] = true
			if err := m.Spell.validate(); err != nil {
				return fmt.Errorf("%sspell.%w", mp, err)
			}
		}
	}
	return nil
}
