// Package cfg loads replaceable, versioned Relax Raid data. It owns no scoring,
// parsing or fight-splitting engine. Load a fresh snapshot before each re-score.
package cfg

// Metadata follows rules/rules.json: Schema is the reader format, Version is
// the independently updatable content revision, and Source records provenance.
type Metadata struct {
	Schema  int    `json:"schema"`
	Version string `json:"version"`
	Source  string `json:"source"`
}

// Config is one complete snapshot. A failed Load never returns a partial one.
type Config struct {
	snapshot    *Config // private validated state; exported fields are detached copies
	Scoring     Scoring
	Specs       Specs
	SpecIDs     SpecIDs
	Support     Support
	Candidates  Candidates
	Encounters  Encounters
	Consumables Consumables
	Realms      RealmLabels
	Instances   Instances
}

type Scoring struct {
	Metadata
	Weights         RoleWeights  `json:"weights"`
	Night           NightWeights `json:"night"`
	Confidence      Confidence   `json:"confidence"`
	RatioPoints     []RatioPoint `json:"ratio_points"`
	CountStartScore float64      `json:"count_start_score"`
	Healer          Healer       `json:"healer"`
	Fights          Fights       `json:"fights"`
	Death           Death        `json:"death"`
	Toggles         Toggles      `json:"toggles"`
	Assignments     []Assignment `json:"assignments"`
	// Null means not specified, NOT zero or a compiled-in fallback. Leave the
	// affected measure unmeasured until a sourced/tuned value is supplied.
	NewAt60Seconds    *int64   `json:"new_at_60_seconds"`
	ActivityCastCapMs int64    `json:"activity_cast_cap_ms"` // positive bound for completed cast-start intervals
	GCDMs             *int64   `json:"gcd_ms"`
	GCDEnergyMs       *int64   `json:"gcd_energy_ms"` // cast PowerType 3; other casts use GCDMs
	RoundingStep      *float64 `json:"rounding_step"`
	// Inner measure ids belong to the score registry, not to cfg. Empty maps
	// do not authorize guessed equal weights or guessed death charges.
	MeasureWeights map[string]map[string]float64 `json:"measure_weights"`
	CountPenalties map[string]float64            `json:"count_penalties"`
}

type RoleWeights struct {
	DPS    PartWeights `json:"dps"`
	Healer PartWeights `json:"healer"`
	Tank   PartWeights `json:"tank"`
}
type PartWeights struct {
	Output      float64 `json:"output"`
	Activity    float64 `json:"activity"`
	Mechanics   float64 `json:"mechanics"`
	Utility     float64 `json:"utility"`
	Preparation float64 `json:"preparation"`
}
type NightWeights struct {
	TrashShare    float64 `json:"trash_share"`
	MaxTrashShare float64 `json:"max_trash_share"`
	MinBossShare  float64 `json:"min_boss_share"`
	KillWeight    float64 `json:"kill_weight"`
	WipeWeight    float64 `json:"wipe_weight"`
}
type Confidence struct {
	MinCoverage            float64 `json:"min_coverage"`
	ProvisionalMinCoverage float64 `json:"provisional_min_coverage"`
	LowConfidenceFightMs   int64   `json:"low_confidence_fight_ms"`
	ShortFightWeight       float64 `json:"short_fight_weight"`
	MinBaselinePlayers     int     `json:"min_baseline_players"`
	MinMeasurablePlayers   int     `json:"min_measurable_players"`
}
type RatioPoint struct {
	Ratio float64 `json:"ratio"`
	Score float64 `json:"score"`
}
type Healer struct {
	TeamShare         float64  `json:"team_share"`
	IndividualShare   float64  `json:"individual_share"`
	UnhealedTeamScore float64  `json:"unhealed_team_score"`
	UnhealedOutputCap float64  `json:"unhealed_output_cap"`
	LowHealthFraction *float64 `json:"low_health_fraction"`
}

// Fights supplies R4's pull and role rules. A tank share is over the boss
// units' landed swings, not source-side swings. Healing means effective healing.
// HealerStrictlyGreater distinguishes > from >=; tank classification wins first.
type Fights struct {
	PullGapMs                   int64   `json:"pull_gap_ms"`
	TailMs                      int64   `json:"tail_ms"`
	TankMinBossSwingShare       float64 `json:"tank_min_boss_swing_share"` // at least this share of the boss units' landed swings
	TankMinTopRatio             float64 `json:"tank_min_top_ratio"`        // and at least this fraction of the top taker's count
	ReentryGapMs                int64   `json:"reentry_gap_ms"`            // out and back into the same instance within this: one run
	HealerRequiresNotTank       bool    `json:"healer_requires_not_tank"`
	HealerMinHealingDamageRatio float64 `json:"healer_min_healing_damage_ratio"`
	HealerStrictlyGreater       bool    `json:"healer_strictly_greater"`
	BossUnitSelection           string  `json:"boss_unit_selection"`
}
type Death struct {
	FullHealthFraction float64 `json:"full_health_fraction"`
	MinHealthSamples   int     `json:"min_health_samples"`
	RaidwideShare      float64 `json:"raidwide_share"`
	RaidwideWindowMs   int64   `json:"raidwide_window_ms"`
	WindowMs           int64   `json:"window_ms"`
	PulledAggroMs      int64   `json:"pulled_aggro_ms"`
	WipeDeadShare      float64 `json:"wipe_dead_share"`
	WipeAllTanksDead   bool    `json:"wipe_all_tanks_dead"`
	WipeAllHealersDead bool    `json:"wipe_all_healers_dead"`
	OneShotWindowMs    int64   `json:"one_shot_window_ms"`
	UnhealedMinMs      int64   `json:"unhealed_min_ms"`
	LostBossGapMs      *int64  `json:"lost_boss_gap_ms"`
	SummaryWindowMs    *int64  `json:"summary_window_ms"`
}
type Toggles struct {
	RequiredConsumables bool `json:"required_consumables"`
	Readiness           bool `json:"readiness"`
	SupportCredit       bool `json:"support_credit"`
	History             bool `json:"history"`
}
type Assignment struct {
	Job                   string  `json:"job"`
	Role                  string  `json:"role"`
	OutputNA              bool    `json:"output_na"`
	MechanicsSurvivalOnly bool    `json:"mechanics_survival_only"`
	UtilityFromOfficer    bool    `json:"utility_from_officer"`
	OutputToUtility       float64 `json:"output_to_utility"`
}

