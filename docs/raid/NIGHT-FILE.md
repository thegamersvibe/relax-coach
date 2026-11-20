# Night result file, format version 1

The one file the Relax Coach app gives the Guild Logs site per raid night (REQ-WEB-01).
Sources: DESIGN.md 10.2; PLAN.md section 4, items 4, 6, 7 and 8; PLAN.md section 4A (rows 1 to 19),
which settles the open field points. The site's upload route (W2) and pages are written against this document. Changing
a field's meaning, or adding a field or a flag, raises `formatVersion`.

## General rules

- JSON, one object, always in the canonical form below. File name `night-YYYYMMDD-<first 8 of nightId>.json`; the date is the
  local date of the first fight's start. A re-score writes the same name and replaces the file.
- `formatVersion` starts at 1. A reader refuses a file whose `formatVersion` is newer than it knows,
  and refuses a flag or an enumeration value it does not know.
- `kind` is `raid`. A dungeon run has its own file (last section).
- One file covers one run. Leaving and returning to the same instance continues that run only when
  zone-out to zone-in is strictly under the configured re-entry gap (default 30 minutes) and no
  different non-zero instance was entered in between. A header starts a new run. Outside lines
  belong to no fight; each exit closes the current fight/tail. The run's end boundary remains its
  final zone-out timestamp (the first zone-out if it never returns), not a later outside line or
  repeated outside-zone change. Empty instance visits produce no run file; visits with fights do.
  All complete boss fights on both sides of an accepted return contribute to the same `nightId`.
- Players are keyed by the character name **exactly as the log wrote it**, the whole string. The file
  has **no `siteCharacterId` and no `siteName` field**: the site links names when a file is uploaded
  (PLAN.md 6.1, 6.2), and the app has no site client before W6.
- A value that is unknown or could not be measured is `null`, **never 0**. Coverage is the exception:
  it is always the measured number (see "Not enough data"). A field marked "absent"
  below does not exist in the file; it is not `null`.
- No raw events. Evidence is a summary (below); the full event lists stay in the app.
- Times: the night start and each fight start are a **time pair** `{"timeMs": int, "offsetH": int}`:
  local wall time as the log wrote it, in milliseconds since the Unix epoch, and the writer's
  whole-hour UTC offset. Every time inside a fight is milliseconds from that fight's start.
- Numbers: part, fight and night scores are 0 to 100 with exactly one decimal place (`87.0`, not
  `87`). Coverage is 0 to 1 with exactly two (`0.75`). Rounding is half away from zero, in integer
  arithmetic, so the same input gives the same bytes. Counts and times are integers.

## Identity and revision

| Field | Type | Meaning |
|---|---|---|
| `nightId` | string, 64 lowercase hex | SHA-256 of the sorted ids of complete boss fights joined by a newline (`\n`, no trailing newline). Trash pulls and partial fights are excluded. Says *which night*. |
| `contentHash` | string, 64 lowercase hex | SHA-256 of the file's bytes with the 64 characters of the `contentHash` value replaced by 64 zeros (`0`). Says *which scoring of it*. |

The file on disk is always canonical (next section), so a reader checks `contentHash` on the raw bytes
and never re-writes the file. On upload: same `nightId` and same `contentHash` changes nothing. Same `nightId` with a
different `contentHash` is a re-score: it replaces the stored result, returns an approved night to
"uploaded" and shows what changed (REQ-WEB-05). Two loggers who saw the same complete boss fights get the same boss ids and `nightId`, despite
clock drift, different trash and logger-only failed casts. A missing/partial boss changes `nightId`;
the site resolves overlapping nights. Phase 1 scores one source log; later merging is PLAN 6B.

A complete boss has both encounter lines. Its id is lowercase hex SHA-256 of these
newline-joined values (no trailing newline): `boss`, instance id, encounter id, difficulty id,
group size, end-line success, end-line fight length in milliseconds, then the distinct player
ids from its `COMBATANT_INFO` lines in ascending byte order. Numbers are decimal. No timestamp,
failed cast, or other logger-specific event enters this hash.

When complete boss fights in the same file have every input above equal, the
second and later in file order append one additional newline-separated value:
`repeat 2`, `repeat 3`, and so on. The first appends nothing, preserving existing
ids. The counter is file-local, crosses headers/run boundaries, and counts only
complete bosses. A different server fight length (even by one millisecond) is a
different input, not a repeat. `nightId` still hashes the sorted complete ids,
including repeats. Reading the same file twice gives the same ids; a file that
contains only the second pull necessarily gives it the original, unsuffixed id.
No timestamp is added to the hash.

