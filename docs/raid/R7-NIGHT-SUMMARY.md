# Transient night role and part values

`score.NightRoleParts(fights []model.Fight, night model.NightScore,
c cfg.Config) (score.NightSummary, error)` derives one player's comparison
inputs without modifying or storing anything. Call once per `NightScore`.
Use the original fights and the same configuration used for scoring; a missing
fight returns an error, never an assumed trash weight.

`NightSummary.Role` is the role with the largest **positive** sum of fight
weights among scored fights. `Split` means equal positive leading role weights:
role is empty and no parts are returned. No scored fights, or no role with
positive total weight, yields no role or parts and `Split=false` (no data).
In particular, a player seen only in wipes with configured `wipe_weight=0` is
not a role split, even if more than one zero-weight role was observed. Only the
positive maximum matters, not a tie between two lesser roles.

`NightSummary.Parts` holds exact `*big.Rat` values in **points**, not tenths.
The returned rationals and map belong to the caller. Part values are the weighted
means of measured parts in the chosen role, separately for boss and trash,
joined with the night shares. A measured part of an otherwise unscored fight
is still measured; an absent part is skipped, never zero. Missing groups are
normalized away. Neither coverage nor duration is an additional part weight.
The comparison computes exact role medians and deltas before display rounding.

Both consumers share `fightWeight`: trash 1; boss kill weight unless explicitly
failed, then wipe weight; multiply by short-fight weight when flagged.
Both consumers share `nightShares`: configured trash share capped by maximum
trash share and the complement of minimum boss share. Decimal configuration
values are converted to rationals **before** the complement is computed. This
is an explicitly accepted correction, not an output-preserving change for all
valid configs. The reviewer case `.2/.2/.8`, one offset-zero trash fight, tank
mechanics weight 100 and measured death coverage 75 now yields exact night
coverage 15 and provisional score 100.0 (no rank); the old float complement
yielded coverage 14 and no score. `TestNightExactDecimalTrashCap` pins that case
through the real config loader. Its float-complement overlay fails with the
old 14/absent result; the retained exact implementation passes at 15/100.0.

## Preparation clocks

Selection compares server `Stamped` against true UTC fight-end seconds,
using the first event's `OffsetH`. The latest at/before-end report is eligible,
including a receipt exactly at the end; same-stamp ties retain sender-clock then
stable full-report JSON ordering. A future report is never used, for gear **or
stock**. With no eligible report, preparation is wholly unmeasured: no earliest
fallback, no selected receipt clocks, no credit or charge. This intentionally
replaces the old night-wide stock/fallback policy: miss over false claim.
`fight_end_utc_s` accompanies the selected receipt and sender clocks in evidence.
PC clock error beyond the zone offset remains uncorrected. No-report and
future-only scores have the same evidence shape and coverage.

## Original verification (before S1–S3)

- `TestPrepUTCClockSelection` failed on the old implementation at offset +3:
  selected 79360 instead of 75760; green with the fix. Offset -4 and zero,
  inclusive end, evidence and first-event authority are covered too.
- Night summary tests were run red before each behavior: single value,
  weighted role/equal split, exact grouped part means, missing metadata.
- Real-log replay uses actual fights and actual offset 3, with **synthetic**
  receipts at UTC end and one hour later. With the frozen old preparation
  function in a Go overlay, after-end selections were 340/340 and 331/331.
  With the fix they were 0/340 and 0/331. This is not evidence of real addon
  report timing: no real prep store accompanied the logs.
- Existing real-log checks still see one dungeon run and 13 bosses per log,
  five unique players each (3 normal, 2 provisional), no dungeon ranks,
  one classified death each, and deterministic evidence.

- Score mutation pack: **144/144 killed**, none survived or refused in the
  final run. One earlier mutant had an int/int64 compilation error; only that
  mutant expression was repaired before rerunning the whole pack.
- `go vet ./internal/raid/...` and `go test -count=1 ./internal/raid/...`
  passed. The module runner returned **9/10**, because unchanged `death`
  tests passed but took 33.4 seconds against its 30-second budget; a bounded
  `GOMAXPROCS=2` retry still took 31.8 seconds. No deadline was changed.