type Specs struct {
	Metadata
	Profiles []Profile `json:"profiles"`
}
type SpecIDs struct {
	Metadata
	Classes map[string][]SpecID `json:"classes"`
}
type SpecID struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Default bool   `json:"default"`
}
type Profile struct {
	ID                 string     `json:"id"` // canonical CLASS.spec, never the non-unique spec word alone
	ClassName          string     `json:"class_name"`
	Spec               string     `json:"spec"`
	Roles              []string   `json:"roles"`
	ActiveTime         ActiveTime `json:"active_time"`
	ResourceExceptions []string   `json:"resource_exceptions"`
	Pets               string     `json:"pets"` // none, optional (including no-pet builds), required, totems
	Spells             []Spell    `json:"spells"`
	CastSignatures     []Spell    `json:"cast_signatures"` // empty: unknown spec, not a guessed match
	Support            []string   `json:"support"`         // ids in Support.Effects, each with scope
	Comparison         string     `json:"comparison"`
	Uncalibrated       bool       `json:"uncalibrated"`
	CalibrationNote    string     `json:"calibration_note"`
	Source             string     `json:"source"`
}
type ActiveTime struct {
	Casts       bool `json:"casts"`
	Channels    bool `json:"channels"`
	GCD         bool `json:"gcd"`
	AutoAttacks bool `json:"auto_attacks"`
	Dots        bool `json:"dots"`
	Pets        bool `json:"pets"`
}

// A name is always required. ID is optional and only legal with its own source.
type Spell struct {
	Name   string       `json:"name"`
	ID     int64        `json:"id,omitempty"`
	Source *SpellSource `json:"source,omitempty"`
}
type SpellSource struct {
	Kind string `json:"kind"` // real_log or forever_source
	Note string `json:"note"`
}
type Support struct {
	Metadata
	Effects []SupportEffect `json:"effects"`
	Overlap string          `json:"overlap"`
}
type SupportEffect struct {
	ID             string `json:"id"`
	Spell          Spell  `json:"spell"`
	Scope          string `json:"scope"`
	Stacks         *bool  `json:"stacks"` // null means unverified; never assume additive credit
	ExclusiveGroup string `json:"exclusive_group"`
	Credit         bool   `json:"credit"`
	Verified       bool   `json:"verified"` // sourced scope/stacking, not runtime attribution
	Source         string `json:"source"`
	Note           string `json:"note"`
}
type Candidates struct {
	Metadata
	Encounters []Candidate `json:"encounters"`
}
type Candidate struct {
	Name      string  `json:"name"`
	Raid      string  `json:"raid"`
	Abilities []Spell `json:"abilities"`
	Source    string  `json:"source"`
	Note      string  `json:"note"`
}
type Encounters struct {
	Metadata
	DefaultTargetRule string          `json:"default_target_rule"`
	Encounters        []EncounterRule `json:"encounters"`
}

// BossUnits replaces R4's maximum-health heuristic when nonempty; selectors
// identify NPC ids only, not names or per-pull GUIDs. TargetRule is a separate
// output policy and MUST NOT change which units supply the tank swing denominator.
type EncounterRule struct {
	ID         int64              `json:"id"`
	Name       string             `json:"name"`
	Source     string             `json:"source"`
	BossUnits  []CreatureSelector `json:"boss_units"`
	TargetRule *TargetRule        `json:"target_rule,omitempty"`
	Mechanics  []Mechanic         `json:"mechanics"`
}
type CreatureSelector struct {
	ID     int64  `json:"id"`
	Name   string `json:"name,omitempty"` // optional label, never a matching key
	Source string `json:"source"`
}
type TargetRule struct {
	Mode  string             `json:"mode"` // boss_only or boss_and_units
	Units []CreatureSelector `json:"units"`
}
type Mechanic struct {
	ID              string      `json:"id"`
	Spell           Spell       `json:"spell"`
	Avoidable       bool        `json:"avoidable"`
	Roles           []string    `json:"roles"` // empty means every role; key remains required
	AffectedPlayers string      `json:"affected_players"`
	ValidResponse   string      `json:"valid_response"`
	EvidenceSource  SpellSource `json:"evidence_source"` // real Forever log evidence, not a datamine
}
type Consumables struct {
	Metadata
	Categories []string `json:"categories"`
}
type RealmLabels struct {
	Metadata
	Labels []string `json:"labels"`
}
type Instances struct {
	Metadata
	RaidGroupSizeAbove int64      `json:"raid_group_size_above"`
	Instances          []Instance `json:"instances"`
}
type Instance struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
}
