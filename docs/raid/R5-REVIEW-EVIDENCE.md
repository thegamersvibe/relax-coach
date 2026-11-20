# R5 review round 1 — evidence

Base: `d67098eee72f48c375cdf4ac2a2401c618aa1a09`, `feat/raid-cfg`, initially clean.
One follow-up commit; no model edits, root README release edits, push, PR or tags.
Review inputs: `findings-mani-r5.txt`, `p-mut-hand-a.log`, `p-mut-hand-b.log`
and the review probes. Study consulted directly from the supplied raid-analyzer
handoff's `class-study/` folder, not spell names reconstructed from game memory.

## Items and test-first evidence

Each test group below was added and run before its corresponding implementation
or seed change. Commands used `go -C desktop test -count=1 -run '<names>'
./internal/raid/cfg`; red exits were 1. Each implementation then received a full
`go -C desktop test -count=1 ./internal/raid/cfg` run. The final acceptance run
below includes all the groups, not just the first passing subset.

| Item | Change and red test(s) | Observed red | First successful cfg run |
|---|---|---|---|
| 1 | ID-only `Matches`, required positive selector id, optional label; `TestSelectorNPCIdentity`, `TestDuplicateBossNPC` | accepted missing/zero id and duplicate NPC; no NPC-id matching contract | 1.066s |
| 2 | required boolean `verified`; uncertain scope/stacking credit withheld; `TestSupportVerificationSeed`, `TestUnverifiedCreditRefused` | missing `verified`/credited uncertainty; accepted credit=true with verified=false | seed 1.125s; guard 1.237s |
| 3 | study-exact names and removal of unsupported expansions; `TestStudySpellNames` | missing Rockbiter; four unsupported expanded names/references retained | 1.125s |
| 4 | document enabled encounter 478 (Balnazzar); `TestEnabledRegistryDocumentation` | README does not describe seeded enabled registry | 1.125s |
| 5 | check raw token bytes for UTF-8 before replacement decoding; `TestRejectInvalidUTF8`, `TestTrailingJSON` | accepted invalid UTF-8 in values; malformed key reported only as missing field | 1.348s |
| 6 | cutoff >=5, tail<=gap, fractional/point bounds, normalized duplicate keys/ids; `TestReviewRanges`, `TestNormalizedDuplicates` | accepted cutoff 1/4, tail above gap, weights above bounds, case/space duplicates | ranges 2.308s; identities 2.563s |
| 7 | duplicate profile/encounter/candidate/assignment tests, cutoff -1/0, ratio ordering, spell-id identity; `TestReviewSurvivorCases`, `TestSpellIdentity`, `TestReviewScoringFields`, `TestLowercaseClassRefused` | accepted conflicting spell ids; existing guards lacked field context; previously surviving guards now fail their mutation tests | 2.563s; 100/100 final mutants killed |
| notes | every validation error names file/field; `TestCatalogueFieldErrors`, `TestNestedDecodeFieldErrors` | incomplete nested field paths | 3.201s |
| notes | detached Load view, deep lookup copies and `Snapshot`; `TestLookupCopies`, `TestLoadViewCopies`, `TestSnapshotCopies` | caller mutation changed lookup; Load exposed state; no snapshot accessor | 3.279s |

The pre-fix negative cases fail by assertion, not by intentionally breaking the
build. Existing rejection behaviors use mutation-red evidence: the named tests
pass on the real implementation and fail with their independent guard removed.
The map/nested-null, exact-duplicate and trailing-JSON cases intentionally test
already-supported behavior; they close coverage holes rather than claim new
production behavior.

Representative original failing output (verbatim excerpts):