Trash and partial fights have file-local ids: the preceding complete boss id (or `start`),
a colon, and the one-based position after it. They are not matched between logs. Missing
server numeric fields are `unknown`, never an invented zero. The normal complete boss
contract requires all these fields; a malformed decoder must report the missing field.

## Canonical form

The writer emits exactly this, and it is the only form the file has:

- UTF-8, no byte-order mark, no trailing newline. No whitespace between tokens.
- Object keys in ascending order of their UTF-8 bytes; arrays in the order given.
- Strings: the bytes as logged, no Unicode normalisation. Only `"` and `\` are escaped (as `\"` and
  `\\`), and control characters below U+0020 as `\u00xx` with lower-case hex. Everything else,
  including all non-ASCII, `<`, `>`, `&`, U+2028 and U+2029, is written raw.
- Integers: digits with an optional leading `-`, no leading zeros, no `-0`.
- Scores and coverage: the fixed decimals above (`87.0`, `0.75`). No exponent form anywhere.
- `true`, `false`, `null` in lower case.

Go's `encoding/json` does not produce this (it escapes U+2028, U+2029 and, by default, `<`, `>`, `&`,
and does not write `87.0`), so the writer is its own small one, with a test per rule.

## Top level

| Field | Type | Required | Meaning |
|---|---|---|---|
| `formatVersion` | integer | yes | `1`. |
| `kind` | string | yes | `raid`. |
| `nightId` | string | yes | See above. |
| `contentHash` | string | yes | See above. |
| `configVersion` | string | yes | Version of the scoring configuration used (REQ-PAR-07, REQ-SCO-10). |
| `dataVersion` | string | optional | Shared item database release tag, present only when a measure read it; otherwise absent (4A row 18). |
| `sourceLog` | object | optional | `{sha256, bytes}` of the entire unchanged raw log, including lines outside runs. No path or PC identifier (4A row 19). |
| `gameBuild` | string or null | yes | Game build from the log header, as written; `null` if the log has none. |
| `raid` | object | yes | `{"name": string, "instanceId": integer or null}`: the instance as the log names it. |
| `start` | time pair | yes | Start of the first fight. |
| `loggers` | array of strings | yes | Characters whose logs this file was built from, names as logged. |
| `fights` | array of fight | yes | In start order. |
| `players` | object: name → player | yes | One entry per roster member. Each entry stands alone, so the site can serve one raider's part without the rest. |
| `wipes` | array of wipe summary | yes | One per wiped boss fight; empty when there were none. |
| `assignments` | array of assignment | yes | Empty when none were set. |
| `softReserve` | | absent | Does not exist in format version 1. |

`dataVersion` and `sourceLog` are optional additive fields: `formatVersion` remains 1.
The parser retains each fight's raw byte range from its first line's `Offset` through
its last line's `Offset + Len`, including the final line ending. Those ranges are
local indexing metadata, not raw events added to the night file.

### Local segmentation report (not serialized)

`fights.Result` exposes per-source-file counters, separate from this per-run
night/dungeon format:

- `PlayerDeathsOutsideRuns` ("player deaths outside runs"): non-feign player
  death lines outside every returned run's inclusive `[StartMs, EndMs]`. They
  are out of scope: not reviewed and not scored.
- `PlayerDeathsInRunsWithoutFight`: non-feign player death lines inside a
  returned run's span but assigned to no fight. Expected zero; a nonzero count
  is a segmentation diagnostic, not a death cause or a score adjustment.

Both require a `Player-` destination GUID and `model.Event.IsDeath()`. Fight
membership includes its retained post-end tail. Run-span membership uses the
final boundaries, including accepted re-entry gaps; it does not change which
outside lines are excluded from fight events. These counters are available in
Build and streaming Read, including index-only reads. They are not copied into
`model.RaidNight`, `model.DungeonRun` or this JSON format; `formatVersion` stays 1.

## Fight

