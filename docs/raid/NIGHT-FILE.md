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

## Wipe summary (`wipes[]`)

| Field | Type | Required | Meaning |
|---|---|---|---|
| `fightId` | string | yes | The wiped boss fight. |
| `firstDeath` | object or null | yes | `{"player": name as logged, "atMs": integer, "cause": cause}`; `null` if nobody died before the end line. |
| `following` | array of objects | yes | Deaths inside the configured window after the first, same shape, in time order (REQ-DTH-06). |

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
| `partial` | Boolean at top level: the log starts or ends inside the run |
| Fight `type` | `boss` or `pull`, as in the night file |
| Player `night` | Replaced by `run`: `{"score": number or null, "coverage": number}`, no rank |
| `wipes`, `assignments` | Absent |