```
01-red.log
        review_test.go:25: accepted invalid encounters-enabled.json; partial=true err=<nil>
        review_test.go:25: accepted invalid encounters-enabled.json; partial=true err=<nil>
    review_test.go:31: selector has no NPC-id matching contract
    review_test.go:31: selector has no NPC-id matching contract
    review_test.go:31: selector has no NPC-id matching contract
    review_test.go:42: accepted invalid encounters-enabled.json; partial=true err=<nil>
02-data-red.log
    review_data_test.go:32: study paladin-shaman.md:153,158,163 says Rockbiter
    review_data_test.go:36: unsourced full spell name retained: Greater Blessing of Wisdom
    review_data_test.go:36: unsourced full spell name retained: Greater Blessing of Kings
    review_data_test.go:36: unsourced full spell name retained: Fire Resistance Aura
    review_data_test.go:36: unsourced full spell name retained: Frost Resistance Aura
    review_data_test.go:43: README does not describe seeded enabled registry
02-guard-red.log
    review_data_test.go:18: accepted invalid support.json; partial=true err=<nil>
05-red.log
        review_decode_test.go:27: accepted invalid support.json; partial=true err=<nil>
        review_decode_test.go:27: accepted invalid support.json; partial=true err=<nil>
        review_decode_test.go:27: accepted invalid realm-labels.json; partial=true err=<nil>
        review_decode_test.go:28: not an encoding error: support.json: $.effects[0].note is required (keys are case sensitive)
06-ranges-fields-red.log
        review_ranges_test.go:12: error lacks field raid_group_size_above: instances.json: invalid raid group threshold
        review_ranges_test.go:12: error lacks field raid_group_size_above: instances.json: invalid raid group threshold
        review_ranges_test.go:26: accepted invalid scoring.json; partial=true err=<nil>
        review_ranges_test.go:26: accepted invalid scoring.json; partial=true err=<nil>
        review_ranges_test.go:26: accepted invalid scoring.json; partial=true err=<nil>
        review_ranges_test.go:26: accepted invalid scoring.json; partial=true err=<nil>
        review_ranges_test.go:26: accepted invalid scoring.json; partial=true err=<nil>
06-identities-red.log
        review_identity_test.go:40: accepted invalid specs.json; partial=true err=<nil>
        review_identity_test.go:40: accepted invalid specs.json; partial=true err=<nil>
        review_identity_test.go:40: accepted invalid specs.json; partial=true err=<nil>
        review_identity_test.go:40: accepted invalid specs.json; partial=true err=<nil>
    review_identity_test.go:47: accepted invalid support.json; partial=true err=<nil>
notes-errors-red.log
    review_errors_test.go:34: error lacks field $.profiles[0].active_time.typo: specs.json: json: unknown field "typo"
notes-copy-red.log
    review_copy_test.go:46: caller mutation changed a later lookup
    review_copy_test.go:55: Load exposed internal lookup state
    review_copy_test.go:60: no detached snapshot accessor
```

Boundary tests also accept cutoff 5, tail equal to pull gap, weight endpoints,
valid non-ASCII labels and sourced same-name/same-id references across catalogues.
Snapshot tests populate optional pointers and both scoring maps, mutate all
exported leaves, then read back. Expected values come from an independent load;
assertions are made after *each* mutation so toggling a shared bool twice cannot
mask an alias.

## Spell corrections and provenance

All references below are to `class-study/paladin-shaman.md` in the supplied study.
These are qualitative research labels, not new parser spell-id mappings.

| Before | After | Study evidence |
|---|---|---|
| Rockbiter Weapon | Rockbiter | line 153: “Rockbiter/Spirit Weapons”; line 158: “verify Rockbiter”; line 163: “tank gear, Rockbiter” |
| Greater Blessing of Wisdom | removed | line 45 says “class-wide Greater versions”; line 211 says “Greater Blessing of Might, Wisdom, Kings”, not this expanded full name |
| Greater Blessing of Kings | removed | lines 45/211, same grouped wording; no full spell-name occurrence |
| Fire Resistance Aura | removed | line 212 says “Fire/Frost/Shadow Resistance Aura”, not the expanded full name |
| Frost Resistance Aura | removed | line 212, same slash-grouped wording; no full spell-name occurrence |

Removed support references from all four Paladin profiles as well. A literal
check of every remaining profile, support and candidate spell name against all
supplied study Markdown returned no absent names. Changed data files
`specs.json` and `support.json` are content version `1.1.0` (from `1.0.0`);
their top-level and affected entry source fields explain each change. The
mutation manifest is an unversioned test array, not a versioned rules document.

Mana Spring, Mana Tide and Healing Stream use the explicit qualification at
lines 195 and 218: nearby **group** wording is provisional party scope and
cross-subgroup confirmation is outstanding. All other seed unverified/unknown
notes also withhold credit. The conservative gate includes unknown same-effect
stacking, not just scope; it does not promote high-confidence tooltip scope to
runtime evidence. Personal effects remain ineligible regardless of verification.
No scoring defaults, spell ids, encounter ids or new scope claims were added.

