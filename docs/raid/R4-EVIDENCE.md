# R4 / D1 implementation and verification ledger

Scope: fights consumes model values; package main finds/open logs read-only.
No model edits, real log contents, dependencies, upload, UI or installation.

## Interface and rulings

- The initial base lacks later model fields; this first slice uses a local
  result sidecar for source, byte ranges, spec, header metadata and timelines.
  Model addendum 2 is now integrated: model.Fight receives byte ranges and
  specs; model raid/dungeon runs receive source/build/log-version/advanced.
  Detail retains convenience mirrors and per-player timelines.
- Power was initially held out by the parent; the subsequent model-addendum
  instruction brought it back in. Power, maximum power and power type are
  sampled for InfoUnit, including present zero mana and absent values.
- Use the brief's cast-signature hook, not PLAN's prospective talent table.
  An empty profile/signature set means unknown, not a guessed spec.
- Use the brief/R5 instance classifier: listed kind wins; an unlisted large
  group may be raid with Listed=false. PLAN 5B's blanket unknown-for-unlisted
  text is superseded by the brief's raid fallback. No dungeon is guessed.
- Follow the brief's newer boss max-health rule and replacement selectors;
  never use the current-target flag as a boss marker.
- Gaps use hostile combat evidence (damage, misses, interrupts, dispels), not
  idle aura/heal/cast chatter. The exact configured gap stays in one pull;
  a strictly larger gap starts another. Boss tail is inclusive and is cut by
  a new encounter start, header, or zone boundary. A return to the same instance
  continues the run only when zone-out to zone-in is strictly under
  `Params.ReentryGapMs` (caller default 1800000 ms) with no intervening non-zero
  instance entry. The exit still closes the fight/tail; outside lines and zone
  markers never enter fight events. A header always starts a new run.
  Run `EndMs` keeps the existing zone-out timestamp convention: the final exit
  after any accepted returns, or the first exit if there is no return. Repeated
  outside-zone changes do not move it or restart the gap. Fight events end at
  the last included line inside the instance. Empty detours emit no run; a
  detour with fights emits its own run (two versus three runs for A/B/A).
- A first non-encounter pull with no observed quiet interval is conservatively
  partial/unscored. Run.Partial means a missing start only; EOF/header cuts
  mark an open fight partial, not a completed boss or the run.
- File-local ids use `anchor:position`; complete boss ids use server fields
  and sorted distinct combatant ids only. Trash is never cross-log matched.
- A malformed complete pair with an absent optional server number uses an
  explicit `unknown` token, not zero; its duration remains absent.
- Main glue accepts the selected installation path; manual path bypasses
  discovery. Discovery uses modification time, ties by path, regular files only.
  No live folder rotation has been observed. R8 must connect the UI.
- M14 index mode uses DetailsFor to omit unselected events/timelines in output.
  Round 1 replaces the original whole-run buffer with an active-fight buffer;
  see the measured retention contract below. Selected detail is retained output,
  not bounded memory for an arbitrarily long run.

## Tests-first slices on the original base

All commands below were actually executed. Package-write tool lint runs from
outside the Go module and emits spurious import errors; the authoritative
checks run inside desktop.

1. Boss identity: RED `undefined: Params`; GREEN raid vet/test exit 0.
   Literal expected SHA input, repeated parse, shifted time/offset and failed
   casts, server duration independent of local clock.
2. Window/run splitting: RED boss+pull count 1 instead of 3, incomplete
   encounter lost, end was encounter end rather than zone-out, header runs
   missing. GREEN raid vet/test exit 0. Covers exact tail/gap, next-start
   ordering, partials, unknown instances, thirteen pairs and missing first boss.
3. Actors/timelines: RED missing Details, then owners/boss roles empty and
   absent timeline sidecar. GREEN fights tests after ownership buffering,
   header reset, same-name pets, replacement selectors, landed-only roles,
   overheal, HP/position subjects, true/feign deaths, spec hook, logger/header
   and multi-boss remaining HP. The first integration run exposed the numeric
   guard on two flag masks; using the protocol's two-bit type field fixed it.
4. Stream/typed output: RED `undefined: Read`; GREEN raid vet/test exit 0.
   Hashes full raw bytes including unread/skipped tails, propagates errors
   without partial result, filters detail selection, refuses wrong run type.
