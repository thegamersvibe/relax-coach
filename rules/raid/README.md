# Relax Raid configuration (R5)

`cfg.Load(dir)` delegates to `cfg.LoadFS(os.DirFS(dir))`; both read all nine
JSON files on **every call**, without caching or writing. They return
`(*cfg.Config, error)` and no partial config on error. Library callers must not
edit a source filesystem while a snapshot is loading.

For raid 1 the app uses only the rules compiled into the exe: one app version
means one config. There is no override beside the exe, in app data, or through
an update channel. After editing these canonical JSON files, run
`go generate` in `desktop/raidrules/` (or `python3 scripts/gen_raid_rules.py` from the repo
root), then commit the generated `desktop/raidrules/raid_rules/` copy. The normal Go test
suite compares every byte and the complete embedded file inventory before any
regeneration; stale, missing and extra files fail CI. The app accessor loads
once, retains any error for the Raids page, and hands callers detached snapshots.
A tuning change therefore needs an app release until a separate validated rule
update channel exists.

`Load` exposes a detached public view and retains a private validated snapshot.
`Profile`, `Encounter`, and `OutputTargets` return deep copies, including nested
slices and optional spell sources. `c.Snapshot()` returns a fresh complete copy
(including scoring maps); mutating any returned value never changes the next
lookup/snapshot. To apply a configuration change, edit its JSON and reload rather
than mutate a returned view.

## Version and validation contract

Like `rules/rules.json`, `schema` is the reader format and `version` is the
content revision. `scoring.json` requires schema 2; the other eight files require
schema 1. Any other schema is refused, including scoring schema 1 (no implicit
migration or defaults). Schema 2 accounts for the newly required energy GCD,
provisional coverage, four death-predicate fields, the bounded-aggro duration and the cast-start cap. Nothing has been released. A newer
`major.minor.patch` **content** version is accepted without rebuilding, provided
its schema is supported. Every file requires a source note. Bump content versions
when tuning data; bump schema for incompatible field changes.

Unknown keys (also nested), case-folded aliases, duplicate keys, missing required
fields, null nonoptional values, malformed/trailing JSON, invalid ranges,
duplicate identities and broken profile/support references are rejected with
file and field context (including nested JSON paths). Invalid UTF-8 is rejected
before JSON decoding can silently replace it. Duplicate ids/keys are compared
with lowercase/trim-space normalization; this does not rewrite canonical ids or
make lookups fuzzy. Distinct spell names cannot share a nonzero spell id, even
across catalogues. Empty arrays and explicit false/zero values are not missing data.
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
  creature type, never a per-pull GUID. A selector requires `id > 0`;
  `Name` is an optional display/comment label, never matched. Use
  `selector.Matches(npcID)` for ID-only matching, including localized logs.
  Repeating an NPC id within an encounter's boss selectors is refused.
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

- Scoring 1.5.0 adds `Scoring.ActivityCastCapMs int64` (`activity_cast_cap_ms`):
  10000 ms, the previous completed-cast cap moved into data by the round-2
  ruling. Required, strictly positive, non-null; fractional/string values are
  refused. Snapshot copies this scalar by value. A matching start at the cap
  is included; one beyond it is not. No compiled-in fallback.


- `Scoring.Weights RoleWeights { DPS, Healer, Tank PartWeights }`;
  `PartWeights { Output, Activity, Mechanics, Utility, Preparation float64 }`.
- `Scoring.Night NightWeights { TrashShare, MaxTrashShare, MinBossShare,
  KillWeight, WipeWeight float64 }`.
- `Scoring.Confidence Confidence { MinCoverage, ProvisionalMinCoverage float64;
  LowConfidenceFightMs int64; ShortFightWeight float64;
  MinBaselinePlayers, MinMeasurablePlayers int }` (3 and 2; a sole player is N/A).
  `MinCoverage` stays 0.6; `ProvisionalMinCoverage` is 0.15. At or above 0.6 a
  raid night can receive a normal score/rank; at or above 0.15 but below 0.6 it
  receives an unranked provisional score, labelled with measured coverage.
  Below 0.15, or without a scored fight, the night score is null. The same
  coverage thresholds apply per fight (which has no rank). Dungeon runs still
  have no rank. These are consumer rules, not a scorer implemented in cfg.