## Range and ownership decisions

The rules README documents the bounds and their existing semantics: role-part
and measure weights in [0,100], role-part sum 100 and nonempty measure total>0;
kill weight in (0,1], wipe weight in [0,kill), fractional shares in [0,1], and
short-fight weights in (0,1]. A kill is the existing unit contribution, not an unbounded multiplier;
inner weights are relative 100-point components, not arbitrary magnitudes.
Five is the valid dungeon-size cutoff; only greater group sizes infer a raid.
Tail may equal but not exceed pull gap. Ratio-point ratios increase strictly and
scores do not decrease. Duplicate identity normalization is lowercase/trim-space;
composite class/spec and raid/name keys normalize components separately.

A loaded Config owns a private validated snapshot; its exported fields are a
detached view. `Snapshot()` and lookups copy all reachable maps/slices/pointers.
To change scoring input, edit the rules directory and reload rather than mutate
an earlier return value. There is no cross-load cache and no JSON writes by Load.

## Reviewer hand survivors -> independent killing tests

The reviewer hand indices below are zero-based, exactly as in the two supplied
logs; cfg manifest indices are one-based. Every survivor has its own manifest
entry carrying `reviewer_hand_index`. No equivalence exemptions were claimed.

| Hand index | Guard | Manifest # | Killing test |
|---|---|---|---|
| 2 | `n.KillWeight <= 0` | 22 | `TestReviewScoringFields` |
| 3 | `n.WipeWeight < 0` | 23 | `TestReviewScoringFields` |
| 5 | `c.LowConfidenceFightMs <= 0` | 24 | `TestReviewScoringFields` |
| 6 | `c.ShortFightWeight > 1` | 25 | `TestReviewScoringFields` |
| 16 | `p.Score < s.RatioPoints[i-1].Score` | 26 | `TestReviewSurvivorCases` |
| 18 | `d.OneShotWindowMs > d.WindowMs` | 27 | `TestReviewScoringFields` |
| 19 | `d.UnhealedMinMs > d.WindowMs` | 28 | `TestReviewScoringFields` |
| 21 | `!positiveOptional(d.LostBossGapMs)` | 29 | `TestReviewScoringFields` |
| 23 | `\|\| seen[normalized(a.Job)]` | 30 | `TestReviewSurvivorCases` |
| 24 | `!oneOf(part, "output", "activity", "mechanics", "utility", "preparation")` | 31 | `TestReviewSurvivorCases` |
| 25 | `if total <= 0 {` | 32 | `TestReviewSurvivorCases` |
| 27 | `if c.Support.Overlap != "split_equally" {` | 33 | `TestReviewSurvivorCases` |
| 30 | `!expected[p.ID] \|\| seen[normalized(p.ID)] \|\|` | 34 | `TestReviewSurvivorCases` |
| 32 | `len(p.Spells) == 0` | 35 | `TestReviewSurvivorCases` |
| 34 | `if c.Instances.RaidGroupSizeAbove < 5 {` | 36 | `TestReviewRanges` |
| 40 | `e.ID <= 0 \|\| ids[e.ID]` | 37 | `TestReviewSurvivorCases` |
| 42 | `(t.Mode == "boss_only" && len(t.Units) != 0)` | 38 | `TestReviewSurvivorCases` |
| 46 | `if candidates[key] {` | 39 | `TestReviewSurvivorCases` |
| 47 | `class != strings.ToUpper(class)` | 40 | `TestLowercaseClassRefused` |
| 48 | `if _, err := d.Token(); err != io.EOF {` | 41 | `TestTrailingJSON` |
| 53 | `case reflect.Map:` | 42 | `TestReviewSurvivorCases` |

Verified programmatically: all 21 survivors are represented once,
all 100 manifest indices appear as KILLED in the final output. The manifest
has 79 new mutants and 8 existing mutants re-anchored:
`1, 3, 5, 7, 8, 9, 20, 21`. Re-anchoring follows field-specific guard
refactors, normalized duplicate detection and relocated unknown-field checks;
none were dropped. In particular old #5 now challenges normalized JSON-key
comparison, because the old explicit reflection case-alias branch was subsumed
by exact known-field checking plus token-level normalized duplicate checks.