5. Main discovery: RED undefined raidLogChoices/openRaidLog; GREEN
   `go test -count=1 -run '^TestRaidLog' .` => `ok .../desktop`.
6. Review edges: RED first inferred instance overwritten, pet incoming damage
   charged to its player, six invalid configs accepted. GREEN raid vet/test
   exit 0. Equality, missing duration, orphan end, empty stream covered too.

## Harness

`test/raid/fights/module.json` is auto-discovered by the existing runner's Go
`--all` and changed-model dependency selection; runner source needs no change.
CI adds fights mutants and the focused package-main discovery suite.

Initial final-run outputs:
- raid `go vet` + `go test -count=1 ./internal/raid/...`: exit 0, fights/model/rules ok.
- focused main tests: `ok .../desktop 0.015s`.
- `python3 -I -B tools/raidtest.py --all --kind go`: `RESULT: 3/3 modules passed`.
- uv Python 3.14 runner `--all --kind lua,py`: `RESULT: 4/4 modules passed`.
- Initial mutants: `RESULT: 15/17 mutants killed`. One unused-variable build
  refusal was fixed to a runnable mutation; one effective-healing survivor
  exposed a missing numerical assertion, now added without changing production.
  Follow-up verification: `RESULT: 17/17 mutants killed`; raid vet/test exit 0.

## Model-addendum follow-up

The initial slice was committed as 8e4ea59b7a60f666a3d81741735944b303652c64.
It was rebased onto the supplied model commit
5020a0dd5734ee5db13c7f2ce137db7babf1a982 as
4a200b55873f8091f997dcc7e20f5bf81aba36c8. No backup tag was created under
the standing no-tags rule; the original SHA is retained here. The first rebase
attempt stopped for missing committer identity; replay completed with the
authorized per-command identity. No shared model file was edited by this lane.

- Power/range/spec/source RED: missing Sample.Power, then power-only sample
  missing, model fight fields missing and model run source missing. GREEN:
  raid vet/test exit 0 after projection into the new fields.
- Contract review RED: zone instance replaced fight-start instance, empty
  signatures admitted a guessed spec, typed advanced block ignored. GREEN:
  raid vet/test exit 0. Positive signature fixtures now explicitly supply a
  synthetic signature; the unknown-signature regression keeps an empty list.
- Source-error RED: a decoder swallowing an I/O error could publish a partial
  raw hash. GREEN: the counting reader remembers non-EOF source failures.
- Partial/progress RED: a zone-truncated boss did not mark its run partial;
  a stale HP sample could resurrect a dead boss in wipe progress. GREEN:
  run partial propagation, cut trash partials, and dead-unit zero health.
- REQ-PAR-04 power is implemented, no longer held open. Real-log event-family
  golden sampling is still not run. REQ-PAR-06 class remains open: model has
  no typed class field, and fights does not interpret COMBATANT_INFO.Raw.
- PHASE1-BRIEF currently has the power rows in step 0 but no separate heading
  titled “Model addendum 2”; the supplied model revision and the updated
  step-0/v2 text were used. This did not block implementation.

## Final verification on model addendum 2

Commands (Go PATH configured as directed; PY is the supplied uv Python 3.14
invocation with `.github/requirements-ci.txt`):

- `(cd desktop && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)`
  => exit 0; `ok .../internal/raid`, `ok .../internal/raid/fights`,
  `ok .../internal/raid/model` (modeltest is a builder package with no test files).
- `(cd desktop && go test -count=1 -run '^TestRaidLog' .)`
  => `ok github.com/thegamersvibe/relax-coach/desktop 0.010s`.
- `python3 -I -B tools/raidtest.py --all --kind go`
  => `RESULT: 3/3 modules passed` (includes race-enabled fights/model commands).
- `$PY -I -B tools/raidtest.py --mutants fights`
  => `RESULT: 27/27 mutants killed`, no survival or refusal.
- `$PY -I -B tools/raidtest.py --all --kind lua,py`
  => `RESULT: 4/4 modules passed`.
- `git diff --check`: exit 0. Model path diff against the supplied model base:
  empty. PAR-08/PAR-09 requirement text unchanged.

## Earlier integration boundaries (superseded by round 1 below)

