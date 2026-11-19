# R4 / D1 implementation and verification ledger

Scope: fights consumes model values; package main finds/open logs read-only.
No model edits, real log contents, dependencies, upload, UI or installation.

## Interface and rulings

- The initial base lacks later model fields; this first slice uses a local
  result sidecar for source, byte ranges, spec, header metadata and timelines.
  Model addendum 2 is now integrated: model.Fight receives byte ranges and
  specs; model raid/dungeon runs receive source/build/log-version/advanced.
  Detail retains convenience mirrors and per-player timelines.
- Power was initially held out by the parent; the subsequent model-addendum
  instruction brought it back in. Power, maximum power and power type are
  sampled for InfoUnit, including present zero mana and absent values.
- Use the brief's cast-signature hook, not PLAN's prospective talent table.
  An empty profile/signature set means unknown, not a guessed spec.
- Use the brief/R5 instance classifier: listed kind wins; an unlisted large
  group may be raid with Listed=false. PLAN 5B's blanket unknown-for-unlisted
  text is superseded by the brief's raid fallback. No dungeon is guessed.
- Follow the brief's newer boss max-health rule and replacement selectors;
  never use the current-target flag as a boss marker.
- Gaps use hostile combat evidence (damage, misses, interrupts, dispels), not
  idle aura/heal/cast chatter. The exact configured gap stays in one pull;
  a strictly larger gap starts another. Boss tail is inclusive and is cut by
  a new encounter start, header, or zone boundary. A return to the same instance
  continues the run only when zone-out to zone-in is strictly under
  `Params.ReentryGapMs` (caller default 1800000 ms) with no intervening non-zero
  instance entry. The exit still closes the fight/tail; outside lines and zone
  markers never enter fight events. A header always starts a new run.
  Run `EndMs` keeps the existing zone-out timestamp convention: the final exit
  after any accepted returns, or the first exit if there is no return. Repeated
  outside-zone changes do not move it or restart the gap. Fight events end at
  the last included line inside the instance. Empty detours emit no run; a
  detour with fights emits its own run (two versus three runs for A/B/A).
- A first non-encounter pull with no observed quiet interval is conservatively
  partial/unscored. EOF/header cuts mark the run partial, not a completed boss.
- File-local ids use `anchor:position`; complete boss ids use server fields
  and sorted distinct combatant ids only. Trash is never cross-log matched.
- A malformed complete pair with an absent optional server number uses an
  explicit `unknown` token, not zero; its duration remains absent.
- Main glue accepts the selected installation path; manual path bypasses
  discovery. Discovery uses modification time, ties by path, regular files only.
  No live folder rotation has been observed. R8 must connect the UI.
- M14 index mode uses DetailsFor to omit unselected events/timelines in output;
  the stream adapter buffers decoded events for one run, not raw file bytes.
  It is not a bounded-memory parser for an arbitrarily long single run.

## Tests-first slices on the original base

All commands below were actually executed. Package-write tool lint runs from
outside the Go module and emits spurious import errors; the authoritative
checks run inside desktop.

1. Boss identity: RED `undefined: Params`; GREEN raid vet/test exit 0.
   Literal expected SHA input, repeated parse, shifted time/offset and failed
   casts, server duration independent of local clock.
2. Window/run splitting: RED boss+pull count 1 instead of 3, incomplete
   encounter lost, end was encounter end rather than zone-out, header runs
   missing. GREEN raid vet/test exit 0. Covers exact tail/gap, next-start
   ordering, partials, unknown instances, thirteen pairs and missing first boss.
3. Actors/timelines: RED missing Details, then owners/boss roles empty and
   absent timeline sidecar. GREEN fights tests after ownership buffering,
   header reset, same-name pets, replacement selectors, landed-only roles,
   overheal, HP/position subjects, true/feign deaths, spec hook, logger/header
   and multi-boss remaining HP. The first integration run exposed the numeric
   guard on two flag masks; using the protocol's two-bit type field fixed it.
4. Stream/typed output: RED `undefined: Read`; GREEN raid vet/test exit 0.
   Hashes full raw bytes including unread/skipped tails, propagates errors
   without partial result, filters detail selection, refuses wrong run type.
5. Main discovery: RED undefined raidLogChoices/openRaidLog; GREEN
   `go test -count=1 -run '^TestRaidLog' .` => `ok .../desktop`.
6. Review edges: RED first inferred instance overwritten, pet incoming damage
   charged to its player, six invalid configs accepted. GREEN raid vet/test
   exit 0. Equality, missing duration, orphan end, empty stream covered too.

