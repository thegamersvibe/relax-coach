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
  Round 2 makes nil index-only; a non-nil selector explicitly requests detail.
  Detail.RolesComputed distinguishes uncomputed index roles from evaluated roles
  with insufficient evidence (an empty model.Fight.Roles map).
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
on requested output. In round 1 a caller needing index memory had to set
DetailsFor false; round 2 makes nil the index-only default.
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

## Review round 2 on the unchanged stacked base

Entry gate: branch `feat/raid-fights`, clean tree, HEAD
`db019815578f028ebde15d4112c0947d7472f760`. The unchanged fights suite passed
before edits (`ok .../fights 0.117s`). Both gated private-log tests passed before
production changes; complete boss and full night identities were captured in a
private comparison file outside the repository. Reviewer files were read-only.
No model, logfmt, imports manifest, dependency, README or release files changed.
This section supersedes round 1's default-detail and boss high-water-end rules.

### Tests-first behavior ledger

Targeted commands used `go -C desktop test -count=1 -v -run '^Test...$'
./internal/raid/fights`. Each behavior was followed by a passing full fights
suite. All RED/GREEN entries below are executed results, not predicted failures.

| Ruling | Observed RED | Change and observed GREEN |
|---|---|---|
| MUST: unavailable tank evidence | `TestBossRolesRequireTankEvidence` failed in both no-boss-unit/damaging-swing-taker and known-boss/no-landed-swings cases: DPS and healer were assigned. The positive control passed. | A boss with zero eligible landed boss swings assigns no roles, even with positive damage/healing. Timelines still retain output. All three cases PASS. The player taking the swings also deals damage in the no-boss-unit case. |
| Should 1: END owns boss boundary | `TestBossEndOwnsTailAfterClockSpike`: `fights=1: clock spike swallowed post-END trash`; `TestBossTailUsesRawEndWhenStartIsLater`: display clamp extended the tail. | Reset boss EndMs from END, lower-clamped only to START; use a separate raw END timestamp for tail membership. Both tests PASS. Derived from the second opinion's synthetic START(1000), hit(5000), END(2000), hit(2100), tail=50 repro; now also asserts exact end, inclusive equality, next pull, preserved raw timestamps and server duration. |
| Should 2: nil means index only | `TestNilDetailsForIsIndexOnly` failed for Build and Read: nil retained/computed details. | Nil now selects no detail. Only a non-nil selector returning true requests it. Both cases PASS, including a second pass explicitly naming one fight ID, unchanged IDs/ranges/participants, and no unrequested spec inference. |
| Should 3: explicit computation state | Initial test-first compile RED: missing `RolesComputed`; after field declaration, both selected cases failed with false status. | `Detail.RolesComputed` is false in index mode, true after detail inference even when Roles is empty for no evidence. `TestRolesComputedDistinguishesIndexFromNoEvidence` PASS in nil-index, rejected-index, selected-no-evidence and selected-evidence cases. No shared-model edit. |

The full suite initially exposed five inherited role-fixture assumptions under
MUST. Absorb, activity and ratio-strictness tests now supply a selected boss and
landed swing so their existing healer/DPS assertions still test those rules.
Only the no-landed-swings tank row and a selector overriding to a boss with no
swings changed their expected roles to absent, as explicitly required. Tank
share/top-ratio equalities and all effective totals remain asserted.

Call-site discovery across `desktop/` found no production consumer yet. The
synthetic `params()` helper and external `realLogParams()` explicitly select
full detail for their existing assertions. The memory stress and private index
pass now use nil. Existing selective/false callbacks remain supported. A
separate per-fight sidecar stays available even in index mode, with byte range,
confidence, unscored reason and the false computation status.

### Nine reviewer survivors: actual failing overlays, then clean passes