- Private logs: not run, R3 decoder absent. TestRealLogsR4 is gated on
  RELAX_RAID_REAL_LOGS plus decoder/config/prefix hooks. It emits only counts
  or fixed failures; no private path, name, line or id. Expected prefix is
  supplied privately, never committed. The expected 13 bosses/15 feign lines,
  cross-logger identity and zone-out properties remain unverified on real data.
- No full desktop suite, native Windows/game checks, upload, push, PR or tags.
- D1's existing model compile-fail type gate runs, plus checked conversions;
  an actual night scorer and raid-history writer are absent, so their negative
  dungeon admission/baseline integration tests remain downstream work.
- No independent reviewer tool is available in this lane. Self-review only;
  parent/custodian review is still required.

## Review round 1 on the stacked R3 base

Entry gate: branch `feat/raid-fights`, clean tree, HEAD
`8d958fd987f65dc0ff43e518be5ab26e2148ed00`. Production baseline measurements
below were taken before any production fix, with only the external integration
test adapter installed. No model or logfmt files were changed. This section
supersedes the earlier run-partial, buffering and private-test limitations.

### Red / green ledger

Each targeted RED command was `go -C desktop test -count=1 -run <test> ./internal/raid/fights`,
except the package-main and Python runner cases stated below. Each implemented
slice was followed by the whole fights suite, not only the newly added case.

| Item | Observed RED | Implemented behavior and GREEN test |
|---|---|---|
| MUST 1 | `player classified as unowned pet`, `player owned`; `charmed friendly damage credited or hostile damage lost` | Player GUID wins over pet/guardian flags and all ownership channels. NPC-controlled player damage to a friendly destination is excluded from DamageDone (incoming damage remains real). `TestPlayerGUIDNeverOwnedOrUnowned`, `TestCharmedFriendlyDamageExcluded` pass with the three reviewed flags and an additional guardian-bit case. Comments name CONTROL_NPC, REACTION_FRIENDLY and type bits. |
| MUST 2 | `inactive combatant assigned a role`; `trash landed swings did not assign tanks at top-ratio equality`; `trash without output or landed swings assigned role` | No zero-evidence DPS/healer default. Trash uses landed non-pet creature swings, the same minimum share and top-player ratio as boss tanks. `TestRolesRequireActivity`, `TestTrashTankEvidence`, `TestNoSwingsNoInventedRoles` pass. |
| s1 | `absorb caster not credited` | Caster enters participation/timeline and receives absorbed Amount as HealingDone; Src/Dst do not. Caster/recipient overlap is not duplicated. `TestAbsorbCreditsCaster` passes. |
| s2 | `closed=0 while decoding, want=1`; retained heap delta `18624688` exceeded `12582912` | Close/project at fight boundaries; discard unselected events immediately. Run-wide compact actor/owner facts repair earlier closed fights, including indexed boss selection, without storing run events. `TestReadClosesFightsWhileDecoding`, `TestReadRetainedHeapBoundedPerFight`, `TestLateOwnershipAcrossClosedFights` pass. |
| s3 | `quiet EOF/header made complete run partial` | Only a missing first fight start makes Run.Partial true. Open fights at a cut remain fight-partial. `TestRunPartialMeansMissingStartOnly` passes. |
| s4 | `run end before start` | Clamp run end to start and fight end to its observed high-water time/start; preserve raw event times and server duration. `TestBackwardTimeBounds` passes. |
| s5 | `duplicate END changed first end or created fight`; follow-up RED `ignored duplicate END moved run end` | First end wins; later ends until the next start for that encounter are counted and skipped, including their timestamps. Result and Run expose DuplicateEncounterEnds. `TestFirstEncounterEndWins` passes inside/outside the tail and across a subsequent pull. |
| s6 | All twenty reviewer survivors now produce named test FAIL under runnable overlays | Mapping below; no equivalence claims, build-failure kills, or surviving mutants. |
| s7 | Test-first compile RED `undefined: raidLogChoicesWithReadDir`; restoring the original fail-list behavior is a killed runnable mutant | Per-call injected directory reader, failed Info increments skipped then continues. `go -C desktop test -count=1 -run '^TestRaidLog' .` passes; `TestRaidLogInfoFailureSkipsOne` kills both fail-list and omit-counter mutants. |
| repeat rule | `repeat 2 has wrong identity` | Complete identical inputs receive `repeat N` from the second occurrence, per file (not per run); first is unchanged. `TestRepeatBossIdentity` passes for two/three wipes, 1 ms difference, repeated Read, standalone second pull, headers, partial exclusion, and sorted night hash. |

