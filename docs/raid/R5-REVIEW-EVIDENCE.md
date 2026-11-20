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