The nine exact substitutions from the reviewer's hand manifest were applied
one at a time through Go overlays; no production files were rewritten. Each
named test emitted FAIL with exit 1 under its overlay and PASS with exit 0 on
the clean candidate. No compilation-failure kills, refusals or equivalence
claims. These are regression-coverage additions for existing correct guards.

| Reviewer number | Manifest entry (one-based) | Killing test | Observed RED |
|---|---|---|---|
| 4 | 95 | `TestCasterOnlyActorObservation` | caster-only actor lost name/flags in roster or fight |
| 8 | 96 | `TestLateBossSelectionClearsStaleRoles` | late boss reselection retained stale roles without new tank evidence |
| 9 | 97 | `TestIndexBossEvidenceKeepsSourceCreaturePairs` | compacted evidence lost an independently owned source/creature pair |
| 26 | 98 | `TestHeaderResetsSeenEncounterEnds` | header retained duplicate-END state and lost the new orphan |
| 30 | 99 | `TestNegativeAbsorbDoesNotSubtractHealing` | negative/absent absorb changed healing or removed original events |
| 31 | 100 | `TestPetAbsorbCasterCreditsOwner` | pet caster healing or original event not assigned to owner |
| 34 | 101 | `TestFriendlyDamageRequiresNPCControlToExclude` | friendly reaction alone excluded normal player damage or changed incoming |
| 35 | 102 | `TestOwnedNPCFriendlyDamageIsNotCharmedPlayerDamage` | NPC-controlled owned creature was mistaken for a charmed player |
| 37 | 103 | `TestEnemyPetSwingsDoNotSupplyTrashTankEvidence` | enemy pet/guardian swings contaminated trash tank share or top player |

The late-ownership test first proves a tank role exists, then resolves an owner
that changes the selected boss to one without swings and requires all stale
roles to disappear. The pair-key test first closes an index fight, then resolves
an earlier source despite a later hostile source damaging the same creature.
The pet-caster test checks immediate and late ownership, original events, and
exact healing totals. Index repair remains uncomputed.

Inherited manifest entries 1 and 79 were re-anchored from `f.EndMs` to
`encounterEndMs`, preserving their exclusive-tail and no-tail-close mutations.
No other inherited mutant changed. Entries 104–113 cover the new no-evidence,
END override/raw-tail/display-clamp, nil/default selection, explicit selection,
and roles-computed guards. Total: 94 inherited + 9 survivors + 10 new guards =
113, all killed by named tests.

### Documented known behavior, not a fix

`TestDocumentedKnownBehaviourStrayEndSplitsActiveBoss` passed before and after
the boundary fix. The second opinion's synthetic START(77), stray END(88),
END(77) sequence with an intervening hit still yields three partial fragments,
no complete boss, and the intervening hit in the stray encounter. Exact event
membership and empty complete-boss night identity are asserted. This known miss
was deliberately left unchanged under the lead's ruling.

### Real-log checks (counts only)

Both external-package tests wire the real logfmt iterator into fights.Read.
Before and after runs used RELAX_RAID_REAL_LOGS, the privately supplied prefix,
and a full-identity comparison file outside the repository. All 13 complete
boss identities per file and both entire night identities are unchanged from
the clean entry baseline; index and explicit-detail identities also agree.
Only counts are logged: the previous secondary-creature identifier tuples were
replaced by counts. No real names, IDs, lines or private paths were added here.

| Log | Runs / fights | Complete bosses before -> after | Trash / partial | Player entries in Unowned before -> after | Roles assigned / absent before -> after | Tank / healer / DPS before -> after |
|---|---|---|---|---|---|---|
| 1 | 1 / 75 | 13 -> 13 | 62 / 1 | 0 -> 0 | 345 / 25 -> 345 / 25 | 76 / 63 / 206 -> 76 / 63 / 206 |
| 2 | 1 / 75 | 13 -> 13 | 62 / 1 | 0 -> 0 | 336 / 28 -> 336 / 28 | 74 / 61 / 201 -> 74 / 61 / 201 |