New challenges cover NPC matching/identity, credit verification, both UTF-8
locations, cutoff and tail boundaries, weight ceilings, normalization, global
spell identity, diagnostics, the private snapshot and each deep-copy boundary.

## Iteration issues actually encountered

- The first attempted UTF-8 green run had a malformed realm-label *test anchor*;
  quoting the JSON string fixed the fixture, then the full suite passed.
- The first attempted identity green run failed profile coverage because one
  lookup still used an unnormalized key. Both insertion and checking now agree.
  A later field-specific test exposed component whitespace in spec ids; normalize
  class/spec components separately rather than only the concatenated string.
- The first 100-mutant run killed 98: JSON sorting masked lowercase-first alias
  order in #5; toggling an aliased `Stacks` bool twice masked copy mutant #93.
  Added an authored reverse-order JSON fixture and an assertion immediately after
  the first snapshot mutation. The repeated full run killed 100/100.
- The initial Go-module gate timed out cfg at 35s against its unchanged 30s
  manifest budget; a direct race run passed in 39.681s but was still too slow.
  Removed redundant per-scalar decoding (the final typed JSON decode already
  checks scalar types), and made the error helper reuse one Load result rather
  than load every negative fixture twice. The unchanged module budget then
  passed, with the final cfg race-module time below.

## Final sequential verification

All commands below completed with exit 0, after the last production/test change.
`gofmt -l` produced no filenames; `go vet` produced no diagnostics. No full
`desktop go test ./...` was run. All heavy commands were sequential.

Using the task's Go PATH and resolved uv executable (`$UV`); host paths omitted:

```sh
PY="$UV run --no-project --python 3.14 --with-requirements .github/requirements-ci.txt python"
(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)
python3 -I -B tools/raidtest.py --all --kind go
$PY -I -B tools/raidtest.py --mutants cfg
$PY -I -B tools/raidtest.py --mutants rules
$PY -I -B tools/raidtest.py --all --kind lua,py
```

Observed output (cfg mutation details above; final result retained here):

```
ok  	github.com/thegamersvibe/relax-coach/desktop/internal/raid	0.009s
ok  	github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg	2.679s
ok  	github.com/thegamersvibe/relax-coach/desktop/internal/raid/model	0.097s
?   	github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest	[no test files]
ok   cfg                    24.7s
ok   model                   1.5s
ok   rules                   0.3s
RESULT: 3/3 modules passed
RESULT: 100/100 mutants killed
RESULT: no mutants
ok   _client                 0.1s
ok   _runner                17.7s
ok   core                    0.1s
ok   log                     0.2s
RESULT: 4/4 modules passed
```

`--mutants rules` reports **no mutants**; that is not an invented mutation count.
The rules Go module separately passed its real source/literal/import gates.
Open implementation questions: none. Runtime support scope/attribution,
uncalibrated profile viability and currently unset phase-2 weights remain
explicitly unverified/unconfigured data; this patch does not resolve those with
assumptions. Native Windows/game testing is outside this lane.

# R5 review round 2 — follow-up evidence