| Field | Type | Required | Meaning |
|---|---|---|---|
| `id` | string | yes | Complete-boss hash or file-local id, as above. |
| `type` | string | yes | `boss` (bounded by its two encounter lines) or `pull`. |
| `name` | string | yes | Encounter name for a boss, the main enemy's name as logged for a pull. |
| `encounterId` | integer or null | yes | From the encounter line; `null` for a pull. |
| `start` | time pair | yes | Fight start. |
| `durationMs` | integer or null | yes | Boss: the end line's server fight length, never timestamp subtraction; `null` without that value. Pull: local end minus start. The configurable post-end tail belongs to the fight's events, not its server duration. |
| `outcome` | string or null | yes | `kill` or `wipe` from the end line's success field; `null` for a pull or a fight without an end line. |
| `partial` | boolean | yes | The log starts or ends inside the fight. |

## Player (`players[<name as logged>]`)

| Field | Type | Required | Meaning |
|---|---|---|---|
| `class` | string or null | yes | Class token as the game gives it, e.g. `WARRIOR`. |
| `flags` | array of flag | yes | Night-level flags for this player (`newAt60`, `differentBuild`, `uncalibratedSpec`). |
| `night` | object | yes | `{"score": number or null, "coverage": number, "rank": integer or null}`. |
| `fights` | object: fight id → player fight | yes | Only fights the player took part in. |
| `deaths` | array of death | yes | This player's deaths, in time order. |

`rank` starts at 1 within the night, over non-provisional players with a night score. Equal scores share a rank
and the next rank is skipped (1, 1, 3). Rank compares the stored, rounded score, so the file and the
page cannot disagree.

**Raid-one provisional rule (scoring 1.3.0, schema 2).** Coverage is always the
measured number, never `null`; nothing measurable is `0.00`.

- Coverage at or above `confidence.min_coverage` (0.60): normal score and, in a
  raid, a rank. A provisional *part* does not by itself exclude a normal night.
- At or above `confidence.provisional_min_coverage` (0.15), but below 0.60:
  `score` is set and `rank` is null. Show "provisional, NN% measured" separately
  from ranked players. These players do not consume a rank position.
- Below 0.15, or with no scored fight: `score` and `rank` are null. A reader
  recognises "not enough data" by `score` being null.

The same coverage thresholds apply to each fight, which has no rank. Scores
are rounded half away from zero to the configured step inside `score`, not by
the writer. Eligibility compares exact, unrounded coverage with the exact
configured threshold at both levels; public coverage is rounded DOWN to whole
hundredths. Thus 14.73 stores 14 and has no score, 15.0 has a score, 59.90
stores 59 and has no rank, and 60.0 can rank. Dungeon scores never have ranks, including normal
scores. Index-only fights (empty `Events`), unknown runs, and players missing
from the fight's roles are unscored. A roster-only player with no event evidence
inside the scoring interval is unmeasured, not idle or a zero-valued peer. An
observed unsuccessful action may still yield a measured zero. Partial fights retain a `partialFight`
row but no score. Boss confidence/time basis uses the server `DurationMs`;
trash uses end minus start. Missing duration is not guessed from boss clocks.

Night means use kill/wipe weights, the short-fight multiplier, and fight
exact (not stored/floored) coverage. Night coverage averages exact coverage with kill/wipe/short weights
without applying coverage a second time. Boss and trash groups are combined
only within their configured shares. No history, spec comparison, assignment,
support-credit or readiness scoring is enabled by this phase-one scorer.

## Player fight (`players[<name>].fights[<fight id>]`)

| Field | Type | Required | Meaning |
|---|---|---|---|
| `specId` | string or null | yes | `<CLASS>.<spec>` for this fight: the class token as the game gives it, a dot, and the spec id from the spec list, e.g. `PALADIN.holy`. The 33 entries of the spec list are 33 distinct pairs (only 30 spec words are distinct: `protection`, `holy` and `restoration` occur under two classes). The same string names the profile file, `rules/raid/specs/PALADIN.holy.json`. `null` when the spec is unknown. |
| `role` | string or null | yes | `tank`, `healer` or `dps` in this fight. |
| `score` | number or null | yes | Fight score: the weighted sum of the measurable parts for the role (REQ-SCO-01, 02). |
| `coverage` | number | yes | Share of weight that was measurable; always a number (see "Not enough data"). |
| `flags` | array of flag | yes | Flags for this fight. |
| `parts` | object | yes | Exactly five keys: `output`, `activity`, `mechanics` (mechanics and survival), `utility` (utility and support), `preparation`. Each is a part, or `null` when the part could not be measured. |

