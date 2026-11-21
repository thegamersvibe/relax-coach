# R8 preparation clock glue and remaining comparison seam

## Implemented, no UI

`raidPrepScores(data []byte, fights []model.Fight, deaths []model.Death,
c cfg.Config) (raidPrepResult, error)` lives in `desktop/raid_prep.go`,
`package main`, where the architecture places SavedVariables handoff glue.
It takes the caller-selected file's bytes; no file discovery, page handler,
network path or persistent state is added.

- Converts each fight's wall-clock start/end with **its first event's** offset.
  Chooses earliest true UTC start and latest true UTC end, independent of slice
  order, including when offsets differ between fights.
- Returns the effective window `[floor(first UTC - 2h), ceil(last UTC)]` in
  server seconds. The actual `prepread.Parse` arguments are `floor(first UTC)`
  and `ceil(last UTC)` because **Parse already subtracts two hours**. Supplying
  the effective lower bound to Parse would incorrectly use a four-hour window.
- Passes the parsed reports unchanged to `score.MatchPrep` with logged roster
  names and to `score.Score` with the caller's deaths and config. Never shifts
  `Stamped` or `SenderClock`. Returns reader diagnostics and unmatched reports.
- Refuses empty fights, fights without a first event, negative UTC and reversed
  fight bounds with an error and no partial scores.

Tests exercise the real parser and scorer: lower bound minus one second,
exact lower bound, fractional-second end rounded up, upper bound plus
one second, full-name ASCII matching, unmatched reports, unordered fights,
positive/negative offsets, first-event authority, invalid bounds, invalid store
diagnostics, death forwarding and caller-input purity. They use synthetic
SavedVariables and actors only. Registered as `test/raid/raidprep`; no import
allowlist entry is needed for package-main glue.

## Blocked: four-input comparison

The ruled comparison takes `[]model.NightScore`, `[]model.Actor`, `cfg.Config`
and the officer JSON file. None contains the original fights' `Boss` and
`Success` fields. `score.NightRoleParts` needs those fields to use the same
kill/wipe/trash weights as night scoring. Fight-score flags do not encode them.

Concrete ambiguity: hold those four inputs fixed with one scored tank fight
and one scored DPS fight. With kill weight 2, making the tank fight a boss kill
and the DPS fight trash yields night role tank. Swapping just the original
fight metadata yields night role DPS. Both are legal, identical four-input
comparison calls, but require different role medians and disagreement parts.
No function over those four inputs can distinguish them.

Requested ruling: add `[]model.Fight` as a fifth comparison input (recommended),
or explicitly authorize another typed way to supply that metadata. No boss/
wipe assumptions, stored evidence keys, hidden state, extra officer-file keys,
comparison stub or page code has been added to conceal this missing input.
The full comparison reader, ranking, medians and comparison mutants remain
unimplemented until that interface is corrected.