Base: `981f08e62eda2238562adae12bae404adb3252fb`, branch
`fix/raid-cfg-round2`. At clone time main had moved beyond the requested
`c2d078e` (PR #50) to PR #52's merge; `c2d078e` is an ancestor of this base.
This is one local follow-up commit, with no push, PR or tag. Read the round-2
findings, `mut4.py`, `mut5.py`, `p2-mut-{a,b,c,d,e}.log`, `zz6_test.go`, and
both supplied second opinions/probe descriptions read-only. No model, runner,
CI, module timeout, release-version section or JSON seed data was changed.
Consequently no data content-version bump is needed.

## Test-first refusals and diagnostics

Commands below used `go -C desktop test -count=1 -v -run '<names>'
./internal/raid/cfg`. All listed red runs exited 1 because assertions failed;
all green runs exited 0. Local raw logs are retained in `.git/r5b-evidence/`.
The committed tests and mutant manifest reproduce the behavioral evidence.

| Item | Red observed before implementation | Green after implementation |
|---|---|---|
| Credited stacking | `TestRound2CreditRequiresStacks/null`: `accepted invalid support.json; partial=true err=<nil>` | Same group passed, 0.058s. `credit=true` requires non-null `stacks`; explicit false and true both work. Missing stacks already failed the shape check and remains tested; uncredited null stays valid. |
| Escaped surrogate text | `TestRound2SurrogateStrings`: lone high, lone low, reversed pair, high/high, separated pair and uppercase boundary escapes loaded or lacked surrogate diagnosis, in values and object keys | Group plus `TestRound2ValidStringEscapes` passed, 0.179s. Raw-token checks reject unpaired UTF-16 before replacement can hide it, naming the file/path. Later cases cover low/low and high/non-surrogate too. Valid minimum/maximum/mixed-case pairs, escaped backslashes/quotes, explicit U+FFFD and legal escaped NUL round-trip unchanged in both keys and values. |
| NUL key path | `TestRound2NULKeyPath`: all four cases named only the enclosing object, e.g. `scoring.json: $: invalid character '\x00' in string` | Same group passed, 0.016s. Diagnostic-only key recovery names `$["sou\x00rce"]`, nested fields, array-object fields and a key containing an authored escaped quote. It still refuses the original bytes; it does not repair input. |
| Smaller validation sample | `TestRound2ValidationSample`: `validation fixture still repeats the full catalogues: profiles=33 classes=9 effects=65 candidates=22` | Same test passed, 0.024s, then all cfg tests and the complete existing mutant pack passed. |

The sample retains three Warrior profiles with their class ids and four support
entries, plus two candidate encounters needed for duplicate tests. It compares
retained records and every unrelated catalogue against a complete independent
Load. Only immutable fixture bytes are shared via `sync.OnceValues`; each case
writes a fresh directory and calls the real loader. No decoded Config or load
result is cached. Full-seed, reload, lookup and ownership tests keep full data.
All validation assertions remain, including malformed JSON and field paths.

## Review survivors and missing guard coverage

These behaviors already had production guards. Their red evidence is mutation:
add each targeted test, remove only its guard, observe the test fail, then run
against the unchanged guard and observe green. The selected mutation-red replay
killed every new entry; the final full pack independently killed all 116 entries.
The full cfg green run following these additions passed in 2.665s.

| Reviewer survivor | Manifest # | Failing test with that guard removed |
|---|---|---|
| `mut4.py #1`, output part range | 101 | `TestRound2PartRanges` |
| `mut4.py #2`, preparation part range | 102 | `TestRound2PartRanges` |
| `mut4.py #19`, rounding step ceiling | 103 | `TestRound2RoundingStep` |
| `mut4.py #38`, personal scope credit | 104 | `TestRound2SupportScope` |
| Requested unknown scope credit coverage | 105 | `TestRound2SupportScope` |
| `mut4.py #42`, duplicate profile support | 106 | `TestRound2DuplicateProfileSupport` |
| `mut4.py #56`, target-rule unit selectors | 107 | `TestRound2TargetSelectors` |
| `mut5.py #5`, candidate ability spell identity; second opinion M10 | 108 | `TestRound2EncounterSpellIdentities` |
| `mut5.py #6`, enabled mechanic spell identity; second opinion M11 | 109 | `TestRound2EncounterSpellIdentities` |

The findings call #38 "unknown scope", but the actual mutation removes
**personal** from the guard. Both branches now have independent mutants and
fixtures with verified=true and known stacks so other credit guards cannot mask
them. Output/preparation cases cover all three roles at -1, 0, 100 and 101; the
negative case keeps the total exactly 100 and every other part in range.
`rounding_step` already had the `(0,100]` guard: its unit is score points and
100 is the entire score scale, so a larger quantum exceeds that scale rather
than expressing score precision. Tests cover -1, 0, .01, 100 and 100.01; null
remains unconfigured. The rules README now states this existing bound.

Selector tests cover absent/zero/negative ids, duplicate ids, missing source
and a valid id-only selector. Identity tests assign a sourced id to a profile
spell, then use it in each encounter catalogue under a different name; refusal
must name both paths. Normalized same-name/same-id references remain accepted.
Duplicate support fixtures use two identical resolving references, so a dangling
reference check cannot mask the uniqueness guard.

New-guard mutants are #110 (credited null stacks), #111/#112 (surrogate value/key
checks), #113-#115 (surrogate ordering/adjacency/second-half checks), and #116
(NUL key diagnostic path). All 100 original manifest entries are unchanged:
**16 new, 0 re-anchored, none dropped**. The first local replay exposed masking
of old #8 and #48 by the new stacks guard. Setting otherwise-valid stacks (and
verification where testing scope) in those fixtures restored independent kills;
no assertion or mutant was weakened. Final #8 and #48 both report KILLED.

