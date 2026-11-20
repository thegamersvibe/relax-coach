# Officer comparison (REQ-SCO-14)

The `compare` module is R8's non-UI comparison. The package adds only
`model`, `cfg` and `score` to its raid import allowlist; there is no new Go
module dependency and no page, path picker or stored-night change.

```go
ParseOfficer(data []byte) (OfficerFile, error)
Compare(scores []model.NightScore, roster []model.Actor, c cfg.Config,
    officer OfficerFile, fights []model.Fight) (Report, error)
```

The caller chooses/reads the officer file and supplies the same run and config
used to score it. Neither function performs I/O. Invalid officer data returns
an error and no comparison, including typed files passed directly to Compare.
Parsing enforces case-sensitive keys, one JSON object, no duplicate keys,
valid Unicode without replacement, version 1, and 1–5 distinct nonempty names
in each list. ASCII-case variants count as repeats across both lists.

Names resolve through the roster using `score.MatchPrep`: exact full spelling
first, then unique ASCII-lowercase spelling, without trimming or realm removal.
An ambiguous roster identity is not compared. Unmatched names say
`not in this night`; roster players without ranks say `no rank: NN% measured`.
Neither gets a verdict. With fewer than four ranked players, Report contains
only the officer label, ranked count and `fewer than 4 ranked players` reason.

Only supplied ranks determine halves (upper half includes `ceil(N/2)`). Officer
list order is preserved. Own top/bottom fives are selected **before** excluding
names in either officer list, without backfill or verdict. Equal ranks sort by
logged name then ID; bottom five runs worst first.

Night roles and exact part scores come only from `score.NightRoleParts`.
`TestCompareFightsUseGuard` parses every production file: the only permitted
fight type is Compare's fifth parameter, and its only permitted reference is
the direct first argument of that score function. Guard controls reject reads,
indexing, iteration, aliases, mutation, forwarding through other functions,
another fight type in another file and a missing forwarding call.

For disagreements, each measured part needs two ranked same-night-role peers
with values (the player included). Medians, configured decimal weights and
weighted deltas remain `big.Rat`. `Impact = (Score - Median) * Weight`, where
Weight is the configured percentage number; missing parts are not renormalized.
Top selects the smallest delta, bottom the largest; ties use larger role weight,
then ASCII part name. Round the returned exact rationals only for display with
`FloatString(1)`. Role ties and missing comparable parts have explicit reasons.

Tests cover parsing, half boundaries, provisional/unscored/missing names,
case collisions, no identity guessing, own-five exclusions, role availability,
odd/even exact medians, all role weights, tie breaks, original kill/wipe/trash
weight propagation, input ownership, deterministic ordering and the AST guard.
`mutants.json` contains behavioral replacements run by the standard raid runner.

`TestRealLogsComparison` runs only with `RELAX_RAID_REAL_LOGS` set. It decodes,
segments, reviews deaths and scores real logs before comparing a **synthetic**
officer file. It prints ordinal/counts only. The supplied dungeon logs have no
ranked players, so this verifies `no report`, not real-raid disagreement accuracy.