- `Scoring.RatioPoints []RatioPoint { Ratio, Score float64 }` stores
  0.5→0, 1→70, 1.3→100. `CountStartScore float64` = 100.
- `Scoring.Healer Healer { TeamShare, IndividualShare, UnhealedTeamScore,
  UnhealedOutputCap float64; LowHealthFraction *float64 }` stores the half/half
  split and the unhealed-death guard. The health cutoff is 0.5, an architect-ruled
  starting guess, not a calibrated threshold.
- `Scoring.Death Death { WindowMs int64; WipeDeadShare float64;
  WipeAllTanksDead, WipeAllHealersDead bool; OneShotWindowMs, UnhealedMinMs int64;
  LostBossGapMs, SummaryWindowMs *int64 }` holds DESIGN 8/9 values for R6 too.
  `LostBossGapMs` is 4000 (starting guess for rule 4); `SummaryWindowMs` is
  10000, matching `WindowMs`, for deaths following the first death in a wipe.
- `Scoring.Toggles Toggles { RequiredConsumables, Readiness, SupportCredit,
  History bool }` starts all off. `Assignments []Assignment` holds the five
  future jobs and 15-point output-to-utility transfers. Neither implements
  phase-2 behavior in cfg.
- `NewAt60Seconds, GCDMs, GCDEnergyMs *int64`, `RoundingStep *float64`,
  `MeasureWeights map[string]map[string]float64`,
  `CountPenalties map[string]float64` expose data, with no compiled-in defaults.
  `GCDMs` is 1500; `GCDEnergyMs` is 1000 and applies when a cast's `PowerType`
  is 3. `RoundingStep` is 0.1. `NewAt60Seconds` stays null: it is a prep-report
  label, not a score. All these JSON keys are required; the pointer values,
  including `gcd_energy_ms`, may explicitly be null to mean unconfigured.
  Consumers must leave the affected measure unmeasured, not substitute zero
  or treat it as fully measured. The shipped GCD/rounding values are non-null.

### Architect-ruled raid-1 data (scoring 1.2.0 and 1.3.0)

The PHASE1-BRIEF section **R6 and R7 on main 6208b16** supplies the starting
values; **Raid 1 gets a score** supersedes its tank-output, preparation and
provisional-score rules. No role-part weights, healer shares or phase-2 toggles
change. The JSON `source` distinguishes measured evidence, rulings and guesses.

| JSON field | Value | Source / status |
|---|---|---|
| `rounding_step` | 0.1 | PLAN 4A row 7 |
| `gcd_ms` | 1500 | Architect reports second-log cast-gap peak at 1.4–1.5 s |
| `gcd_energy_ms` | 1000 | Second peak at 0.9–1.0 s; cast PowerType 3 |
| `healer.low_health_fraction` | 0.5 | Starting guess |
| `death.lost_boss_gap_ms` | 4000 | Starting guess |
| `death.summary_window_ms` | 10000 | Same as the death window |
| `confidence.provisional_min_coverage` | 0.15 | Raid-1 architect ruling, starting value |
| `count_penalties.owned_death` | 40 | Starting guess |
| `count_penalties.boss_swing_taken` | 2 | Starting guess |
| `count_penalties.better_item_in_bags` | 25 | Raid-1 architect ruling, per better-gear slot |
| `new_at_60_seconds` | null (unchanged) | Prep-report label, not scored |

Exact `measure_weights` (original measures are guesses; preparation weights are
the newer architect ruling; none are calibrated):