## Part

| Field | Type | Required | Meaning |
|---|---|---|---|
| `score` | number | yes | 0 to 100, one decimal. |
| `coverage` | number | yes | Share of the part's inner weight that was measurable. |
| `baseline` | string or null | yes | For `output` and `activity`: which baseline was used, `own` (the player's history on that boss and spec), `guild` (the guild's same-spec history on that boss) or `role` (the same role in this fight) (REQ-SCO-12). `null` for the other parts. |
| `provisional` | boolean | yes | Fewer baseline samples than the configured minimum. `false` for parts without a baseline. |
| `evidence` | evidence | yes | Why the part has this score. |

These existing file fields are carried directly by `model.Part` (model addendum 3),
not by reserved keys in `Evidence.Counts`:

- `Coverage int`: hundredths, 0 to 100 inclusive, rounded down from exact measured
  weight. Measured parts require 1..100; a part below one hundredth remains
  unmeasured rather than inventing upward-rounded coverage. The writer formats
  it as `0.00` to `1.00` without rounding again; for example, 75 becomes `0.75`.
- `Provisional bool`: fewer baseline samples than the configured minimum.
  It is `false` when not set, including parts without a baseline.
- `Baseline string`: exactly `"own"`, `"guild"`, `"role"` or `""`. The empty
  string means not set and is written as `null`.

The model's zero values (`0`, `false`, `""`) mean not set. Coverage has no separate
presence bit; `Tenths` remains the optional score that distinguishes an unmeasured
part from a measured score of zero. This is an additive in-memory model change,
not a new file field or a change to `formatVersion`.

**Publication guard.** `score.ValidatePart` refuses a measured part with coverage
outside 1..100, score tenths outside 0..1000, absent/blank evidence text or an
absent counts object, text over 200 characters, more than 20 evidence times, a
baseline outside `""`, `own`, `guild`, `role`, or an output/activity part with an
empty baseline. An empty counts object and zero evidence times are valid. `score.Validate` walks the complete result and
`Score` calls it before returning. The future app/writer boundary must retain
this check; the `result` package's model-only import boundary is unchanged.

### Phase-one measures