- Final diff self-reviewed; independent custodian review remains required.

## S1–S3 follow-up verification

The follow-up implements the lead's three rulings; it does not restore the
reviewer's proposed zero-weight role label or the former float-cap result.

- **S1:** `TestNightRolePartsZeroWeightIsNoData` loads `wipe_weight=0` via
  `cfg.Load`. Before the guard, single-role and two-role wipe-only players
  reported `role="" split=true`; both failed. With the positive-weight guard,
  both return no role/parts and no split. The positive-role control and existing
  positive-tie tests pass.
- **S2:** `TestNightExactDecimalTrashCap` passes with the accepted exact cap.
  A Go overlay restoring the float complement fails with
  `night_score={0 false} night_coverage=14`; the retained implementation returns
  `night_score={1000 true} night_coverage=15 rank={0 false}`.
- **S3:** `TestPrepOnlyAfterEndIsUnmeasured` failed before the fix with measured
  preparation 87.5, part coverage 60 and night coverage 23 instead of 17. It now
  matches the complete no-report result, including absent receipt clocks.
  Whole/fractional end times, positive/negative/zero offsets, exact-end inclusion,
  mixed reports and ordering are covered. `TestPrepFutureStockCannotCreditEarlierFight`
  independently failed after only removing the fallback: future stock still
  raised empty-bag prep from 50.0 to 100.0, or unreadable-bag coverage from 30 to
  60. Applying the cutoff to stock makes both controls pass.
- The reviewer's unchanged `TestReviewFutureOnlyPrepIsUnmeasured` and
  `TestReviewFutureStockCannotCreditEarlierFight` were also replayed via Go
  overlays: both fail against the frozen pre-fix preparation implementation and
  both pass against the candidate. Reviewer evidence files were read-only.
- The inherited earliest-fallback assertion was deliberately migrated to
  unmeasured/no-clock assertions. The same-stamp tie test now uses an eligible
  exact-end receipt instead of a future fallback; its ordering assertions remain.
- Mutation manifest: six added entries plus replacement of the obsolete
  earliest-fallback mutant with one that restores the forbidden fallback.
  All anchors are unique. Actual runner output: **`RESULT: 150/150 mutants killed`**,
  with no survival or refusal.
- Required Go checks: `gofmt -l internal/raid` produced no filenames;
  `go vet ./internal/raid/...` exited 0; `go test -count=1 ./internal/raid/score/`
  passed. `python3 -I -B tools/raidtest.py --all --kind go` returned
  **`RESULT: 10/10 modules passed`**, including the race-enabled score module.
  `TestImports` and `TestNoLargeLiterals` also passed explicitly.

### Counts-only real-fight replay before/after

`TestRealLogsR7` uses the real fights/offsets with **synthetic** receipts. No real
prep store accompanied either log: actual prep reports and unmatched counts
remain zero before and after. The augmented test failed before the S3 fix and
passed after it. Rows below are player/fight preparation rows, not unique players;
coverage sums add stored hundredths across those rows, not a night average.

| Log | Replay | Rows | Measured before → after | Prep coverage sum before → after | Fight coverage sum before → after |
|---|---|---:|---:|---:|---:|
| 1 | No reports | 345 | 0 → 0 | 0 → 0 | 18119 → 18119 |
| 1 | Future only | 345 | 340 → 0 | 20400 → 0 | 20159 → 18119 |
| 1 | At end + future | 345 | 340 → 340 | 20400 → 20400 | 20159 → 20159 |
| 2 | No reports | 336 | 0 → 0 | 0 → 0 | 17633 → 17633 |
| 2 | Future only | 336 | 331 → 0 | 19860 → 0 | 19619 → 17633 |
| 2 | At end + future | 336 | 331 → 331 | 19860 → 19860 | 19619 → 19619 |

The mixed replay still selects 340/331 at-end reports with zero after-end
selections. Both logs retain one dungeon run, 13 bosses, five unique players
(3 normal, 2 provisional), no dungeon ranks and one classified death. The
no-report night coverage values remain 17/29/73. These synthetic timing controls
do not establish actual addon report availability or native game behavior.

No page, config, model, parser, fight or death implementation changes. README
and dependency files are untouched. Native client testing was not run.
