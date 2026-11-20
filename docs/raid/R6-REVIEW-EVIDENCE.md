# R6 round-1 follow-up evidence

Base: `1203953614ac8473133e6db190968a3a06bb0440`. The lead authorized two
commits after the architect supplied **R6: rules 3 and 4 bounded**: first the
cfg/data prerequisite, then the death-engine correction. No native/game test,
release, push or model/parser/fights change is part of this work.

## Configuration prerequisite

- Scoring content 1.4.0 adds required positive int64 `death.pulled_aggro_ms`
  (10000, explicitly an architect guess). Reflection decoding supplies strict
  shape/type/null/required checks; the scalar copies through `Snapshot()`.
- The previously missing cross-field guard is
  `0 < death.raidwide_window_ms <= death.window_ms`. Existing fraction
  `(0,1]` and sample minimum `>= 1` guards already existed and were retained.
  The representation-edge test raises the outer window to MaxInt64 so the
  typed integer boundary remains independently tested.
- `TestPulledAggroConfig` failed on accepted positive values (unknown key) and
  missing-key acceptance; `TestPulledAggroSeedAndCopies` failed on the absent
  seed. `TestRaidwideWindowBound` failed at 4001/4000, 10001/10000 and
  12001/12000. All passed after the change. Existing fraction/sample boundary
  tests passed before and after, including zero, adjacent floats, one and
  integer overflow.
- `TestRaidOneProsePins` pins the exact GCD source clause and README threshold
  statements. Separate scratch data copies changed only the GCD source or
  threshold inequality: both produced an actual named test failure, while the
  unchanged data passed. No source or reviewer evidence was altered by these
  negative controls.
- Initial full cfg run caught a diagnostic-order regression: invalid outer
  `window_ms` was reported as `raidwide_window_ms`. Checking the outer window
  first restored the inherited diagnostic assertion; no test was weakened.

Observed commands/results (sequential):

```text
go test -count=1 ./internal/raid/cfg ./internal/raid/death   baseline PASS
new cfg tests: RED, then GREEN
python3 -I -B tools/raidtest.py cfg
ok   cfg                    16.6s
RESULT: 1/1 modules passed
python3 -I -B tools/raidtest.py --mutants cfg
RESULT: 170/170 mutants killed
(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)
PASS; gofmt/vet silent; raid, cfg, death, fights, fixture, logfmt, model green
```

The cfg deadline remains 30 seconds. All 164 prior mutations were retained;
six new mutations cover the pulled-aggro zero/one boundary, required key,
validated snapshot isolation, and cross-field deletion/equality. Prose
negative controls are separate because the module's mutant runner correctly
refuses data files outside its Go-source scope.

## Death-engine correction

Cfg prerequisite commit: `bf07a14b87d61224cee5fd0d8ab110e02218959f`.
The architect's bounded-rule and round-1 additions replace the original
unbounded/no-living-tank and latest-tank-hit interpretations. Reviewer source
files were read only. A separate archive of the exact original base reproduced
all six named reviewer probes unchanged: the false-blame subcases, same-ms
cutoff, confidence and recap assertions failed; the old attack-time assertion
failed with unresolved; the observational lifetime probe showed the earlier
life's mechanic charged again. Committed regressions apply the current rulings,
not superseded reviewer expectations.

| Requirement | Observed RED | GREEN / committed regression |
|---|---|---|
| MUST 1: positive victim damage and bounded rule 3 | Passive victim blamed; 60-second opener still blamed; attack after turn blamed | `TestZZFalseBlame`, `TestPulledAggroBound`, `TestPulledAggroKillingRun`, `TestPulledAggroDamageOnlyAndFightReach`; 9 seconds and inclusive config edge blame, 60 seconds does not |
| MUST 2: known non-tank role | Nil/missing/empty/unknown roles charged as puller or tank loss | `TestAggroUnknownVictimRole`, unknown-role reviewer subcase; only `dps`/`healer` qualify |
| MUST 3: observed holder, alive at run start | Wrong tank selected; dead-at-turn/resurrected-by-kill blamed; live-at-turn/dead-by-kill lost | `TestLostBossObservedHolder`, `TestLostBossHolderAliveAtTurn`, reviewer dead-tank cases; only the last observed tank target can own the death |
| Holder age / same-ms order | Older holder accepted; later same-ms holder line accepted; same-ms tank damage ignored | `TestLostBossHolderWindow` at 9999/10000/10001 plus changed config; `TestLostBossSameTimestampOrder` pins both orders and same-ms damage |
| Missed swings / no tank hit | Missed holder/run swings and a holder with no damage produced no match | `TestLostBossMissedSwingsAndNoHit`; boss MISS/PARRY/DODGE/ABSORB participate; no holder damage satisfies the gap |
| Exact death-line cutoff | Later same-ms resurrect/damage charged the first death; later heal/health/death altered it | `TestZZSameTimestampPostDeath`, `TestDeathLineCutsHealthAndHealerState`, `TestAggroActionsAfterDeathExcluded`; two deaths in one millisecond remain distinct |
| One life per window | Earlier-life mechanic charged the next death | `TestZZLifetimeEvidence`; additional prior-death-line, missing-res, feign, run-start and one-shot guards |
| Environmental provenance | Nil-source spell/swing/unknown type all became unavoidable/high | `TestNilSourceIsNotEnvironmental` now unresolved/low; `TestEnvironmentalParserProvenance` decodes synthetic ENVIRONMENTAL_DAMAGE controls through the real parser |
| Confidence explicitly stated in brief | After-wipe + avoidable high; avoidable + unresolved fallback medium | Both `TestZZConfidenceRuling` subcases now pass: medium and high respectively; priority/owners unchanged |