| Part | Measure weights | Phase-1 consumer meaning |
|---|---|---|
| `output` | `boss_damage: 100` | DPS **and tanks**, each against their own role's median; two tanks are provisional, one is unmeasured |
| `activity` | `active_time: 100` | Cast/swing activity against the role median |
| `mechanics` | `deaths: 50`, `avoidable: 30`, `boss_swings: 20` | Avoidable is unmeasured until mechanics are enabled; tanks have no boss-swings measure |
| `preparation` | `stock: 30`, `bag_gear: 30`, `used: 40` | Prep-report stock and better gear; used is unmeasured until a consumable aura map exists |

Utility is absent from the map and remains unmeasured (interrupts/dispels are
evidence only). Healer output uses the existing team/individual shares, not a
new damage measure. No separate tank-output key or weight is added.
Preparation requires a report: potion and food stock contribute 50 each;
flasks/elixirs are evidence only. Bag gear subtracts the configured 25 per
better-item slot, floors at zero, and is unmeasured if unchecked or unreadable;
not-scored slots are skipped. With both stock and bag gear measurable,
preparation coverage is 0.6, not 1. A weight is not evidence of measurability.
The app-side prep reader and scoring/attribution belong to later lanes.

### R6 predicate data (scoring 1.3.0)

The PHASE1-BRIEF **R6: four predicates DESIGN 8 left open** ruling adds four
required, non-null fields under `death`. All four are architect guesses to tune
after raid 1, not log-calibrated values:

| JSON key | Go field/type | Value | Validation |
|---|---|---|---|
| `full_health_fraction` | `FullHealthFraction float64` | 0.9 | `(0,1]` |
| `min_health_samples` | `MinHealthSamples int` | 2 | integer >= 1, bounded by Go int |
| `raidwide_share` | `RaidwideShare float64` | 0.6 | `(0,1]` |
| `raidwide_window_ms` | `RaidwideWindowMs int64` | 2000 | `0 < raidwide_window_ms <= window_ms` |

Existing reflection decoding enforces exact keys, types, required values and
file/field errors. `Snapshot()` and Load views copy these scalar fields by value.
A changed value requires reloading the files, not rebuilding the consumer.
R6 uses the full-health cutoff to start the unhealed span, the sample minimum to
withhold attribution, and the raid-wide share/window to test distinct players
hit around the killing hit. Pets belong in neither side of that share.

Version 1.3.0 also follows **R7: three gaps found by the builder** and PLAN 4A
row 15: `confidence.provisional_min_coverage` is 0.15. A lone
tank's measurable coverage can now qualify without inventing a peer baseline.
The validation rule stays `0 < provisional_min_coverage < min_coverage`.

### R6 bounded aggro (scoring 1.4.0)

PHASE1-BRIEF **R6: rules 3 and 4 bounded** adds required, non-null
`death.pulled_aggro_ms` (`PulledAggroMs int64`), starting at 10000. This is an
architect guess to tune after raid 1, not a measured threshold. It must be
positive; the typed decoder rejects fractions and int64 overflow. `Snapshot()`
copies the scalar by value. Rule 3 bounds the time from the victim's first
damage on the boss to the start of its killing run with this value, rather
than blaming an early opener for a later unrelated death.

### Validation bounds

- `instances.raid_group_size_above >= 5`: five is the dungeon-size cutoff;
  equality is valid, and only a group strictly above the configured cutoff is
  inferred as a raid. Larger configured cutoffs remain valid.
- `0 <= fights.tail_ms <= fights.pull_gap_ms`, with a positive pull gap: a
  post-combat tail cannot bridge more than the gap that separates pulls.
- Role-part weights and inner measure weights are in `[0, 100]`, matching the
  existing 100-point weighting/score semantics. Role parts and each configured
  inner measure set must sum to 100, using the same floating-point tolerance.
  An absent part (or an empty outer map) remains unconfigured, not implicit
  equal weights; a present part with an empty inner map is refused.
- Night kill weight is in `(0, 1]`, and wipe weight in `[0, kill_weight)`:
  the existing full-kill contribution is the unit reference, while a wipe
  contributes strictly less (the pre-existing kill/wipe distinction). This is a guardrail, not a new tuned seed value.
  Short-fight weights, shares and fractions stay in `[0, 1]` (positive where
  already required); role scores, penalties and point transfers stay in `[0, 100]`.