MUST changes zero roles in these fixtures; its missing-boss/missing-swing cases
are covered synthetically. Both retain 15 feign lines outside true deaths. Log
2 still ends at its first zone-out, with no later event entering the run. The
private index pass asserts no events, timelines or roles and false
RolesComputed; the explicit-detail pass asserts true status for every fight.

### Final commands actually observed

All jobs ran sequentially with the requested local Go toolchain; Python mutant
and Lua/Python gates used uv Python 3.14 with the CI requirements file.

- `(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)`: no gofmt output, vet exit 0; raid, fights, fixture, logfmt, model all `ok`; modeltest has no test files.
- `(cd desktop && go test -count=1 -run '^TestRaidLog' .)`: `ok .../desktop 0.011s`.
- `python3 -I -B tools/raidtest.py --all --kind go`: `RESULT: 6/6 modules passed`, including race-enabled fights and focused main glue.
- `$PY -I -B tools/raidtest.py --mutants fights`: `RESULT: 113/113 mutants killed`.
- `$PY -I -B tools/raidtest.py --mutants logfmt`: `RESULT: 54/54 mutants killed`.
- `$PY -I -B tools/raidtest.py --mutants raidlogs`: `RESULT: 2/2 mutants killed`.
- `$PY -I -B tools/raidtest.py --all --kind lua,py`: `RESULT: 4/4 modules passed`.
- Gated `go -C desktop test -count=1 -v -run '^TestRealLogsR4' ./internal/raid/fights`: both tests PASS; explicit output confirms all complete boss and full night identities unchanged, package `ok .../fights 5.694s`.
- `git diff --check`: exit 0. Model/logfmt path diffs empty.

No full desktop suite, native Windows/game test, push, PR, tag, amend, rebase,
installation or release. No unresolved ruling questions. A single open fight
still buffers until a boundary; nil/index default is not a hard memory cap.

## Round 3: boss role gate counts player swings only

Reopened must (reviewer plus second opinion): the boss role gate tested the
total of landed boss swings, so swings landing only on a pet, another NPC or an
empty target still let players receive dps/healer roles. The gate now requires
at least one landed boss swing on a player (`top == 0` skips roles).

- RED before the fix: `TestBossRolesRequirePlayerSwingEvidence` failed 8/8 for
  pet-only, pet with a late owner, NPC-only and empty target, each in Build and
  Read (roles `dps` and `healer` assigned); the real-tank control passed.
- GREEN after: all 10 subtests pass; a player taking boss swings stays tank.
- Mutants: the two existing gate mutants were re-anchored to `top == 0`; a new
  mutant restoring `total == 0` is killed by the new test.
- Real logs: both gated tests pass; all 13 complete boss ids and the full
  night identity unchanged; roles 345 assigned / 25 absent and 336 / 28,
  identical to round 2.

## R4 follow-up: player-death scope

Base: `6208b163a0a5491a641809d7cd3c764905276c36` (`origin/main` when
this follow-up started). The architect's "R6 on the real logs" ruling is
implemented as two per-file `fights.Result` counters, beside the existing
`DuplicateEncounterEnds` counter:

- `PlayerDeathsOutsideRuns`: real player-death lines whose timestamp is outside
  every returned run's inclusive `[StartMs, EndMs]`. These deaths are out of
  scope, not reviewed or scored. Empty visits do not emit runs, so their deaths
  count here too.
- `PlayerDeathsInRunsWithoutFight`: real player-death lines inside at least one
  returned run's span but assigned to no fight. This is a runtime segmentation
  diagnostic, expected zero, also asserted by the gated real-log tests.

Both counters use `Event.IsDeath()` and the destination's `Player-` GUID, never
actor flags or pet ownership. Feigns, absent/nonzero death flags, `PARTY_KILL`,
NPCs and pets do not count. The classifier runs after final run boundaries are
known: pending re-entry, headers, empty visits and non-monotonic timestamps
cannot make the current zone flag stand in for the union of returned spans.
Per the literal span rule, a death in an accepted re-entry gap is in-run and
unassigned; this reports a diagnostic without admitting outside lines to fights.