The existing non-tank expectations in `tank_test.go` now require absent roles
where those synthetic players have no output evidence. Tank/share/top-ratio
assertions remain intact. `TestIncompleteFightMarksRunPartial` retains its test
identity but now asserts the approved end-cut meaning (fight partial, run not
partial). No other inherited acceptance assertion was relaxed.

The discovery mutant module has source `desktop/` but invokes only TestRaidLog
or a named mutant test, never the full desktop suite. Its first mutant run
exposed the runner's rejection of the root Go package. Test-first
`test_go_root_package_mutant` failed with `source must be under desktop/`;
the two-line root-package mapping then passed. A runner mutant removes that
mapping and is killed by the same test. CI runs the two added mutant modules.

### Reviewer hand survivors (zero-based indices from the review log)

| Survivor | Killing test |
|---|---|
| 2 | `TestInitialPullGapEquality` |
| 3 | `TestOnlyFirstPullCanMissRunStart` |
| 4 | `TestCutPullGapEquality` |
| 5 | `TestZoneClosureIsNotEOFCut` |
| 7 | `TestMismatchedEndDoesNotCompleteBoss` |
| 9 | `TestRepeatedSameZoneDoesNotSplit` |
| 12 | `TestCutPullGapEquality` |
| 13 | `TestInterruptDispelStartCombat` |
| 14 | `TestClassifierGetsMaximumGroup` |
| 16 | `TestPartialBossExcludedFromNight` |
| 17 | `TestQuietOnlyRunNotEmitted` |
| 18 | `TestEqualRankOwnerFirstWins` |
| 22 | `TestBossCandidateNeedsPlayerDamage` |
| 24 | `TestSelectorRejectsTruncatedGUID` |
| 28 | `TestConfidenceThresholdEquality` |
| 31 | `TestSelfEventAppearsOnce` |
| 33 | `TestOverkillExcludedFromDamage` |
| 37 | `TestTankPrecedesHealerWhenRequired` |
| 40 | `TestUnknownBossHealthStaysAbsent` |
| 44 | `TestHeaderAdvancedZeroMeansOff` |

All twenty are entries 46–65 (one-based) in `test/raid/fights/mutants.json`.
Hand #9 is re-anchored to the same-zone early return after re-entry refactoring;
#12 to the streaming EOF flush; #18 to actorIndex priority; #37 to the healer
condition with the new nonzero-evidence guard. These are behavioral mutations,
not claims of equivalence.

Existing manifest re-anchors: #9 (`s.priority`); #20 and #21 (field projection
moved from actors.go to fights.go); #27 (`s.advanced`). Existing #25's obsolete
end-partial propagation mutation is replaced with the missing-start propagation
mutation and `TestInitialPullGapEquality`, under the explicit s3 ruling.
The manifest grew from 45 to 94: 20 reviewer survivors plus 29 new guards.
Discovery adds 2 and the runner adds 1; 52 new entries across the three packs.

### Real-log integration (counts only)

`real_logs_test.go` is now in external package `fights_test`. It adapts the real
logfmt.Decode iterator to fights.Read and propagates ParseReport.ReadError.
TestImports explicitly excludes `_test.go`; production fights imports remain
model/stdlib only, and imports.json is unchanged. The module's test dependency
now includes logfmt. The test classifier maps instance 329 to listed dungeon;
it does not guess other small-group instances as dungeons.

Both gated tests passed with RELAX_RAID_REAL_LOGS, the privately supplied
RELAX_RAID_NIGHT_PREFIX, and an external private identity comparison file.
Before/after comparison checks all complete boss ids and the entire night id,
not just the prefix; no identity value is logged or stored in this repository.
The original TestRealLogsR4 expectations remain enabled and passed: 13 complete
kills each, 15 feigns each never in Deaths, matching cross-logger ids, first
partial pull, and log 2 ending at its FIRST zone-out despite later outside lines.

