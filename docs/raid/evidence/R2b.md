# R2b: read-only prep reader evidence

Branch `feat/raid-prep-reader`, based on
`2c962c9202bc6ca5eb3142118646252db11f72d2` (R2 store merged with main `6208b16`).
`07f23c6` is an ancestor, verified by `git merge-base --is-ancestor` returning 0.
Local-only work: no push, PR creation, tag, install, private player data, or new
dependency. Fixture names/items are invented. No root README/version edits.

## API and design

New stdlib-only package `desktop/internal/raid/prepread`, registered in
`imports.json` with only `model` allowed. No model, cfg, fights, logfmt or addon
source changes; no UI wiring. `parseLuaAssignedTable` and the AH Profit save
parser are in package main, which the raid import rules prohibit importing.
The new parser therefore implements a small independent strict data grammar.

```go
func Parse(data []byte, firstFightStartS, lastFightEndS int64) ([]model.PrepReport, Report)
func ReadFile(path string, firstFightStartS, lastFightEndS int64) ([]model.PrepReport, Report, error)
```

The reader has no fights dependency. These functions expose the brief's night
filter directly: the caller supplies first fight start / last fight end in
**seconds**, and the reader applies inclusive `[first - 7200, last]`. Do not
pass `model.Fight` milliseconds directly. The caller owns clock alignment and
conversion; the reader does not guess a PC/server offset. Server clocks from
both sender and receiver remain available for R7's evidence.

One result per retained `(day, whole-name key, body index)` with at least one
reference in that interval. `Stamped` is the minimum over every reference,
not merely eligible references. A-B-A entries remain separate; repeated
references to one index do not duplicate it. Played time is decoded from the
current refreshed body, while its original sender clock is retained. Sort by
`Stamped`, then exact `Name`; complete ties retain source-night insertion
order, sorted ASCII name key, then body index. UTC-noon buckets do not replace
receipt-time filtering: a previous-night report is excluded by the interval.

`Report.Warnings` has one bounded reason for refusal. `Report.Skipped` counts
stored bodies by reason (including `outside night`), never repeats a bad body's
count for each window reference, and contains no player names or input text.
I/O errors are separate. `ReadFile` accepts only the caller's explicit regular
file, rejects a symlink final component, checks identity after open, uses a
bounded read, and never writes, searches another account, or caches old input.

### Bounds and validation

Larger constants are embedded in `limits.json`, following logfmt's data-limit
convention; no non-test Go numeric literal above 999 was introduced.

- File: 16 MiB, checked before copying/parsing and during actual file reads.
- Literal parser: depth 16, 150,000 table entries, 4,096 bytes per decoded
  string/key, 64-byte numeric tokens and 64 `=` characters in long comments.
- Only `RelaxRaidDB = <table literal>` plus whitespace/comments/optional final
  semicolon. Typed numeric/string keys stay distinct. Duplicate assignments,
  sparse arrays, executable expressions, invalid escapes, non-finite numbers,
  trailing statements and incomplete files are refused. Quoted strings support
  Lua decimal byte escapes; long strings and hex numbers are intentionally not
  part of this SavedVariables subset. Comments include balanced long brackets.
- Wire: eleven fields, body <=4,096 bytes, all Prep scalar vocabularies/ranges,
  at most 32 consumables and 19 gear/notScored entries, unique consumable item
  IDs and notScored slots, agreeing gear/list markers. No local realm fallback.
- Store: at most 3 unique days, 80 nonempty windows/night, 40 characters/window;
  dense body lists, at most 80 bodies/character, hence 3,200 bodies/night and
  9,600 total. Every body is referenced and every reference resolves.
- Byte budgets inclusive: 100,000/character/night and 4,000,000/night. Count
  retained body-string bytes once, not serialized escapes or reference rows.
  For accepted canonical bodies this is exactly `#Wire.encode(report)`;
  byte-for-byte Go re-encoding is tested against every retained Lua fixture body.
  Raw invalid body strings cannot bypass structural budgets by failing decoding.
- Unlike the mutating addon load path, structural corruption/cap violations
  refuse the snapshot rather than silently evicting/repairing saved data. This
  is the lead's stricter read-only acceptance rule. Invalid wire bodies inside
  an otherwise structurally valid snapshot are individually counted/skipped.

## Mapping decisions to confirm (not missing model fields)

1. “Distinct body” means the stored identity `(day,key,index)`, not a global
   hash of byte-identical content. This preserves the documented A-B-A history.
2. Earliest receipt means **all** references, even if the body qualifies only
   through a later in-range receipt. This follows the brief literally.
3. Level 0 is only the explicitly authorized unknown-level sentinel; no
   unreadable consumable, gear, played-time or score is mapped to measured zero.
4. Preserve gear list order/duplicate slots and its overlap with notScored,
   exactly as `Prep.validate` accepts them. Do not infer/scoring-filter here.

