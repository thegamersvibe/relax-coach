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

The late lead request was tested before changing any behavior. The actual
`fights.Build` path retained a far-ahead death at 50000 in a boss whose end was
2000 and whose run ended at 2100. It also retained a legitimate tail death at
2050. The requested exclusion test failed:

```text
--- FAIL: TestRunEndClockSpikeExclusion
run end=2100 fight end=2000: reviewed 2 deaths; want only the in-run tail death at 2050
```

`model.Fight` (`model.go:159-182`) has no run start/end field. Its `EndMs`
is the fight bound, and the documented post-end tail belongs to its events.
`fights.Run.EndMs` (`fights.go:46-63`, finalized by `flush`) is not passed to
`death.Review`; even the two model run types currently omit a final end.
The referenced NIGHT-FILE line 120 on this base is `encounterId`, not the
outside-run clause; the brief's "deaths outside fights" ruling supplies that
policy. The contract note now records it and the missing input explicitly.

Following the lead's instruction not to guess when information is unavailable,
no `TimeMs > Fight.EndMs` filter was added. It would drop the valid tail too.
`TestRunEndClockSpikePendingContract` now documents the gap: real segmentation
at run ends 2100 and 60000 produces **identical Fight inputs**, but the desired
scoped counts differ (1 versus 2). Both calls to the current fight-only Review
return 2, as pinned. `TestFightEndDoesNotExcludeValidTailDeath` protects the
legitimate tail. The pending-contract test passing is **not** an exclusion fix.
An approved run-aware caller or shared-input/API change is required outside
this lane's permitted model/fights scope. No model, fights or logfmt edit was
made. The full raid Go suite and all 8 Go modules passed after these tests.

### Pending ruling

Should special ambiguous-aggro boss-swing evidence also be capped at ten?
The brief explicitly assigns boss swings as that evidence, while M22's recap
uses the last ten. Current special evidence remains twelve in the reviewer's
case, pinned by `TestZZRecapBossCapPendingRuling`; ordinary recap remains capped
at ten. No behavior was guessed for this question. Lifetime scope is no longer
pending. `TestZZAggroAtAttackTime` intentionally remains unresolved because the
architect removed the no-living-tank alternative, not because its old expected
puller charge was implemented.