Only same-role-in-this-fight baselines are available, never spec or history.
Output and activity need at least two eligible role peers and a positive
baseline. Two peers give a provisional part; one gives no relative measure.
REQ-SCO-13 excludes profiles explicitly flagged `uncalibrated` from all Output
measures and output-peer baselines (including healer halves), not merely their
label. Other parts remain measurable. An unknown spec retains the phase-one
role fallback and `uncalibratedSpec` flag; it is not an explicitly flagged profile.
R6/R7 exchange model cause and owner constants, not file-format cause keys.
DPS and tanks compare effective boss damage per millisecond alive to the role
median (all hostile, unowned creatures for trash). Only landed swing amounts
count; overkill is clamped at zero before subtraction. Pets use `Fight.Owners`.
Healer output combines the team and individual halves: unhealed deaths charge
only healers alive at the death, including the post-end tail; individual credit
is effective healing plus consumed absorbs (the shield's `Caster`) on players
at or below the cutoff in their preceding valid `InfoUnit` HP/HPMax sample.
Unknown health is not low health. The individual share is compared with an
equal share of the healing team's total. A lone healer retains the team half,
not an invented individual baseline. The configured unhealed output cap applies.

**Active time** is the union of completed cast intervals and the cooldown after
each completed cast (`gcd_energy_ms` for cast `PowerType` 3, else `gcd_ms`), plus
a `gcd_ms` interval after each melee swing (landed stream or melee miss) by the
player. Overlaps count once; intervals are clipped to time alive, with real
deaths and resurrections but no feign deaths. Failed/incomplete casts, pet
uptime, auras and RANGE damage/miss events provide no activity. Shots already
have `SPELL_CAST_SUCCESS` lines. A zero role median is unmeasured, not a free 70.
A start older than the known spell cast time must not pair with a success. The
current model/config exposes no cast-time or timeout field, so the fallback
cap is ten seconds, inclusive (`10 * time.Second / time.Millisecond`). A stale
success contributes only its cooldown. A melee miss requires spell ID zero,
empty spell name and distinct source/target; a spell-zero self miss is not a swing.

Mechanics measures owned deaths (50) and non-tank boss swings (20); avoidable
mechanics (30) remain unavailable. Only `OwnerKind=player` with the full logged
name in `Owners` permits a death charge. Unresolved, unhealed and after-wipe
causes are not charged here. The configured death penalty scales linearly from
full at the pull to half at the end. Non-tanks lose the configured penalty per
landed boss swing; scores floor at zero. Utility counts interrupts and dispels
but has no measured score. Idle trash is visible through active/alive evidence;
no new flag is added to the closed format-version-1 flag list.

Preparation uses `MatchPrep(reports, names)`: full logged name first, then ASCII
lowercase only. Ambiguous folded names and unmatched reports score nobody and
are returned to the page. For gear, choose the latest report stamped at or
before fight **end**, or the earliest report of the night if none is eligible.
A same-stamp tie chooses the larger sender clock, then the bytewise greatest
full-report JSON as a stable key; this applies to the earliest fallback too.
Both chosen clocks are retained. Stock (30) gives 50 points each for any readable
report of potion/food stock in the night, not necessarily the chosen gear report.
Flasks and elixirs are evidence only. Bag gear (30) starts at 100 and loses the
configured 25 per distinct better-item slot, skipping `NotScored` slots. Unchecked
or unreadable gear is unmeasured; unreadable stock is unmeasured. Empty readable
bags are a measured zero stock, not missing data. Used consumables (40) are
unmeasurable. No report leaves preparation wholly unmeasured.

The fixture-night coverage table (with / without readable stock and checked gear):
DPS 0.80 / 0.74; two or more healers 0.70 / 0.64; lone healer 0.32 / 0.26;
two or more tanks 0.53 / 0.47; lone tank 0.23 / 0.17. Measurable peer output and
activity are prerequisites for the non-lone rows. All rows floor coverage; lone
rows retain a provisional night score without a rank.

## Evidence

| Field | Type | Required | Meaning |
|---|---|---|---|
| `text` | string | yes | One plain English sentence, at most 200 characters, e.g. "No cast for 38 seconds from 2:10 in the add phase." |
| `counts` | object: string → integer | yes | Short name to count, e.g. `{"interrupts": 3}`. May be empty. |
| `atMs` | array of integers | yes | At most 20 times, milliseconds from the fight start. May be empty. |

### Score evidence count keys

Every role measure has `measured.<measure>` equal to 0 or 1. Only measured
measures have `points_tenths.<measure>`, an integer score times ten. The complete
measure names (and therefore the complete suffixes for both key families) are:

| Part / role | Availability key | Points key (only if measured) |
|---|---|---|
| Output DPS/tank | `measured.boss_damage` | `points_tenths.boss_damage` |
| Output healer | `measured.healer_team` | `points_tenths.healer_team` |
| Output healer | `measured.healer_individual` | `points_tenths.healer_individual` |
| Activity | `measured.active_time` | `points_tenths.active_time` |
| Mechanics | `measured.deaths` | `points_tenths.deaths` |
| Mechanics | `measured.avoidable` | `points_tenths.avoidable` |
| Mechanics non-tank only | `measured.boss_swings` | `points_tenths.boss_swings` |
| Preparation | `measured.stock` | `points_tenths.stock` |
| Preparation | `measured.bag_gear` | `points_tenths.bag_gear` |
| Preparation | `measured.used` | `points_tenths.used` |

Utility has no scored measure; its raw counts remain available. `avoidable` and
`used` are always unavailable in phase one, so their points keys are absent.

All additional raw count keys are listed here. Missing keys mean unavailable,
not zero. Reserved part metadata is **not** hidden in these maps.

| Keys | Meaning |
|---|---|
| `boss_damage`, `alive_ms` | Effective boss damage (hostile creature damage in trash), and time alive in milliseconds. Divide for the exact output rate. |
| `active_ms` | Union duration, clipped to alive intervals; divide by `alive_ms` for active share. |
| `baseline_players` | Eligible same-role peer count; also the healer equal-share divisor. |
| `baseline_damage_num`, `baseline_damage_den` | Exact reduced fraction for role-median damage per millisecond alive. May be recorded even when insufficient peers prevent measurement. |
| `baseline_active_num`, `baseline_active_den` | Exact reduced fraction for the role-median active share. |
| `low_health_effective`, `team_low_health_effective` | Player's low-health effective healing plus consumed absorbs, and the healers' total. |
| `unhealed_deaths` | Unhealed deaths at which this healer was alive; controls the team half and output cap. |
| `owned_deaths`, `unresolved_deaths` | Death charges owned by this player, and their unresolved deaths (evidence only). |
| `death_severity_num`, `death_severity_den` | Exact reduced sum of death severity, each `1 - elapsed / (2 * duration)` with elapsed clipped to the fight. Multiply by configured `owned_death` penalty. |
| `boss_swings_taken` | Landed swings from the fight's boss units on this player; no tank measure. |
| `interrupts`, `dispels` | Counts including actions credited through the fight's owner map; not scored. |
| `prep_stamped_s`, `prep_sender_clock_s` | Server receipt stamp and sender clock of the chosen gear report, in seconds. |
| `potion_count`, `food_count`, `flask_count`, `elixir_count` | Maximum summed bag count per category across readable reports, not a sum of repeated reports. Only potion and food presence score. |
| `better_gear_slots`, `not_scored_slots` | Distinct penalized slots and distinct explicitly skipped slots in the chosen report. |

Per-measure `points_tenths` values are rounded for display only. Raw, clamped
measures retain exact fractions through inner weighting and round once at the
part score. Recompute from raw evidence and config, not the rounded display
points. Part coverage stays exact internally through fight and night weighting;
stored floors must not be fed back into those calculations. The healer cap is applied after combining
its measured halves. `atMs` contains up to 20 sorted relative owned-death times;
raw event records are never embedded. The pure scorer returns all part evidence,
including unavailable-measure markers, for the officer page before file formatting.

## Death

| Field | Type | Required | Meaning |
|---|---|---|---|
| `fightId` | string | yes | Fight the death happened in. |
| `atMs` | integer | yes | Milliseconds from the fight start. |
| `cause` | string | yes | `avoidable` (avoidable ability), `unhealed` (unhealed over time), `pulledAggro`, `tankLostBoss`, `unavoidable` (unavoidable or one-shot), `afterWipeCall` or `unresolved` (REQ-DTH-01). |
| `suggestedCause` | string or null | only when `cause` is `unresolved` | One of the six other causes, or `null`. Absent for any other cause. |
| `confidence` | string | yes | `high`, `medium` or `low`. |
| `ownerKind` | string | yes | `player`, `healers`, `nobody` or `undecided` (always `undecided` for `unresolved`). |
| `owners` | array of strings | yes | Names as logged: the one player for `player`; every healer charged for `healers`; empty for `nobody` and `undecided`. A name and a keyword never share a field. |
| `evidence` | evidence | yes | Why this cause. |

## R6 engine contract (phase 1)

`death.Review(fight, runEndMs, config)` returns `[]model.Death`, `[]model.Wipe`
an integer count of excluded real player deaths stamped **after** the final
run end, and an error. `runEndMs <= 0` is an error with no partial results,
including for index-only fights. It writes nothing. Callers pass
`fights.Run.EndMs`, not `Fight.EndMs`.
The latter is an encounter boundary: valid post-end tail deaths can exceed it.
A death at the run end is included. Excluded deaths are never silently dropped;
callers expose the count alongside their diagnostics. No model struct changes.
It reads selected `Fight.Events` plus the roster, roles, owners and boss units.
No events means no judgment, not zero observed deaths. Feign lines and pet deaths
never become player deaths or excluded-player counts. Pet damage is credited
through `Fight.Owners`, never by name. Callers select detail and omit wipe
summaries from dungeon files. `TestRunEndClockSpike` proves that identical Fight
inputs with final run ends 2100 and 60000 yield one and two reviewed deaths;
the first also reports one exclusion, without dropping the valid tail at 2050.

The model uses descriptive cause values; the result writer maps them without
inventing another cause:

| `model.Death.Cause` / nonempty suggestion | File value |
|---|---|
| `avoidable ability` | `avoidable` |
| `unhealed over time` | `unhealed` |
| `pulled aggro` | `pulledAggro` |
| `tank lost the boss` | `tankLostBoss` |
| `unavoidable or one-shot` | `unavoidable` |
| `after the wipe call` | `afterWipeCall` |
| `unresolved` (cause only) | `unresolved` |

The classifier follows DESIGN section 8 order, with the architect's phase-1
predicates. One confident match is `high`; multiple confident rules retain the
first cause with `medium`. No confident match is `unresolved` / `low` /
`undecided`. A sparse unhealed match suggests `unhealed over time` but charges
nobody; otherwise the suggestion is empty. Wipe-call decisions keep priority,
but a second actual match (for example avoidable) makes confidence `medium`.
An unresolved aggro fallback does not count as a second rule match.
Environmental deaths are terminal `unavoidable or one-shot`, or `unresolved`
when an enabled mechanic covers the environmental killing hit; they never
charge the healing team. An absent source alone is **unknown**, not environmental.
The environmental model shape from `logfmt` is no source, no spell id, no swing
stream and an environmental type in `SpellName` (falling, drowning, fatigue,
fire, lava or slime). Unknown types remain unresolved.

- Positional rules use the original `Fight.Events` order, never a timestamp
  sort: this death line, the previous real death, the last preceding boss
  swing and window membership all retain their original event indices. A
  backward clock step anywhere in the fight's log order (including unrelated
  events, before the window or after the death) makes every death in that
  fight `unresolved` / `low` / `undecided`, with empty owners and suggestion;
  no rule runs. Only returned death rows are sorted for presentation.