`pulledAggroInRange` isolates the inclusive opener-to-run bound and reads
`cfg.Death.PulledAggroMs`; the scan finds first damage in the fight, not just
this life or a last-target-change interval. The architect supplied the field,
so the temporary unbounded-since-fight-start policy was not used. Damage by a
tank before his death remains opener evidence (`TestDeadTankOpenerStillCounts`).
There is no threat inference: the architect's damage-event definition supersedes
the earlier request to count a tank's unsuccessful attacks as damage. Boss
missed/dodged/parried swings still establish holder and run order.

### Mutation and inherited-test audit

All 60 inherited death mutation entries remain, with obsolete anchors migrated
to the replacement predicates; 44 new entries produce 104/104 actual kills.
The initial 90/91 run exposed a single-target fixture masking a last-target
mutation. Adding an earlier different boss target killed it; the final pack
has no survivors, anchor refusals or compiler-error kills.

The reviewer's three stock-suite survivors are covered:

- Dead-holder guard deletion and checking survival at the killing hit: killed
  by `TestLostBossHolderAliveAtTurn`.
- Tied tank attacks cannot invent a holder: `TestLostBossTiedAttacksDoNotChooseHolder`
  kills a fallback to the first tank attacker. The old tie boolean no longer
  exists: ordered boss targets resolve holder identity, not tied damage times.
- Unowned guardian exclusion: the exact exclusion-deletion mutant is killed
  by `TestUnownedGuardianExclusion`, for both candidate and counted source.

Inherited test identities remain. Intentional migrations were limited to:

- No-living-tank puller expectation becomes unresolved under the architect's
  replacement; its feign control and lost-boss positives gain an observed
  prior boss target. Tank exclusion is otherwise a concrete qualifying loss.
- Same-ms healing/healer-death controls now occur **before** the victim death
  line; their assertions stay unchanged. Separate new tests cover after it.
- The environmental fixture supplies the parser's `FALLING` type instead of
  treating an absent actor as provenance.
- The wipe-priority fixture now expects medium because avoidable also matches.

A synthetic parser control initially had an invalid timestamp (missing offset);
that setup error was corrected before recording the valid environmental RED.
The original failed logs are retained, not counted as behavioral kills.

### Final executed gates

All commands were sequential; full-desktop tests were not run. Go formatting
and vet emitted nothing. The required raid-package Go suite passed. Actual
module output:

```text
python3 -I -B tools/raidtest.py --all --kind go
ok   cfg                    16.0s
ok   death                   2.6s
ok   fights                  2.2s
ok   fixture                 1.5s
ok   logfmt                 19.4s
ok   model                   1.7s
ok   raidlogs                0.9s
ok   rules                   0.5s
RESULT: 8/8 modules passed
python -I -B tools/raidtest.py --mutants death
RESULT: 104/104 mutants killed
python -I -B tools/raidtest.py --all --kind lua,py
RESULT: 7/7 modules passed
```

The Python mutation and Lua/Python commands used the CI requirements on Python
3.14. The final JSON Go test run recorded 148 PASS / 0 FAIL / 1 SKIP test
and subtest identities; the skip was the opt-in real-log test, run separately:

```text
Log 1: Deaths=1, Causes={unresolved:1}, Confidence={low:1}, SuggestedCause=""
Log 2: Deaths=1, Causes={unresolved:1}, Confidence={low:1}, SuggestedCause=""
Each: FeignsExcluded=15, Wipes=0, HealthSamples=0, RaidwideShare=null
--- PASS: TestRealLogsR6 (2.06s)
ok  raid/death  2.109s
```