- Ratio points have strictly increasing ratios and nondecreasing scores.
- A configured `rounding_step` is in `(0, 100]` score points: 100 is the entire
  score scale, so a larger quantum exceeds that scale rather than expressing
  useful score precision. Null stays unconfigured; this bound chooses no default.
- `0 < confidence.provisional_min_coverage < confidence.min_coverage`;
  the new provisional threshold is required and non-null, and compares against
  the configured normal threshold rather than a hardcoded 0.6.
- Non-null `gcd_ms`, `gcd_energy_ms`, `death.lost_boss_gap_ms` and
  `death.summary_window_ms` are positive `int64` milliseconds. Their upper
  representation bound is `9223372036854775807`; overflow is refused by the
  typed decoder. No unsourced gameplay ceiling or equality to the starting
  value is imposed. `rounding_step` is in `(0, 100]` and
  `healer.low_health_fraction` in `(0, 1]` when configured. All configured
  measure weights and count penalties remain in `[0, 100]`.

These bounds preserve the fractional-versus-point distinction in `scoring.json`
and its DESIGN 7/9 source; they do not supply compiled-in weight defaults.

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
Amethrax from resemblance. The enabled registry holds encounter 478 (Balnazzar)
with sourced NPC selectors and no enabled mechanics. Enabling a
mechanic requires affected-player and valid-response descriptions plus
`EvidenceSource SpellSource` with kind `real_log`; datamining alone is refused.
Unknown-ability reports belong to the event consumers, not the loader.

Per PHASE1-BRIEF **R6: four predicates DESIGN 8 left open**, rule 3, an empty
mechanic `roles: []` means **every role**. A nonempty list restricts applicability
to its named roles (`dps`, `healer`, `tank`) and must not contain duplicates.
The encounters schema still requires the `roles` key and a non-null array; an
absent key or explicit null is refused, not interpreted as all roles. Profile
roles remain nonempty; this relaxation is specific to mechanics.

Support `Stacks *bool` describes additive copies from **different providers**,
not a single aura's charges or Sunder stack count. It is null where unverified;
no cross-family stacking behavior is invented. Every effect also has a required
`Verified bool` (`verified`): a conservative source-evidence gate for documented
scope and known same-effect stacking, not a claim of runtime validation. Every
seed marked unverified/unknown, including Mana Spring, Mana Tide and Healing
Stream, has `verified=false` and `credit=false`. This also withholds credit where
cross-provider stacking is still unverified. Loading refuses `credit=true` with
`verified=false`; `verified=true` is necessary, not sufficient, for credit.
The seed's 50 unverified effects receive no credit; 15 have documented
classifications, of which seven are credit-eligible and eight are personal.
Personal effects never earn
support credit. Furious Howl has conflicting party versus party/raid scope in
the two study reports: its scope is `unknown` and credit is disabled pending
source resolution. A phase-2 consumer must not assume additive credit from null
stacking. Loading also refuses `credit=true` with `stacks=null`: both explicit
`false` (non-stacking) and `true` are stack rules, but null is not. Unknown and
personal scope remain ineligible even with `verified=true` and known stacking.
Consumables are categories only; requirements and item mappings need
separate evidence.

The raid-1 rulings now supply the inner-measure weights, penalties, low-health
cutoff, GCD durations, rounding step, provisional coverage, lost-boss gap and
wipe-summary window listed above. These are replaceable starting data, not
validated numerical benchmarks. `new_at_60_seconds` remains null by ruling;
utility, consumable-use mappings and phase-2 history/support credit remain
unmeasured or disabled, never filled from builder memory.

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

Round-1 changes, study-line spell corrections, test-first red/green runs, all
reviewer hand-survivor mappings, mutation counts and the actual final verification
output are recorded in [R5 review evidence](../../docs/raid/R5-REVIEW-EVIDENCE.md).
