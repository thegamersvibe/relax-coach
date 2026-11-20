# PR description draft — R2b prep reader

Local draft only; no PR has been opened.

## Summary

- Add stdlib-only `internal/raid/prepread`: explicit-file read-only SavedVariables
  parsing and strict v2 store validation, returning `[]model.PrepReport` for R7.
- Keep the model and addon unchanged. No UI, scoring, dependencies or save writes.
- Expose `Parse(bytes, firstFightStartS, lastFightEndS)` and
  `ReadFile(path, firstFightStartS, lastFightEndS)`. Inputs are seconds, not fight
  milliseconds; apply inclusive `[first - 2h,last]`, select by receipt time,
  deduplicate by stored body index, and stamp with the earliest reference.
- Guard 16 MiB input, parser depth/entry/string limits, canonical wire grammar,
  reference reachability, count caps, and inclusive character/night byte budgets.

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

## Mapping decisions to confirm

- Stored identity `(day,key,index)`, not global byte equality, defines distinct
  reports: preserve A-B-A entries while collapsing multiple references to one.
- Earliest `Stamped` uses all references, including earlier out-of-range receipts.
- Keep accepted gear row order/duplicates and notScored overlaps as the addon
  validator does. R7 performs slot exclusion, stock interpretation and matching.
- The caller handles timestamp unit conversion/PC-server clock differences; no
  offset is guessed here. Level 0 is the brief's sentinel only, never a score.

No protocol/code contradiction or missing model field was found.

## Verification

- Real addon-driven generator commits 14 small fixtures (13 ASCII, one accented
  UTF-8); CI checks exact regeneration and Lua 5.1 reload. 91 retained bodies
  round-trip byte-identically in Go. The separate serializer fix replaces JSON
  Unicode escaping with raw UTF-8 and Lua 5.1 byte escapes, with Lua/Go RED→GREEN
  and control-byte/Unicode tests recorded in [R2b.md](R2b.md).
- RED normal reader, dedup/filter/file entrypoint, long-comment and future-version
  cases recorded in [R2b.md](R2b.md), including the first 62/64 mutation result.
- Final: gofmt clean; raid-tree vet/test pass; discovery **8/8 Go** and **18/18
  Lua/Python**; **65/65 reader mutants** and **4/4 serializer mutants killed**;
  single-worker 5-second fuzz passed (123,143 executions); package statement
  coverage **97.6%**.
- Reader discovery module 6.4s and fixture module 0.2s against unchanged 30s caps.
- No full desktop suite, native game/Windows execution or R7/UI integration.

Base: `2c962c9202bc6ca5eb3142118646252db11f72d2`; branch
`feat/raid-prep-reader`. See [R2b.md](R2b.md) for API, exact limits, fixture
provenance, detailed test output, rollback and remaining native uncertainty.