- The review window begins at death minus `death.window_ms` (inclusive), or
  just after the victim's previous real `UNIT_DIED` line, whichever is later.
  It ends at **this particular death line**, not the end of its millisecond.
  Later same-ms damage, healing, health and survival transitions cannot change
  that death. A resurrection is not the life boundary; feigns are not deaths.
  A health sample belongs to the victim only when `InfoUnit` is the victim and
  present `HPMax` is positive; unrelated destination/source health is not
  substituted. Samples may occur on any event family.
- Unhealed starts at the first victim damage **after** the latest sample at or
  above `full_health_fraction`, otherwise the first victim damage in the
  window. Equal timestamps retain event order. The span must reach
  `unhealed_min_ms`, have zero effective healing and consumed absorbs, and
  have a healer alive throughout. Fewer than `min_health_samples` gives only
  a suggestion. A match names the entire healing team alive at death, not
  one selected healer. Death/resurrection transitions determine survival;
  feign death does not.
- Killing damage is the last landed damage to the victim. A majority spell
  contributes **more than half** of window damage after removing nonnegative
  overkill; `-1` overkill does not add damage. Source-side swings are excluded
  from totals. Self-source debuffs remain eligible mechanic evidence, and
  death-time aura removal cannot erase a spell/role match.
- An avoidable match needs an enabled mechanic for this encounter, the
  killing or majority spell id, `avoidable`, and the victim's role. Empty
  `Roles` means all roles; `AffectedPlayers` and `ValidResponse` are display
  text only. Candidate/research entries cannot assign blame.