Counts are unchanged; these logs do not prove any positive cause or wipe rule.
No real lines, names or private paths are committed. Native Windows, game,
rendering, result serialization and officer overrides remain untested.

### Run-end clock spike: reproduced contract gap, not fixed

The run-end exclusion and aggro evidence bound now have explicit lead rulings.

- `Review(fight, runEndMs, config)` keeps the existing tuple result pattern,
  adding an integer excluded-player-death count. All callers pass a run end;
  private integration passes the real `run.EndMs` and independently checks counts.
- `TestRunEndClockSpike` first failed with `short deaths=2 long deaths=2`.
  It now reviews the valid tail at 2050 for run end 2100 (fight end 2000),
  excludes/counts the death at 50000, and reviews both with run end 60000.
  The tail test also pins inclusive run-end equality.
- `TestRecapBossKeepsRecentTen` first returned all twelve swings (6000..8200).
  It now requires exactly the last ten (6400..8200), including their order.
- Five additive mutants remove the run filter, change its equality, drop its
  counter, remove the evidence cap, or retain the oldest rather than latest ten.
- No model struct, fights, logfmt, cfg or rules changes were needed.

Verification after these changes: raid-tree gofmt clean, go vet and uncached Go
suite pass; raidtest Go modules 8/8, Lua/Python modules 7/7, death mutants
109/109 killed (no refusals/survivors). Gated R6 real-log test passes: each log
has one unresolved/low death, 15 feigns, zero wipes and zero past-run exclusions.
The synthetic clock-spike test supplies positive exclusion coverage absent from
these logs. The constants commit separately killed 42/42 model mutants.

The older pending-contract and pending-cap assertions are replaced, not reported
as fixes merely because their old observational tests passed. The removed
no-living-tank puller alternative remains removed per the earlier ruling.

## Round-2 delta (supersedes the earlier API/order notes)

The small prerequisite is scoring 1.5.0: required positive int64
`activity_cast_cap_ms=10000`, strict type/null/required checks, detached scalar
Snapshot copies and rule README. `TestActivityCastCapConfig` was RED for unknown
positive values and accepted omission; the seed/copy test was RED for the absent
seed. The cfg suite is GREEN, the race module took 16.3s (final all-Go run 16.2s),
under the unchanged 30-second limit. Both new guard mutants were killed;
**172/172 cfg mutants killed**, no refusal/survivor.

- The three adapted reviewer probes `TestSecondPostDeathClockRollback`,
  `TestSecondHolderLogOrderClockRollback`, `TestSecondHolderRollbackThroughFights`
  were RED: false pulled-aggro/lost-boss, high confidence. All are GREEN with
  unresolved/low/undecided, empty owners/suggestion; the real fights.Build
  monotonic control remains GREEN. Events are never sorted in production death
  views. Returned deaths alone retain stable time-order presentation.
- `TestClockRollbackDeclinesEveryRule` was RED for avoidable, unhealed,
  suggestion, wipe and below-lower-time-bound clock steps. It is GREEN with
  attribution suppressed before rule dispatch. Scope controls preserve valid
  attribution for equal times, earlier-life/before-window and after-death steps.
- `TestReviewRequiresPositiveRunEnd` was RED against the old three-result API.
  Review now returns `(deaths, wipes, excluded, error)` and rejects zero/negative
  ends before the index-only fast path. Every caller checks errors.
- Two reported survivor guards have independent tests and mutations:
  `TestLostBossOnlyHolderDamageRepairsGap` kills removal of holder ownership;
  `TestPulledAggroDoesNotAlsoMatchLostBoss` kills removal of the pulled-aggro
  exclusion (otherwise confidence incorrectly becomes medium).
- `TestLastHolderLineOutsideWindowIsNotSkipped` was RED with an older tank
  incorrectly substituted for the actual last, too-old non-tank target. GREEN:
  choose the last target by position first, then validate its age.
- Legacy time-predicate fixtures explicitly construct monotonic logs via the
  copying `chronological` helper. No expected cause, confidence, ownership or
  boundary assertion was relaxed; tied-line order is stable. Raw positional and
  rollback tests never use that helper. Three inherited mutant anchors were
  migrated to the linear scans; all inherited mutants remain.

Executed sequentially, with Go on PATH and CI Python 3.14 requirements:

