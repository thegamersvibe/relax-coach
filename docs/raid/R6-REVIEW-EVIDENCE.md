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
