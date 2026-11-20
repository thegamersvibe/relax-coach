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
R6 is absent from this branch, so classified deaths are deliberately empty.
RANGE cannot be separated from SPELL in model.Event; the optional unmatched
range/cast check is explicitly skipped rather than guessed from spell IDs.

Tests live beside the package as external Go tests; the module is the
registration and mutation boundary. See `docs/raid/evidence/R7.md` for results.