- Raid-wide candidates are non-swing damage from hostile creatures, not pets
  or guardians; an enabled avoidable mechanic excludes the spell regardless
  of its role list. Count distinct player targets of that spell from any
  creature inside the inclusive `raidwide_window_ms` interval on either side
  of the killing hit. Divide by players alive at interval start (victim
  included), never group size or pets. At least three must be hit and the
  share must reach `raidwide_share`. A simultaneous unhealed match makes this
  `medium`. One-shot damage must reach the victim's sampled maximum health
  inside `one_shot_window_ms` before death.
- Both aggro rules require a **known** non-tank role (`dps` or `healer`) and
  killing damage that is a landed swing by one of `BossUnits`. Missing or
  uncomputed roles do not prove non-tank status. The killing run starts at
  that boss unit's first landed/missed/dodged/parried swing on the victim
  inside the life-scoped window; source-side swing duplicates do not count.
- Pulled aggro requires the victim's first damage to that unit **in the fight**
  to precede every tank's first damage to it. The run must start at or after
  that damage and within `death.pulled_aggro_ms` (inclusive). The range check
  is isolated in `pulledAggroInRange`; no hardcoded lookback is supplied.
  A passive victim cannot be blamed. The no-living-tank alternative is removed;
  all tanks dead belongs to the configured wipe rule. A tank's earlier damage
  is not erased because that tank subsequently died.