### Equivalent survivor: mut4.py #64

The reviewer already marked this one equivalent: replacing `b = value // Shape`
with `_ = value // Shape` leaves original bytes, rather than the first
`json.RawMessage`, for shape/typed decoding. Before that assignment, `uniqueKeys`
and its EOF check have required exactly one valid JSON value. Thus the only
remaining difference is legal surrounding JSON whitespace, accepted by both
subsequent decoders. Invalid suffixes still fail before this assignment.

Evidence: ran the full cfg suite normally and with exactly that source change
through a Go `-overlay` file. Both exited 0, each with 416 passing test/subtest
records and identical complete pass/fail/skip inventories (control 3.459s,
mutant 1.491s). This includes existing `TestTrailingJSON` invalid suffix cases
and new `TestRound2TrailingWhitespace` acceptance cases. The overlay, JSON event
logs and inventory check are retained locally. No fictitious killing test or
unkillable entry was added to the pack for this equivalent assignment.

## Race budget: three runs before and after

All six sequential runs used exactly:

```sh
python3 -I -B tools/raidtest.py --all --kind go
```

Every run exited 0 with `RESULT: 5/5 modules passed`; cfg is run with `-race`
by its unchanged module manifest. Times below are the runner's cfg wall time
(to its displayed 0.1s precision) and separately measured whole-command wall
seconds. No timeout was raised and no assertion or race detector was removed.

| Run | Before cfg | After cfg | Before all-Go wall | After all-Go wall |
|---|---:|---:|---:|---:|
| 1 | 27.0s | 10.5s | 50.405s | 31.015s |
| 2 | 25.5s | 9.4s | 50.024s | 32.177s |
| 3 | 27.6s | 10.8s | 48.383s | 32.613s |

Against the unchanged 30s budget, the slowest measured cfg run went from
**1.09x margin / 2.4s spare** to **2.78x margin / 19.2s spare** (budget divided
by slowest time, not a claim about arbitrary host contention). Profiling showed
repeated full-catalogue validation in the slow table tests; the smaller fixture
removes that work rather than relaxing their checks. These are local measured
runs, not a claim of cold-cache or Windows timing coverage.

## Final sequential acceptance and iteration issues

After the last production/test edit:

```text
(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)
  gofmt: no filenames; go vet: no diagnostics
  raid             0.020s
  raid/cfg         1.908s
  raid/fixture     0.011s
  raid/logfmt      5.294s
  raid/model       0.218s
  raid/model/modeltest [no test files]
python3 -I -B tools/raidtest.py --all --kind go
  5/5 modules passed on each of three runs (times above)
$PY -I -B tools/raidtest.py --mutants cfg
  RESULT: 116/116 mutants killed
$PY -I -B tools/raidtest.py --mutants rules
  RESULT: no mutants
$PY -I -B tools/raidtest.py --all --kind lua,py
  _client 0.1s; _runner 22.4s; core 0.2s; log 0.4s;
  prep 0.2s; store 16.3s; wire 0.7s
  RESULT: 7/7 modules passed
```

`$PY` is the resolved uv executable running `--no-project --python 3.14
--with-requirements .github/requirements-ci.txt python`. All acceptance commands
above exited 0; heavy commands ran sequentially. No full desktop `go test ./...`
was run. A first Go acceptance attempt correctly rejected `1<<10` as a folded
1024 constant under the no-large-literals gate. Reading the needed bit as
`code>>10&1` / `low>>10&1` preserves the surrogate test without a large literal;
the final rules gate and all mutants passed. No exemption was added.

Open implementation questions: none. Native Windows/game execution, arbitrary
host-load bounds and concurrent multi-file editing are not established here;
the existing non-transactional load contract and phase-2 unknowns are unchanged.

