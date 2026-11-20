# R6 death review checks

The runner discovers `module.json` automatically; production imports remain
`model` and `cfg` only. `fights` and `logfmt` are external-test dependencies.
Committed events are synthetic `modeltest` values or synthetic parser controls. Nothing is tested in
the game, and no real line, actor name, id or path is recorded here.

## Gates

From the repository root, with Go on PATH and the CI Python environment:

```sh
(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)
python3 -I -B tools/raidtest.py --all --kind go
python -I -B tools/raidtest.py --mutants death
python -I -B tools/raidtest.py --all --kind lua,py
```

The module runs race-enabled synthetic tests, including seven cause paths,
feign/pet exclusions, inclusive window edges, sample ownership, sparse
suggestions, full-health resets, whole-span healer survival, healing/absorb
0/1, strict majority with overkill removed, role-specific/empty mechanic roles,
raid-wide shares 0.59/0.6, two versus three targets, source/swing/mechanic
exclusions, alive-at-start denominator, pet attribution, lost-boss gap,
wipe call time and summary boundaries. Round-1 regressions add bounded
fight-wide openers, proven roles, observed holders, missed swings, log-order
survival, exact death-line/life cutoffs, environmental provenance, confidence
counting and unowned guardians. Round 2 adds raw clock-order, nonpositive-run-end
and survivor regressions. Round 3 adds prerequisite-history guards, future-wipe
line exclusion, seeded timestamp perturbation and both reviewer survivors.
`mutants.json` now has 124 behavioral mutants;
all must be killed by their named test, not by compiler errors.
The initial race-enabled module run was 2.6 seconds. Round 3 passed in 25.8
seconds against the unchanged 30-second death-module deadline, with the full
40000-fight perturbation test. Current all-Go and Lua/Python totals are 9/9
and 18/18; historical runs are retained in the evidence document.

Each rule was exercised RED before implementation, then GREEN. The first
mutation pass exposed eight masked boundary tests and two uncompilable
mutations. The tests were strengthened and the mutations made compilable;
no mutant was removed. The next pass killed 60/60. Self-review also caught
foreign-target recap evidence, an environmental/earlier-mechanic precedence
case, and incomplete multi-boss swing evidence, each verified RED→GREEN.

Round-1 is recorded separately in [R6-REVIEW-EVIDENCE.md](../../../docs/raid/R6-REVIEW-EVIDENCE.md),
including actual RED→GREEN output, authorized fixture/expectation migrations,
all surviving reviewer-hand-mutant guards, the pending evidence-cap ruling
and the reproduced run-end input gap (`TestRunEndClockSpikePendingContract`).
The run-end gap is now fixed by an explicit final run-end argument plus an
error for a nonpositive end; `Fight.EndMs` cannot substitute for it. The later
round-2 section records current counts and chronology behavior.
The first death mutation run killed 90/91; adding a prior alternate holder
made the remaining last-target mutant observable. The final pack killed
104/104, including 44 additions to the 60 inherited entries. Obsolete anchors
were migrated to the replacement predicates, never counted as compiler kills.

## Chronology (round 4: whole-fight decline)

One backward timestamp step anywhere in the selected fight's original log order
(`monotonic(events)` over every adjacent pair, unrelated lines included; equal
timestamps are valid) makes **every death in that fight** `unresolved` / `low` /
`undecided`, with empty owners and no suggestion. It is checked once per fight
(`fightView.clock`) before any rule runs: no rule runs and nothing falls through
to a later rule. The round-3 per-rule spans are deleted, because declining one
rule let a later one charge (the reviewer's wipe -> avoidable/victim and
raid-wide -> unhealed/healers repros).

The range is the whole fight, not only fight start through the death line: the
raid-wide subrule reads same-spell hits and survival across the fight, including
lines after the death, so a post-death step must decline too (reviewer repro
with 20002/20001; the 20002/20002 control keeps its verdict). A miss is
preferred to a false charge: a fight with clock jitter gives unresolved deaths.
`TestClockPerturbationCauseShapes` perturbs one line of clean wipe, avoidable,
raid-wide and unhealed fights (8000 cases) and requires the declined result.

A clock-stepped wiped boss still returns its summary row (when the summary
window is configured), but `FirstDeath = -1` and `Following` is empty. This keeps
the observed wipe without naming a first death from timestamps that cannot
establish order. The officer page uses the existing `firstDeath: null` and empty
`following` shape; it must not infer a first death from the presentation-sorted
death rows. `TestWipeSummaryClockStep` keeps raw deaths stamped 20000 then 10000
in log order and also covers an unrelated step after both deaths.
`TestWipeSummaryCleanClock` preserves the original summary for increasing and
equal timestamps. The summary clock-guard removal has its own named mutant.

The holder's `e.TimeMs > at` defensive guard is independently unit-tested below
dispatch, so the prefix guard cannot mask its deletion. Both reported survivor
mutants now have named behavioral killing tests; compiler errors are refusals.
Only cfg's module declares a 60-second budget. Death and every other module
retain the runner's 30-second default; named-mutant deadlines are unchanged.

## Private integration, opt-in only

Set `RELAX_RAID_REAL_LOGS` to the private fixture folder, then run:

```sh
go -C desktop test -count=1 -v ./internal/raid/death -run '^TestRealLogsR6$'
```

`TestRealLogsR6` is in the **external** test package. It adapts the real
`logfmt.Decode` and `fights.Read`, selects every fight's detail, then calls
`death.Review` with the shipped config. It verifies exactly one result per
real player-death line, byte-identical re-review, no dead healer ownership,
and no wipe summaries in these two logs. Only anonymous indices and counts
are printed: deaths, feigns excluded, cause/suggestion/confidence totals,
and per-death sample counts and candidate spell exposure. A null share
means no qualifying candidate was observed, **not** a measured zero share.
Synthetic diagnostics independently pin the sample count and 3/5 exposure.

Both private logs currently produce one unresolved, low-confidence death,
no suggested cause, and exclude 15 feigns. Each death has zero health samples
and no damage candidate in the configured review window, hence null raid-wide
share. No wipes were observed; wipe logic is fixture-proven, not calibrated
from these logs. The private logs cannot validate the positive death-cause
rules; synthetic boundaries and mutation controls do that. Live raids,
Windows rendering, officer overrides and result-file serialization are not
verified by this module.
