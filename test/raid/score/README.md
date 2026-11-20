# Score module

`tools/raidtest.py` auto-discovers this module's `module.json`; no runner switch
or production import is added. `depends` includes the external integration
readers; production imports remain only model and cfg (plus stdlib).

- `python3 -I -B tools/raidtest.py score`: synthetic unit tests under `-race`.
- `python -I -B tools/raidtest.py --mutants score`: compile-valid behavior
  mutants, checked against their named passing baselines through Go overlays.
- `RELAX_RAID_REAL_LOGS=<private-folder> go -C desktop test -count=1 -v
  -run '^TestRealLogsR7$' ./internal/raid/score`: opt-in counts-only integration.

No private fixtures are committed. The real test emits ordinal log labels,
counts, coverage sets and active-share distributions, never names, IDs or paths.
Real R6 classifications and final run-end errors are consumed by the integration
test. Score uses its returned healer owners, never its own survival replay.
Round-2 tests cover same-ms ownership, exact decimal shares, incoming-only
players and configured cast caps. Round 3 requires a nonempty exact owner name
mapping to one distinct roster ID across the entire fight, not merely its
healers or living subset. Empty, unknown or ambiguous names yield no charge
for that death; the scorer does not guess an identity or replay survival.
Joined tests include duplicate names, a nonhealer collision, and repeated rows
for the same ID as a positive control. The manifest has 130 behavioral mutants.
RANGE cannot be separated from SPELL in model.Event; the optional unmatched
range/cast check is explicitly skipped rather than guessed from spell IDs.

Tests live beside the package as external Go tests; the module is the
registration and mutation boundary. See `docs/raid/evidence/R7.md` for results.
