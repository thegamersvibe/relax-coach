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
counting and unowned guardians. `mutants.json` has 104 behavioral mutants;
all must be killed by their named test, not by compiler errors.
The observed final race-enabled module run was 2.6 seconds against the unchanged
30-second runner deadline. The all-Go runner passed 8/8 modules and the
Lua/Python runner passed 7/7.

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
The latter is not fixed: `Fight.EndMs` cannot substitute for `fights.Run.EndMs`.
The first death mutation run killed 90/91; adding a prior alternate holder
made the remaining last-target mutant observable. The final pack killed
104/104, including 44 additions to the 60 inherited entries. Obsolete anchors
were migrated to the replacement predicates, never counted as compiler kills.

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