## R6/R7 raid-1 data — scoring 1.2.0

Base: `6208b163a0a5491a641809d7cd3c764905276c36` (Merge PR #51), exactly
`origin/main` at branch creation. Branch: `feat/raid-scoring-data`. One local
commit; no push, PR, tags, model edits or root README release/version edits.
The authoritative PHASE1-BRIEF section **R6 and R7 on main 6208b16** is read
with the newer **Raid 1 gets a score** ruling, which wins where they differ.

### Data and provenance

Only `scoring.json` changes content version, from 1.1.0 to 1.2.0. Schema remains
1. Its top-level `source` records the architect ruling and distinguishes reported
log measurements from starting guesses; this lane did not inspect private logs
or claim fresh measurements. Role-part weights and phase-2 toggles are unchanged.

| Field | Value | Source text / status |
|---|---|---|
| `rounding_step` | 0.1 | PLAN 4A row 7 |
| `gcd_ms` | 1500 | Architect reports second-log cast-gap peak at 1.4–1.5 s |
| `gcd_energy_ms` (new) | 1000 | Same count's 0.9–1.0 s peak; used for cast PowerType 3 |
| `healer.low_health_fraction` | 0.5 | Guess, starting value |
| `death.lost_boss_gap_ms` | 4000 | Guess, starting value |
| `death.summary_window_ms` | 10000 | Same as the death window |
| `confidence.provisional_min_coverage` (new) | 0.25 | Newer architect ruling, starting value; normal minimum stays 0.6 |
| `count_penalties` | `owned_death: 40`, `boss_swing_taken: 2`, `better_item_in_bags: 25` | First two are guesses; bag-gear penalty is the newer ruling |
| `measure_weights.output` | `boss_damage: 100` | Starting guess; newer ruling also applies it to tanks against their own role's median |
| `measure_weights.activity` | `active_time: 100` | Starting guess |
| `measure_weights.mechanics` | `deaths: 50`, `avoidable: 30`, `boss_swings: 20` | Starting guesses |
| `measure_weights.preparation` | `stock: 30`, `bag_gear: 30`, `used: 40` | Newer architect ruling, starting values; used remains unmeasured |
| `new_at_60_seconds` | null (unchanged) | Prep-report label, not a score |

No utility entry or separate tank-output weight is invented. Data and consumer
semantics are documented in `rules/raid/README.md`; no scorer or prep reader is
implemented in this cfg/data lane.

### Cfg contract and test-first evidence

Added `Scoring.GCDEnergyMs *int64` and
`Confidence.ProvisionalMinCoverage float64`. Existing reflection-based strict
decoding recognizes their JSON tags; no alternate decoder or default is added.
The energy key is required, like `gcd_ms`, but explicit null is allowed to mean
unconfigured. A configured value must be positive int64 milliseconds; no
unsourced absolute gameplay ceiling is introduced. Tests cover 0/1, negative
values, the maximum int64 and overflow, plus wrong scalar types. The provisional
key is required and non-null, with `0 < provisional < min_coverage` against the
configured minimum. Tests cover both sides and equality at each boundary,
adjacent floats, and changed normal thresholds to expose a hardcoded 0.6.
Every rejection asserts file/field diagnostics and no partial config or rewrite.

`Snapshot`/Load-view copying now detaches the energy pointer. The provisional
scalar already copies by value. Tests independently mutate both returned views.
Exact seed values, map membership and provenance text are asserted after a real
full-directory load. The changed existing scalar, penalty and every named
measure weight have lower/upper valid and invalid tests. Weight tests hold the
part's total positive so that the component guard, not the total guard, is tested.

Each behavior below had an assertion failure before its implementation/data
change, then the indicated passing run. Commands were
`go -C desktop test -count=1 -v -run '^<test>$' ./internal/raid/cfg`.

| Behavior / test | Observed red (exit 1) | First green (exit 0) |
|---|---|---|
| `TestEnergyGCDDecode` | `$.gcd_energy_ms: unknown field`; missing key accepted | 0.033s |
| `TestEnergyGCDRange` | -1 and 0 accepted | 0.080s |
| `TestEnergyGCDSnapshotCopies` | Load view and Snapshot aliased the energy pointer | 0.015s |
| `TestProvisionalCoverageDecode` | New key unknown; missing key accepted; no null diagnostic | 0.029s |
| `TestProvisionalCoverageRange` | Negative/zero, equality, above-minimum and lowered-minimum cases accepted | 0.199s |
| `TestRaidOneScoringSeed` | Version 1.1.0, null values, empty maps and absent provenance | 0.017s |

Existing guards were not rewritten for a data change. Their new boundary tests
use mutation-red evidence: cfg mutants 112–120 remove individual duration,
rounding, health, weight or penalty checks and are killed by
`TestRaidOneScalarRanges`, `TestRaidOneMeasureWeightRanges` and
`TestRaidOneCountPenaltyRanges`. A clean post-mutant run of
`^Test(EnergyGCD|ProvisionalCoverage|RaidOne)` passed in **0.325s**.

The normalized-duplicate test initially failed with `missing duplicate-key
fixture anchor` because its seed assumption was an empty penalty map. It now
constructs that empty map explicitly, preserving the authored lowercase-first
then uppercase duplicate and every original assertion. The full suite and its
existing normalization mutants pass.

### Race budget and mutation inventory

The unchanged base cfg module took **29.1s**. After the new fields/tests, the
first cfg module run failed at **32.0s** against the unchanged 30s budget
(test process 30.467s). A race CPU profile took 33.609s and sampled 31.92s of CPU
over 32.52s: race-instrumented JSON decoding, not idle waiting, dominated.

Only repeated scoring/schema validation fixtures were reduced: one unchanged
profile and spec id plus all its referenced support effects, with other files
unchanged. Immutable serialized fixture bytes are reused, never loaded Config
objects; every case still gets fresh files and exercises real `Load`.
`TestScoringFixture` compares the retained rows and all unrelated data against
an independent full load, then proves one fixture edit cannot contaminate the
next. Full-seed, reload, catalogue and ownership cases retain full data.
The final cfg race module took **21.6s**, with no runner/module budget edits.

All **100** prior mutant entries are unchanged. **20** additions cover the
energy range/required-key/copy boundary, both provisional bounds and strictness,
configured-threshold comparison, required/non-null decoding and the changed
existing values' range guards. Parsed the actual output against the manifest:
**120 distinct KILLED indices, 120/120 total; no survivors or refusals**.

### Final sequential verification

All commands below completed with exit 0 after the final code/test edit. The
first formatting check named `strict_test.go`; it was formatted and the entire
Go command/module sequence rerun. The final `gofmt -l` emitted no filenames;
`go vet` emitted no diagnostics. No full desktop `go test ./...`, native game
or Windows test was run. The Go rules module checked source literals/imports.

```sh
PY="$UV run --no-project --python 3.14 --with-requirements .github/requirements-ci.txt python"
(cd desktop && gofmt -l internal/raid && go vet ./internal/raid/... && go test -count=1 ./internal/raid/...)
python3 -I -B tools/raidtest.py --all --kind go
$PY -I -B tools/raidtest.py --mutants cfg
$PY -I -B tools/raidtest.py --mutants rules
$PY -I -B tools/raidtest.py --all --kind lua,py
```

Actual final output:

```text
ok  	github.com/thegamersvibe/relax-coach/desktop/internal/raid	0.022s
ok  	github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg	2.578s
ok  	github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights	0.242s
ok  	github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture	0.004s
ok  	github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt	4.882s
ok  	github.com/thegamersvibe/relax-coach/desktop/internal/raid/model	0.098s
?   	github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest	[no test files]
ok   cfg                    21.6s
ok   fights                  3.2s
ok   fixture                 1.5s
ok   logfmt                 16.4s
ok   model                   1.5s
ok   raidlogs                0.7s
ok   rules                   0.3s
RESULT: 7/7 modules passed
RESULT: 120/120 mutants killed
RESULT: no mutants
ok   _client                 0.2s
ok   _runner                20.2s
ok   core                    0.2s
ok   log                     0.3s
ok   prep                    0.2s
ok   store                  15.6s
ok   wire                    0.6s
RESULT: 7/7 modules passed
```

Open implementation questions: none. Values remain explicitly uncalibrated;
R6/R7 application of these fields and prep-reader integration are separate work.