| Log | Runs | Fights | Boss | Trash | Partial fights | Complete kills | Player entries in Unowned before -> after | Roles assigned / absent after | Tank / healer / DPS after |
|---|---:|---:|---:|---:|---:|---:|---|---|---|
| 1 | 1 | 75 | 13 | 62 | 1 | 13 | 223 -> 0 | 345 / 25 | 76 / 63 / 206 |
| 2 | 1 | 75 | 13 | 62 | 1 | 13 | 220 -> 0 | 336 / 28 | 74 / 61 / 201 |

Raw absorb events: 1458 / 1423. In-fight absorbs with player casters:
1097 / 1062, crediting 53662 / 52839 effective healing respectively. Each
caster's total is independently checked against ordinary effective healing
plus absorbed amounts. Out-of-fight events are not silently credited to a fight.
The secondary-creature probe emitted one qualifying secondary unit for one
encounter in each file, the same numeric tuple in both; the requested numeric
encounter/NPC/health tuple is in the private task report, not copied here.

### Memory measurements and guarantee

The test opens each file as a stream, forces GC before measurement, samples
runtime.ReadMemStats HeapAlloc every 512 emitted events and after Read, then
forces GC with runtime.KeepAlive(result) to measure retained output. Values are
bytes above that invocation's starting heap, not TotalAlloc, RSS, or an exact
continuous peak. GC timing varies; these are observed sampled peaks, not caps.

| Log / mode | Baseline sampled peak | After sampled peak | Baseline retained | After retained |
|---|---:|---:|---:|---:|
| 1 / all detail | 212942968 | 194807768 | 142762128 | 142727464 |
| 1 / index | 176985696 | 8671632 | 677688 | 674792 |
| 2 / all detail | 239882544 | 220361672 | 146213016 | 146207368 |
| 2 / index | 165998624 | 9401296 | 676504 | 673816 |

Guaranteed retention structure: segmentation holds only the active fight and
its inclusive tail, never a second whole-run event buffer. Closed index fights
release events/timelines. Returned fight metadata and CombatantInfo payloads,
per-file repeat input keys, run actor/ownership facts, touched ids and compact
boss-selection evidence grow with fights/actors (distinct source-target pairs),
not repeated discarded event lines. Explicitly selected details retain their
requested events and timeline projections, so all-detail memory still grows
with output. There is no universal byte cap on one arbitrarily large fight or
on requested output; a caller needing index memory must set DetailsFor false.
DetailsFor is called once when the fight closes; late ownership is reconciled
at run completion without calling the selection callback again.

The CI-capable synthetic stress generates 120 fights with 256 payload-bearing
combat events each instead of prebuilding a run. Every 20 fights it forces GC;
retained heap must remain under a 12582912-byte delta. Final observed maximum:
334976 bytes. It fails on the original run buffer and on the no-release mutant.
The existing 40-player/pet retention and re-entry suites also remain green.

### Final commands actually observed

Go PATH was set to the requested local toolchain (go1.27.3). PY was uv's Python
3.14 with `.github/requirements-ci.txt`; no dependencies were added to Go.

- `(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)`: no gofmt output, vet exit 0; raid, fights, fixture, logfmt and model all `ok` (modeltest has no test files).
- `(cd desktop && go test -count=1 -run '^TestRaidLog' .)`: `ok .../desktop 0.010s`.
- `python3 -I -B tools/raidtest.py --all --kind go`: `RESULT: 6/6 modules passed`, including race-enabled fights/decoder/model suites and focused main glue.
- `$PY -I -B tools/raidtest.py --mutants fights`: `RESULT: 94/94 mutants killed`.
- `$PY -I -B tools/raidtest.py --mutants logfmt`: `RESULT: 54/54 mutants killed`.
- `$PY -I -B tools/raidtest.py --all --kind lua,py`: `RESULT: 4/4 modules passed`.
- Additional packs: `--mutants raidlogs`: `RESULT: 2/2 mutants killed`; `--mutants _runner`: `RESULT: 1/1 mutants killed`.
- Gated `go test -count=1 -v -run '^TestRealLogsR4|^TestReadRetainedHeapBoundedPerFight$' ./internal/raid/fights`: all three named tests PASS, package `ok .../fights 4.098s`.
- `git diff --check`: exit 0.

No full desktop test suite, native Windows/game test, push, PR, tag, rebase,
installation or release was performed. R5 class/spec integration remains outside
this lane; these two real fixtures are dungeons, not a live 40-player raid log.
