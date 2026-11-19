# Relax Raid configuration (R5)

`cfg.Load(dir)` reads all nine JSON files in this directory on **every call**;
there are no embedded defaults or process caches. It returns `(*cfg.Config,
error)` and returns no partial config on error. Publish an update as a complete
validated directory, then switch directories between scoring jobs; callers must
not edit the directory while a snapshot is being loaded. Keep the previous good
snapshot when a replacement fails. The loader never writes the input files.

## Version and validation contract

Like `rules/rules.json`, `schema` is the reader format and `version` is the
content revision. Schema 1 is supported; another schema is refused. A newer
`major.minor.patch` **content** version is accepted without rebuilding, provided
its schema is supported. Every file requires a source note. Bump content versions
when tuning data; bump schema for incompatible field changes.

Unknown keys (also nested), case-folded aliases, duplicate keys, missing required
fields, null nonoptional values, malformed/trailing JSON, invalid ranges,
duplicate identities and broken profile/support references are rejected with
file context. Empty arrays and explicit false/zero values are not missing data.
Nullable fields mean **unknown**, not zero and not permission for a compiled-in
fallback. Spell IDs require their own `{kind, note}` source (`real_log` or
`forever_source`); a schema check cannot independently establish the truth of a
source claim, so provenance review is still required.

`spec-ids.json.classes` is the JSON-value copy of the GearQuest spec list in
`wow-forever-leveling/assets/specs.json`, retaining id, label and default.
Profiles use `CLASS.spec` identities, because `holy`, `protection` and
`restoration` are not unique words. Loading requires exact profile coverage of
that copy, and the seed test requires 33 profiles. CI does not access the skill
folder.

## Consumer alignment

The full exported shapes and JSON tags are in
`desktop/internal/raid/cfg/types.go`. No consumer needs to read raw JSON.

`Config` fields:

| Field | Type | File |
|---|---|---|
| `Scoring` | `Scoring` | scoring.json |
| `Specs` | `Specs` (`Profiles []Profile`) | specs.json |
| `SpecIDs` | `SpecIDs` (`Classes map[string][]SpecID`) | spec-ids.json |
| `Support` | `Support` (`Effects []SupportEffect`) | support.json |
| `Candidates` | `Candidates` (`Encounters []Candidate`) | encounters-candidate.json |
| `Encounters` | `Encounters` (`DefaultTargetRule string`, `Encounters []EncounterRule`) | encounters-enabled.json |
| `Consumables` | `Consumables` (`Categories []string`) | consumables.json |
| `Realms` | `RealmLabels` (`Labels []string`) | realm-labels.json |
| `Instances` | `Instances` (`RaidGroupSizeAbove int64`, `Instances []Instance`) | instances.json |

All file structs embed `Metadata { Schema int; Version, Source string }`.

### R4: fights

- `Scoring.Fights.PullGapMs int64` = 10000; `TailMs int64` = 2500.
- `Scoring.Confidence.LowConfidenceFightMs int64` = 45000;
  `ShortFightWeight float64` = 0.5 (R7 applies the weight).
- `Scoring.Fights.TankMinBossSwingShare float64` = 0.15 and
  `TankMinTopRatio float64` = 0.5: a tank takes **at least** 15% of the boss
  units' landed swings (not the source-side swing stream) **and** at least half
  as many as the top taker, so four tanks sharing a boss are all tanks.
- `ReentryGapMs int64` = 1800000 (30 minutes, unproven: no log yet shows a
  return): a ZONE_CHANGE out of an instance and back into the same instance id
  within this gap, with no other instance in between, continues the run; lines
  outside belong to no fight.
- `HealerRequiresNotTank bool` = true, `HealerMinHealingDamageRatio float64`
  = 1, `HealerStrictlyGreater bool` = true. Test tank first; otherwise compare
  effective healing with damage done using the configured strictness. No swing
  evidence is not a tank share of 100%.
- `BossUnitSelection string` = `highest_max_health_damaged_creature`:
  maximum observed health among creatures players damaged, not the encounter
  name. `c.Encounter(encounterID)` returns `(EncounterRule, bool)` from enabled
  rules only. A nonempty `EncounterRule.BossUnits []CreatureSelector` replaces
  the heuristic; selectors have `ID int64`, `Name, Source string`. ID denotes a
  creature type, never a per-pull GUID. Match the supplied selector fields
  conjunctively when both ID and name are present.
- Output policy is separate: `c.OutputTargets(encounterID) TargetRule` defaults
  to `boss_only`; `boss_and_units` adds explicitly listed units. Do not count
  those added output targets in the tank's boss-swing denominator.
- `c.InstanceKind(instanceID, groupSize int64) (model.RunKind, bool)` prioritizes
  the listed kind. The bool is **listed**, not confidence: callers must report
  unlisted IDs even when group size above `Instances.RaidGroupSizeAbove` (5)
  identifies a raid. An unlisted five-player encounter is unknown, not an
  invented dungeon listing. Only Stratholme (329, dungeon) is shipped.