## Harness

`test/raid/fights/module.json` is auto-discovered by the existing runner's Go
`--all` and changed-model dependency selection; runner source needs no change.
CI adds fights mutants and the focused package-main discovery suite.

Initial final-run outputs:
- raid `go vet` + `go test -count=1 ./internal/raid/...`: exit 0, fights/model/rules ok.
- focused main tests: `ok .../desktop 0.015s`.
- `python3 -I -B tools/raidtest.py --all --kind go`: `RESULT: 3/3 modules passed`.
- uv Python 3.14 runner `--all --kind lua,py`: `RESULT: 4/4 modules passed`.
- Initial mutants: `RESULT: 15/17 mutants killed`. One unused-variable build
  refusal was fixed to a runnable mutation; one effective-healing survivor
  exposed a missing numerical assertion, now added without changing production.
  Follow-up verification: `RESULT: 17/17 mutants killed`; raid vet/test exit 0.

## Model-addendum follow-up

The initial slice was committed as 8e4ea59b7a60f666a3d81741735944b303652c64.
It was rebased onto the supplied model commit
5020a0dd5734ee5db13c7f2ce137db7babf1a982 as
4a200b55873f8091f997dcc7e20f5bf81aba36c8. No backup tag was created under
the standing no-tags rule; the original SHA is retained here. The first rebase
attempt stopped for missing committer identity; replay completed with the
authorized per-command identity. No shared model file was edited by this lane.

- Power/range/spec/source RED: missing Sample.Power, then power-only sample
  missing, model fight fields missing and model run source missing. GREEN:
  raid vet/test exit 0 after projection into the new fields.
- Contract review RED: zone instance replaced fight-start instance, empty
  signatures admitted a guessed spec, typed advanced block ignored. GREEN:
  raid vet/test exit 0. Positive signature fixtures now explicitly supply a
  synthetic signature; the unknown-signature regression keeps an empty list.
- Source-error RED: a decoder swallowing an I/O error could publish a partial
  raw hash. GREEN: the counting reader remembers non-EOF source failures.
- Partial/progress RED: a zone-truncated boss did not mark its run partial;
  a stale HP sample could resurrect a dead boss in wipe progress. GREEN:
  run partial propagation, cut trash partials, and dead-unit zero health.
- REQ-PAR-04 power is implemented, no longer held open. Real-log event-family
  golden sampling is still not run. REQ-PAR-06 class remains open: model has
  no typed class field, and fights does not interpret COMBATANT_INFO.Raw.
- PHASE1-BRIEF currently has the power rows in step 0 but no separate heading
  titled “Model addendum 2”; the supplied model revision and the updated
  step-0/v2 text were used. This did not block implementation.

## Final verification on model addendum 2

Commands (Go PATH configured as directed; PY is the supplied uv Python 3.14
invocation with `.github/requirements-ci.txt`):

- `(cd desktop && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)`
  => exit 0; `ok .../internal/raid`, `ok .../internal/raid/fights`,
  `ok .../internal/raid/model` (modeltest is a builder package with no test files).
- `(cd desktop && go test -count=1 -run '^TestRaidLog' .)`
  => `ok github.com/thegamersvibe/relax-coach/desktop 0.010s`.
- `python3 -I -B tools/raidtest.py --all --kind go`
  => `RESULT: 3/3 modules passed` (includes race-enabled fights/model commands).
- `$PY -I -B tools/raidtest.py --mutants fights`
  => `RESULT: 27/27 mutants killed`, no survival or refusal.
- `$PY -I -B tools/raidtest.py --all --kind lua,py`
  => `RESULT: 4/4 modules passed`.
- `git diff --check`: exit 0. Model path diff against the supplied model base:
  empty. PAR-08/PAR-09 requirement text unchanged.

## Not run / integration boundaries

- Private logs: not run, R3 decoder absent. TestRealLogsR4 is gated on
  RELAX_RAID_REAL_LOGS plus decoder/config/prefix hooks. It emits only counts
  or fixed failures; no private path, name, line or id. Expected prefix is
  supplied privately, never committed. The expected 13 bosses/15 feign lines,
  cross-logger identity and zone-out properties remain unverified on real data.
- No full desktop suite, native Windows/game checks, upload, push, PR or tags.
- D1's existing model compile-fail type gate runs, plus checked conversions;
  an actual night scorer and raid-history writer are absent, so their negative
  dungeon admission/baseline integration tests remain downstream work.
- No independent reviewer tool is available in this lane. Self-review only;
  parent/custodian review is still required.