Fight membership means the actual event assignment, including the inclusive
boss tail, not only the displayed fight end. One timestamp and membership bit
are temporarily retained per real player-death line, not an event, name or
GUID; they are released after counting. This adds O(player deaths) temporary
metadata to the earlier streaming-retention contract, not a whole-file event
buffer. Build and streaming Read, with full detail or nil/index selection,
return the same counters. No segmentation, identity, model, parser or config
behavior was changed. The counters are local report metadata, not night-file
fields; `NIGHT-FILE.md` documents that distinction without a format bump.

### Tests first and behavioral mutants

- After declaring the new outside-count field, the new fixture tests failed
  with `player deaths outside runs=0, want 1` (and the two-death fixtures wanted
  2). GREEN: targeted `TestPlayerDeath` and the full fights suite passed.
- After declaring the diagnostic field, the negative fixtures failed with
  `in-run-no-fight=0 outside-runs=0, want 1 / 0`; the two-line fixture wanted
  `2 / 0`. GREEN: targeted tests and the full fights suite passed.
- Fixtures cover boss/pull membership, start/end inclusivity, tail equality and
  one millisecond past it, idle gaps, no runs, pre-instance and post-exit deaths,
  multiple runs, header flush, re-entry, clock rollback/spikes and distinct
  same-time lines with different membership. Death-kind/flag/GUID controls run
  both outside and inside runs, in all four Build/Read and detail/index modes.
- The 114 inherited mutant entries are unchanged. All 18 added substitutions
  build and are killed by their named fixture tests: death/GUID filtering,
  disabled-zone observation, timestamp preservation, detail-independent and
  tail-aware membership, final accounting, both inclusive span guards and their
  removal, outside/diagnostic increments, and checking all runs rather than only
  the first. Full pack: `RESULT: 132/132 mutants killed`, zero refused/survived.

### Real-log evidence (counts only)

The original gated tests passed before edits; complete boss and full night
identities were saved privately outside the repository. After edits, an
independent decoded-line oracle checks every real player death against the run
span and actual selected fight events by source byte offset. The measurement
test also asserts and prints the counters with both all-detail and index reads.

| Log | Player deaths outside runs | Player deaths in runs without a fight |
|---|---:|---:|
| 1 | 0 | 0 |
| 2 | 1 | 0 |

Both gated tests PASS. The full-identity comparison reports `all complete boss
identities and full night identities unchanged from baseline`; all 13 complete
boss ids per log, cross-logger equality, the privately supplied night-prefix
check, 15 feign lines per log and the original zone-out assertions still pass.
No private line, name, id or fixture path was added to the repository.

### Commands actually observed

All test jobs ran sequentially with the requested Go toolchain and uv Python
3.14 plus the CI requirements, with no full desktop suite:

- `(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)`: no formatting output; vet exit 0; raid, cfg, fights, fixture, logfmt and model all `ok`.
- `python3 -I -B tools/raidtest.py --all --kind go`: `RESULT: 7/7 modules passed`.
- `$PY -I -B tools/raidtest.py --mutants fights`: `RESULT: 132/132 mutants killed`.
- `$PY -I -B tools/raidtest.py --all --kind lua,py`: `RESULT: 7/7 modules passed`.
- Gated `go test -C desktop -count=1 -v -run RealLogs ./internal/raid/fights/`, with the supplied prefix and private identity baseline: both tests PASS, package `ok .../fights 4.337s`.
- `git diff --check`: exit 0. No model, logfmt, cfg, dependency or root README changes.

No native Windows/game testing, installation, push, PR, tag or release. No
segmentation fix or scoring rule was added; a nonzero in-run diagnostic reports
the defect rather than hiding it. No unresolved ruling questions.