- `Realms.Labels []string` starts with PvP-EU. `c.Profile("CLASS.spec")`
  returns `(Profile, bool)`. All `Profile.CastSignatures []Spell` are deliberately
  empty: none of the study's shared/hybrid spell lists establishes a validated
  cast-only discriminator. R4 therefore keeps spec unknown, not guessed.

### R7: score

- `Scoring.Weights RoleWeights { DPS, Healer, Tank PartWeights }`;
  `PartWeights { Output, Activity, Mechanics, Utility, Preparation float64 }`.
- `Scoring.Night NightWeights { TrashShare, MaxTrashShare, MinBossShare,
  KillWeight, WipeWeight float64 }`.
- `Scoring.Confidence Confidence { MinCoverage float64;
  LowConfidenceFightMs int64; ShortFightWeight float64;
  MinBaselinePlayers, MinMeasurablePlayers int }` (3 and 2; a sole player is N/A).
- `Scoring.RatioPoints []RatioPoint { Ratio, Score float64 }` stores
  0.5→0, 1→70, 1.3→100. `CountStartScore float64` = 100.
- `Scoring.Healer Healer { TeamShare, IndividualShare, UnhealedTeamScore,
  UnhealedOutputCap float64; LowHealthFraction *float64 }` stores the half/half
  split and the unhealed-death guard. The health cutoff is not specified yet.
- `Scoring.Death Death { WindowMs int64; WipeDeadShare float64;
  WipeAllTanksDead, WipeAllHealersDead bool; OneShotWindowMs, UnhealedMinMs int64;
  LostBossGapMs, SummaryWindowMs *int64 }` holds DESIGN 8/9 values for R6 too.
- `Scoring.Toggles Toggles { RequiredConsumables, Readiness, SupportCredit,
  History bool }` starts all off. `Assignments []Assignment` holds the five
  future jobs and 15-point output-to-utility transfers. Neither implements
  phase-2 behavior in cfg.
- `NewAt60Seconds, GCDMs *int64`, `RoundingStep *float64`,
  `MeasureWeights map[string]map[string]float64`,
  `CountPenalties map[string]float64` are exposed without guessed defaults.
  The maps are empty and the optional fields are null until values are agreed.
  R7 must not silently treat an unconfigured measure as fully measured.

## Evidence boundaries and open tuning

The five uncalibrated profiles are `PALADIN.shockadin`,
`SHAMAN.enhancement_tank`, and all three `MAGE.battlemage_*` profiles. Shockadin's
raid viability and comparison cohort are unsettled; Enhancement tank has only
small-group/emergency design support; the Battle Mage labels are experimental,
not established raid archetypes. Other profiles describe documented archetypes,
**not validated numerical raid benchmarks**; runtime peer baselines remain
provisional. No profile contains a spell ID, item name or talent table.

Candidate encounters contain the study's 22 named encounters and only identified
ability-name leads. Unassigned Crystal Giant mechanics were not assigned to
Amethrax from resemblance. The enabled registry starts empty. Enabling a
mechanic requires affected-player and valid-response descriptions plus
`EvidenceSource SpellSource` with kind `real_log`; datamining alone is refused.
Unknown-ability reports belong to the event consumers, not the loader.

Support `Stacks *bool` describes additive copies from **different providers**,
not a single aura's charges or Sunder stack count. It is null where unverified;
no cross-family stacking behavior is invented. Personal effects never earn
support credit. Furious Howl has conflicting party versus party/raid scope in
the two study reports: its scope is `unknown` and credit is disabled pending
source resolution. A phase-2 consumer must not assume additive credit from null
stacking. Consumables are categories only; requirements and item mappings need
separate evidence.

The brief does not provide inner-measure weights, per-death penalties, the
low-health cutoff, GCD duration, rounding step, new-at-60 threshold, lost-boss gap
or wipe-summary window. These remain explicit null/empty data, rather than
new builder-invented values. They must be settled before the corresponding R6/R7
measures are enabled.

`desktop/internal/itemdata` does not exist on this base. `cfg.ItemLabel(id)`
returns the decimal ID. There is no duplicate loot/item database, no item-based
measure and no database version to claim. Integrating the real provider later
also needs an explicit import-gate change; cfg currently imports only model.

## Verification and layout rulings

The authoritative PHASE1-BRIEF uses `specs.json` and separate candidate/enabled
files. PLAN 3.2 still names per-profile files and a combined `encounters.json`;
the brief's layout is used. Its no-talent-table ruling takes precedence over
PLAN row R5's prospective talent-table slot. No placeholder file is shipped.

`test/raid/cfg/module.json` is discovered by the existing CI Go `--all` step and
uses race detection. Its `inputs: ["rules/raid/"]` selects cfg and dependents for
data-only changes. A new unclaimed raid data path selects all modules rather
than silently selecting none. Mutants remain scoped to Go source, not inputs.

The four `TestReload*` tests edit real files, reload in the same running binary,
re-score a stored synthetic input through a small independent test consumer,
and read the persisted result back. Each is also challenged by a caching mutant.
This verifies R5's no-rebuild contract, **not an integration test of R7**, which
is absent on this branch; R7 must repeat these four cases through its full engine.
Native Windows/game tests and full desktop CI are not part of this lane.
