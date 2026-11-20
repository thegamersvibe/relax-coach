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

`rank` starts at 1 within the night, over all players with a night score. Equal scores share a rank
and the next rank is skipped (1, 1, 3). Rank compares the stored, rounded score, so the file and the
page cannot disagree.

**Not enough data.** Coverage is always the measured number, never `null`; a player with nothing
measurable has `0.00`. Under the coverage threshold in the config, `score` and `rank` are `null` and
coverage still shows why. This holds at night level and at fight level (a fight has no rank). A reader
recognises "not enough data" by `score` being `null`.

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

- `Coverage int`: hundredths, 0 to 100 inclusive. The writer formats it as `0.00`
  to `1.00` without rounding again; for example, 75 becomes `0.75`.
- `Provisional bool`: fewer baseline samples than the configured minimum.
  It is `false` when not set, including parts without a baseline.
- `Baseline string`: exactly `"own"`, `"guild"`, `"role"` or `""`. The empty
  string means not set and is written as `null`.

The model's zero values (`0`, `false`, `""`) mean not set. Coverage has no separate
presence bit; `Tenths` remains the optional score that distinguishes an unmeasured
part from a measured score of zero. This is an additive in-memory model change,
not a new file field or a change to `formatVersion`.

## Evidence

| Field | Type | Required | Meaning |
|---|---|---|---|
| `text` | string | yes | One plain English sentence, at most 200 characters, e.g. "No cast for 38 seconds from 2:10 in the add phase." |
| `counts` | object: string → integer | yes | Short name to count, e.g. `{"interrupts": 3}`. May be empty. |
| `atMs` | array of integers | yes | At most 20 times, milliseconds from the fight start. May be empty. |

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
and an integer count of excluded real player deaths stamped **after** the final
run end; it writes nothing. Callers pass `fights.Run.EndMs`, not `Fight.EndMs`.
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