No unresolved protocol/Lua contradiction or unrepresentable model field was
found. R7 still owns full-name matching, stock across reports, NotScored slot
exclusion, and its last-report/fallback policy. This lane does none of those
scoring operations.

## Field mapping (architect's brief)

Authority: `PHASE1-BRIEF.md`, **Prep reader: saved store to
`[]model.PrepReport`**, as directed by the lead's superseding clarification.
Wire spelling and validation are checked against `Wire.lua:29-89` and
`Prep.lua:140-195`; the saved reference layout is in `PREP-PROTOCOL.md`.

| Wire/saved field | Model field | Marker / meaning | Units |
|---|---|---|---|
| 1 `protocol` | `Protocol` | exactly `1`; other values reject the body | version integer |
| 2 `addonVersion` | `AddonVersion` | literal `unreadable` -> empty string; other valid text retained | text, at most 24 bytes |
| 3 `name` | `Name` | original spelling, whole realm-qualified name retained; never repaired or split | at most 96 bytes |
| 4 `class` | `Class` | literal `unreadable` -> empty string; otherwise the Lua class vocabulary | class token |
| 5 `level` | `Level` | `u` -> **0, the brief's unknown-level sentinel**, not a measured level or score; readable 1..60 unchanged | level |
| 6 `playedLevel` | `PlayedAtLevelS` | `u` -> unset `Opt`; readable `0` -> `Some(0)` | seconds |
| 7 `reason` | `Reason` | exactly `join`, `ready`, or `encounter`; otherwise reject | enum |
| 8 `sentAt` | `SenderClock` | canonical decimal, no unknown marker | server Unix seconds |
| 9 `consumables` | `Consumables`, `ConsumablesUnreadable` | `u` -> empty + flag true; `-` -> empty + flag false; rows preserve category/itemID/count and order | item IDs and bag counts |
| 10 `gear` | `BetterGear`, `GearChecked`, `GearUnreadable` | `n` -> unchecked; `u` -> checked and unreadable; `-` -> checked, readable empty; rows preserve slot/wornID/bagID | slot and item IDs; wornID 0 means confirmed empty slot |
| 11 `notScored` | `NotScored` | `n`/`u` only with matching gear marker; `-` with checked gear means empty; otherwise unique slots 1..19 retained in received order | inventory slot IDs |
| window reference `receivedAt` | `Stamped` | earliest receipt among **all** references to the selected stored body | receiver server Unix seconds |
| `day`, window position, body index | internal selection/provenance only | not added to `model`; day is not the app's content-derived night ID | UTC-noon bucket / 1-based indices |

`n` decodes to the addon's `not checked`; `u` decodes to its `unreadable`.
The Go reader uses the model flags/Opt above, not zero-valued measurements.
Those literal phrases are not accepted as substitutes for `n`/`u` in wire
fields. `notScored` never blanks other gear slots. All consumable categories,
including flask and elixir, remain evidence; R7 owns stock/gear scoring.

| Store version/input | Output |
|---|---|
| account 1, prep 2, valid shape | selected reports |
| prep 1 or another non-newer unsupported/malformed prep version | no reports, one `store version` warning |
| numeric account >1 or numeric prep >2 | no reports, one `newer store` warning; no layout interpretation or writes |
| missing/wrong-type prep, corrupt shape, cap violation | no reports, one specific warning |
| invalid wire body in structurally valid store | skip that body, increment its reason once; retain other valid bodies |

Future-version detection matches Store's early numeric latch, including a
fractional future version and a future prep version beside a malformed account
version. Invalid syntax or parser resource bounds are refused before any schema
interpretation. Neither v1 nor newer input is rewritten.

## Fixtures: real producer output, not a hand-written positive oracle

`test/raid/prepread_fixtures/generate.py` boots `_client/Client` with the real
RelaxRaid TOC, runs real Prep/Wire/Store, then uses `_client/serialize.literal`
to write account SavedVariables. That serializer is the existing simulator's
Lua-literal writer, **not a claim of native Blizzard file formatting**.

Regeneration:

```sh
uv run --no-project --python 3.14 --with-requirements .github/requirements-ci.txt python -I -B test/raid/prepread_fixtures/generate.py --write
```

Normal CI mode regenerates in memory and compares exact fixture bytes without
writing. A discovered `prepread_fixtures` Python module runs this check; the Go
`prepread` descriptor declares the data dependency. No runner edit is needed.

Committed under `desktop/internal/raid/prepread/testdata/`:

| File | Scenario |
|---|---|
| `normal.lua` | potion/food/flask/elixir, played time, two upgrades including confirmed empty worn slot, notScored slot |
| `dedup.lua` | two window receipts sharing one body |
| `played-refresh.lua` | second receipt refreshes played time in place; original sentAt and earlier receipt retained |
| `changes.lua` | retained content changes, with repeated references to the first body |
| `aba.lua` | three indices with A-B-A byte-equal endpoint bodies |
| `markers.lua` | unchecked, unreadable, checked empty, unreadable scalars, readable played zero |
| `night-eviction.lua` | four real night puts retain the newest three |
| `window-eviction.lua` | 81 real windows retain 80; earliest surviving receipt is used |
| `player-eviction.lua` | 41 real identities in one window retain 40 |
| `wide.lua` | 32 real maximum-field bodies spanning every category and all class tokens; preserved nonascending notScored order |
| `version-one.lua` | synthetic v1 tag on an actual Store-produced v2 payload; version gate control, not an old writer claim |
| `version-one-reloaded.lua` | actual addon Init discards that tagged v1 store |
| `newer.lua` | synthetic future tag on actual output, then actual addon Init preserves it |