```text
(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)
PASS; formatting and vet silent
python3 -I -B tools/raidtest.py --all --kind go
RESULT: 9/9 modules passed (cfg 16.2s; death 4.3s)
python -I -B tools/raidtest.py --mutants death
RESULT: 118/118 mutants killed
python -I -B tools/raidtest.py --all --kind lua,py
RESULT: 18/18 modules passed
```

Gated real-log test before: PASS (1.59s); after: PASS (1.65s).
For **each** of anonymous logs 1 and 2, before and after: deaths 1,
unresolved 1, low 1, empty suggestion 1, feigns excluded 15, wipes 0,
past-run exclusions 0, health samples 0. **Neither death result changes.**
These logs still supply no positive-cause/wipe calibration. No model/fights/logfmt
edit, native/game test, full-desktop suite, push, PR or release was performed.

## Round-3 prerequisite-history correction

Base: `9ffa9c26ae9b79446033e5fd3555daf5533c96a7`. One correction commit includes
the ruled cfg module budget override; all reviewer evidence was read-only.

- `TestPreWindowHistoryRollback` was RED with false pulled-aggro or lost-boss
  charges for the exact 9999/9998 opener, 6000/5999 engagement and 1000/999
  opener with death at 11002. It is GREEN for both test and shipped config,
  including multiple step sizes, earlier-life history and positive controls.
- One inclusive log-index `monotonic` helper now checks the full prerequisite
  range of each positional rule. The module README gives the complete range
  table. Wipe, opener, holder and healer survival read the fight prefix;
  avoidable/environmental/one-shot stay life-window scoped. Raid-wide evidence
  checks the full selected input because its symmetric scan reads later lines.
  Separate history/survival and future raid-wide probes were RED then GREEN.
- `TestFutureWipeCallLine` was RED: the later tank-death line stamped 9998
  retroactively labelled the victim at 10000 after-wipe/medium. GREEN preserves
  avoidable/high. The call now carries its log index; the strict timestamp
  boundary remains as well. No model change was needed.
- `TestClockPerturbation` was RED: a single timestamp change turned a victim's
  pulled-aggro charge into a different tank's lost-boss charge. GREEN on all
  40000 seeded fights: baseline causes pulled-aggro 4418, lost-boss 11665,
  unresolved 23917; after perturbation unresolved 40000, low, no owners or
  suggestion. Native fuzzing also passed 5734 executions in 15 seconds.
- Both reviewer survivors are behavioral kills: starting the guard at `from+2`
  fails `TestClockGuardFirstAdjacentPair`; deleting holder `e.TimeMs > at ||`
  fails `TestLostBossRejectsFutureStampedHolder`. The latter directly exercises
  the defensive rule beneath dispatch, where the full-prefix guard cannot mask it.
- The first complete mutant run killed 122/124. The new full-history guard
  masked two inherited tests, not production defects: the unowned-guardian
  fixture appended earlier lines after death, and the last-holder fallback
  fixture deliberately rolled back. The former now explicitly constructs a
  chronological fixture and adds a positive source control, without changing
  its existing assertion. The latter retains its public test and gains a
  direct rule-level test. All 118 inherited mutants remain; only helper-move
  anchors 112/113 and the killing test for 118 were migrated. Final: **124/124**,
  no survivor or refusal. Six entries are additive.
- Runner tests were RED on ignored/invalid per-module limits, then GREEN.
  Only `test/raid/cfg/module.json` declares `limit: 60`; absent overrides still
  use 30 seconds. Acceptance at 60, refusal at 61, subprocess budget, invalid
  forms and cfg-only scope are pinned. Named-mutant deadlines are unchanged.

Executed with Go on PATH, CI Python 3.14 requirements and the shared heavy-job
lock for race/module/mutant runs:

```text
gofmt -l internal/raid; go vet ./internal/raid/...     silent
go test -count=1 ./internal/raid/...                 PASS (death 7.538s)
raidtest.py --all --kind go                         9/9 (cfg 13.8s, death 25.8s)
raidtest.py --all --kind lua,py                     18/18
raidtest.py --mutants cfg                           172/172 killed
raidtest.py --mutants death                         124/124 killed
go test -run '^$' -fuzz '^FuzzClockPerturbation$' -fuzztime=15s -parallel=2 ./internal/raid/death
PASS: 5734 fuzz executions
```

Gated real-log before/after PASS (1.60s / 1.48s). The four emitted JSON rows
compare equal. Each of two anonymous logs still has one unresolved/low death,
empty suggestion, 15 feigns, zero wipes, zero past-run exclusions and zero
health samples; candidate/share remains null. These logs still cannot validate
positive personal charges. No model, fights, logfmt, dependency, native/game,
full-desktop, remote, release or installation change was made.
