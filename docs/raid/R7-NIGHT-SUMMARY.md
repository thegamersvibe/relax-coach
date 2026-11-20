# Transient night role and part values

`score.NightRoleParts(fights []model.Fight, night model.NightScore,
c cfg.Config) (score.NightSummary, error)` derives one player's comparison
inputs without modifying or storing anything. Call once per `NightScore`.
Use the original fights and the same configuration used for scoring; a missing
fight returns an error, never an assumed trash weight.

`NightSummary.Role` is the role with the largest sum of fight weights among
scored fights. `Split` means equal leading role weights: role is empty and no
parts are returned. No scored fights also yields no role or parts, without
`Split`. Only the maximum matters, not a tie between two lesser roles.

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
values are converted to rationals **before** the complement is computed.

## Preparation clocks

Selection compares server `Stamped` against true UTC fight-end seconds,
using the first event's `OffsetH`. It still falls back to the earliest report
when none precedes the end. `fight_end_utc_s` accompanies the existing receipt
and sender clocks in evidence. PC clock error beyond the zone offset remains
uncorrected. No-report scores keep their prior evidence shape.

## Verification

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

No page, config, model, parser, fight or death implementation changes.