- Lost boss requires an observed holder: the destination of this boss unit's
  last swing **before** the run in stable log order, including missed swings
  and an earlier same-ms line. That swing must be no older than `window_ms`
  before the run. Its destination must be a tank alive at run start, not at
  the killing hit. Only that tank can own the death. His last damage on that
  boss must precede the run by **more than** `lost_boss_gap_ms`; no damage counts
  as more. A damage event at the turn millisecond counts (unless after this
  death line). A tank's missed attacks do not refresh this damage-only clock;
  the architect's damage-event definition supersedes the earlier threat idea.
  Two tanks' hit times cannot choose the holder; a prior non-tank target or no
  recent target gives no match. Ambiguous boss-melee deaths remain `unresolved`,
  `undecided`, with **empty** suggestion and boss-swing evidence. R8 may label
  them "aggro: puller or tank"; this is not a new model/file enumeration.

`Death.TimeMs` and its `Evidence` times are absolute model milliseconds. The
writer subtracts `Fight.StartMs` for file times. Recap evidence retains the
last ten victim damage/heal/absorb times in the window; aggro evidence keeps
only the **most recent ten** landed boss swings on the victim. These references
point into selected events, not raw events in this file. Equal timestamps can
refer to multiple lines. `TestRecapBossKeepsRecentTen` pins twelve swings to the
last ten. [R6 evidence](R6-REVIEW-EVIDENCE.md) records the checks.
R8/result rendering and officer override/tagging remain separate lanes.

## Wipe summary (`wipes[]`)

| Field | Type | Required | Meaning |
|---|---|---|---|
| `fightId` | string | yes | The wiped boss fight. |
| `firstDeath` | object or null | yes | `{"player": name as logged, "atMs": integer, "cause": cause}`; `null` if nobody died before the end line. |
| `following` | array of objects | yes | Deaths inside the configured window after the first, same shape, in time order (REQ-DTH-06). |

The default wipe call is the first observed moment the configured dead share
is reached, or all members of a nonempty tank/healer role are dead when that
role toggle is enabled. Deaths strictly later than the call have no owner;
deaths at the call time are still reviewed normally. Resurrections before
that first moment affect the count; they do not retract an established call.
A summary is returned only for a boss with explicit failure (`Success = 0`),
not a kill, unknown outcome or trash. `model.Wipe.FirstDeath` is a zero-based
index into that fight's returned deaths; `-1` means no death before/on the
encounter end. `Following` excludes the first death and includes deaths at
the inclusive `summary_window_ms` edge. A null configured summary window is
not replaced by a guessed default.

## Assignment (`assignments[]`)

| Field | Type | Required | Meaning |
|---|---|---|---|
| `player` | string | yes | Name as logged. |
| `fightId` | string | yes | The fight it applies to. |
| `fromMs` | integer or null | yes | Start of the interval, from the fight start; `null` for the whole fight. |
| `toMs` | integer or null | yes | End of the interval; `null` for the whole fight. |
| `job` | string | yes | `offTank`, `kiter`, `interrupter` or `decurser` (decurser or dispeller) (DESIGN.md 7.5). |
| `result` | string or null | yes | For `kiter`: the officer's mark, `done` or `failed`; `null` until marked and for other jobs. |
| `setBy` | string | yes | The officer who set it, character name as in the app. |

## Flags

A closed list. A reader refuses a flag it does not know; adding one raises `formatVersion`.

| Flag | Where | Meaning |
|---|---|---|
| `lowConfidenceFight` | player fight | The fight lasted under 45 seconds. |
| `partialFight` | player fight | The fight is `partial`. |
| `uncalibratedSpec` | player, player fight | The spec has no calibrated output baseline (REQ-SCO-13). |
| `newAt60` | player | New at level 60. |
| `differentBuild` | player, player fight | The fight's game build differs from the baselines' (REQ-PAR-07). |

## Dungeon file

Same structure and rules as the night file, with these differences. Its id (written in `nightId`) is
built the same way, from the run's sorted complete boss fight ids, excluding trash and partials.

| Field or rule | Dungeon file |
|---|---|
| File name | `dungeon-YYYYMMDD-<first 8 of the run's id>.json`, same date rule |
| `kind` | `dungeon` |
| `raid` | Absent; replaced by `instanceId` (integer) and `name` (string, the instance as logged) |
| `partial` | Boolean at top level: the log starts mid-fight; an EOF/header cut affects the open fight, not this flag |
| Fight `type` | `boss` or `pull`, as in the night file |
| Player `night` | Replaced by `run`: `{"score": number or null, "coverage": number}`, no rank |
| `wipes`, `assignments` | Absent |