There are **13 fixtures** and the Go property visits **90 stored bodies**,
including tagged version controls: all decode and re-encode identically.
Field-by-field normal/marker/wide/retention assertions use independent expected
model values. Hostile mutations and exact-byte controls are test-only synthetic
variants of real bodies, explicitly separate from producer fixture generation.
Exact 100,000-byte and 4,000,000-byte cases pass; +1 byte cases refuse. A rotating
three-night control exercises all 9,600 references/bodies without changing caps.

## Observed RED -> GREEN

| Slice | Actual RED | GREEN |
|---|---|---|
| First real normal save | `TestRealNormalReport`: `reports = []model.PrepReport(nil)` versus the full expected report | same test PASS after literal/codec/store path |
| Distinct body + interval + file entrypoint | dedup returned two reports; interval returned two instead of one; ReadFile returned empty | all three original named tests PASS after selection and read-only I/O implementation |
| Long-comment safety + future versions | accepted `--[=[\nRelaxRaidDB = {}`; valid long comment failed assignment; version 2.5 returned `store version` | literal suite and future-version test PASS after balanced delimiters and early numeric future latch |
| Future prep beside malformed account version | `version=2.5 account=false diagnostic={Warnings:[account version] ...}` | same case PASS after both future latches precede schema checks |

Other hostile/marker/budget/round-trip tests passed on their first run; they are
additional regression coverage, not claimed original RED tests. Temporary
source-authoring syntax/standalone lint errors were fixed before Go test runs;
no compilation failure was counted as a behavioral RED or mutant kill.

First mutation run: **62/64 killed**. Mutant 18 survived because the test omitted
account versions in `(1,2]`; mutant 64 survived because the invalid underscore
name also lacked a hyphen, so the separate qualification guard masked it.
Added independent account 1.5/2 and qualified `Alia_Stone-EU` negatives, retained
all original cases and both original mutants. Added an unordered-receipt mutant.
Final result: **65/65 killed**, none survived/refused. Production guards were
not weakened to obtain this result.

## Final commands actually run

Go 1.27.3 and the existing pinned CI Python 3.14/lupa environment. Commands were
run sequentially; no full desktop `go test ./...` was run.

```text
(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)
# no gofmt paths or vet diagnostics
ok raid             0.031s
ok cfg              2.842s
ok fights           0.280s
ok fixture          0.006s
ok logfmt           4.803s
ok model            0.153s
model/modeltest     [no test files]
ok prepread         0.944s

python3 -I -B tools/raidtest.py --all --kind go
ok cfg 27.0s; fights 3.0s; fixture 1.5s; logfmt 16.0s;
   model 1.8s; prepread 6.2s; raidlogs 2.9s; rules 0.3s
RESULT: 8/8 modules passed

uv run --no-project --python 3.14 --with-requirements .github/requirements-ci.txt python -I -B tools/raidtest.py --mutants prepread
RESULT: 65/65 mutants killed

uv run --no-project --python 3.14 --with-requirements .github/requirements-ci.txt python -I -B tools/raidtest.py --all --kind lua,py
RESULT: 18/18 modules passed
# prepread_fixtures 0.2s; largest module _runner 12.5s

GOMAXPROCS=1 go -C desktop test ./internal/raid/prepread -run '^$' -fuzz FuzzParseNeverPanics -fuzztime=5s -parallel=1
# 4/4 seed inputs, 27,392 executions, no failure
PASS
ok prepread 5.161s

go -C desktop test -count=1 -cover ./internal/raid/prepread
ok prepread 0.898s coverage: 97.6% of statements

go -C desktop test -count=1 ./internal/raid/prepread -run 'TestLuaBodiesRoundTrip|TestNewerVersionsAreNeverInterpreted' -v
# 90 real serialized Lua bodies re-encode identically
PASS

git diff --check
# clean
```

Module deadlines remain unchanged at 30 seconds. This lane's discovered Go
module took 6.2 seconds and fixture module 0.2 seconds. The existing cfg lane
was the nearest deadline (27.0 seconds); it passed unchanged.

## Not verified / release boundary

No native Windows/game execution, Blizzard SavedVariables flush/format check,
live realm spelling, R7 integration or clock alignment is claimed. No UI was
wired. This is a local candidate for the architect's mapping confirmation and
independent review, not merge/release approval. Rollback is removal of the new
reader/test lanes and its imports entry; no saves have been modified.
