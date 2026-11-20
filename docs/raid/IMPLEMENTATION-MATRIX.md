# Relax Raid implementation matrix

- Requirements: `REQUIREMENTS.md`
- Integration owner: the builder of R0
- Updated: 2026-11-19
- Evidence directory: `docs/raid/evidence/`

## Implementation coverage

| Row | REQ ID | Target | PR / slice | Depends on | Owner | Implementation files | Status | Validation |
|---|---|---|---|---|---|---|---|---|
| IMP-001 | REQ-LOG-01 | forever | R1 | R0 | builder | `addon/RelaxRaid/{RelaxRaid.toc,Core.lua,Log.lua}` | built, in review | V-001 |
| IMP-002 | REQ-LOG-02 | forever | R1 | R0 | builder | `addon/RelaxRaid/{RelaxRaid.toc,Core.lua,Log.lua}` | built, in review | V-002 |
| IMP-003 | REQ-LOG-03 | forever | R4 | R0 + model addendum 2 | R4 builder | `desktop/raid_logs.go` | implemented | V-003 |
| IMP-004 | REQ-ADD-01 | forever | R2 | R1 | R2 builder | `addon/RelaxRaid/{Prep.lua,Wire.lua,Store.lua}` | implemented; simulator pass | V-004 |
| IMP-005 | REQ-ADD-02 | forever | R2 | R1 | R2 builder | `addon/RelaxRaid/{Prep.lua,Wire.lua,Store.lua}` | implemented; simulator pass | V-005 |
| IMP-006 | REQ-ADD-03 | forever | R2 | R1 | R2 builder | `addon/RelaxRaid/{Prep.lua,Wire.lua,Store.lua}` | implemented; simulator pass | V-006 |
| IMP-007 | REQ-ADD-04 | forever | R2 | R1 | R2 builder | `addon/RelaxRaid/{Prep.lua,Wire.lua,Store.lua}` | implemented; simulator pass | V-007 |
| IMP-008 | REQ-ADD-05 | forever | R2 | R1 | R2 builder | `addon/RelaxRaid/{Prep.lua,Wire.lua,Store.lua}` | implemented; app label acceptance pending R7 | V-008 |
| IMP-009 | REQ-PAR-01 | forever | R3 | R0 | unassigned | `desktop/internal/raid/{logfmt,fixture}` | planned | V-009 |
| IMP-010 | REQ-PAR-02 | forever | R3 | R0 | unassigned | `desktop/internal/raid/{logfmt,fixture}` | planned | V-010 |
| IMP-011 | REQ-PAR-03 | forever | R4 | R0 + model addendum 2 | R4 builder | `desktop/internal/raid/fights` | implemented | V-011 |
| IMP-012 | REQ-PAR-04 | forever | R4 | R0 + model addendum 2 | R4 builder | `desktop/internal/raid/fights` | implemented; real-log golden open | V-012 |
| IMP-013 | REQ-PAR-05 | forever | R4 | R0 + model addendum 2 | R4 builder | `desktop/internal/raid/fights` | implemented | V-013 |
| IMP-014 | REQ-PAR-06 | forever | R4 | R0 + model addendum 2 | R4 builder | `desktop/internal/raid/fights` | in progress; cast hook only, class open | V-014 |
| IMP-015 | REQ-PAR-07 | forever | R10 | R7 | unassigned | `desktop/internal/raid/result` | planned | V-015 |
| IMP-016 | REQ-PAR-09 | forever | R15b, R15c; W8, W9 | R4, W8 | unassigned | `desktop/internal/raid/rawstore`, app upload/download glue; site access and retention | planned (phase 2); R3 uploads nothing | V-016 |
| IMP-017 | REQ-SCO-01 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-017 |
| IMP-018 | REQ-SCO-02 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-018 |
| IMP-019 | REQ-SCO-03 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-019 |
| IMP-020 | REQ-SCO-04 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-020 |
| IMP-021 | REQ-SCO-05 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-021 |
| IMP-022 | REQ-SCO-06 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-022 |
| IMP-023 | REQ-SCO-07 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-023 |
| IMP-024 | REQ-SCO-08 | forever | R12 | R8 | unassigned | `desktop/internal/raid/score`, officer page | planned | V-024 |
| IMP-025 | REQ-SCO-09 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-025 |
| IMP-026 | REQ-SCO-10 | forever | R5 | R0 | unassigned | `desktop/internal/raid/cfg`, `rules/raid/` | planned | V-026 |
| IMP-027 | REQ-SCO-11 | forever | R12 | R8 | unassigned | `desktop/internal/raid/score`, officer page | planned | V-027 |
| IMP-028 | REQ-SCO-12 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-028 |
| IMP-029 | REQ-SCO-12 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-029 |
| IMP-030 | REQ-SCO-13 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-030 |
| IMP-031 | REQ-SCO-14 | forever | R7 | R0 | unassigned | `desktop/internal/raid/score` | planned | V-031 |
| IMP-032 | REQ-DTH-01 | forever | R6 | model addendum 2, R5 | R6 builder | `desktop/internal/raid/death` | verified (engine; no UI) | V-032 |
| IMP-033 | REQ-DTH-02 | forever | R6 | model addendum 2, R5 | R6 builder | `desktop/internal/raid/death` | verified (engine; no UI) | V-033 |
| IMP-034 | REQ-DTH-03 | forever | R12 | R8 | unassigned | `desktop/internal/raid/score`, officer page | planned | V-034 |
| IMP-035 | REQ-DTH-03 | forever | R6 | model addendum 2, R5 | R6 builder | `desktop/internal/raid/death` | verified (engine; no UI) | V-035 |
| IMP-036 | REQ-DTH-04 | forever | R6 | model addendum 2, R5 | R6 builder | `desktop/internal/raid/death` | verified (engine; no UI) | V-036 |
| IMP-037 | REQ-DTH-05 | forever | R12 | R8 | unassigned | `desktop/internal/raid/score`, officer page | planned | V-037 |
| IMP-038 | REQ-DTH-05 | forever | R6 | model addendum 2, R5 | R6 builder | `desktop/internal/raid/death` | verified (engine; no UI) | V-038 |
| IMP-039 | REQ-DTH-06 | forever | R6 | model addendum 2, R5 | R6 builder | `desktop/internal/raid/death` | verified (engine; no UI) | V-039 |
| IMP-040 | REQ-SPC-01 | forever | R5 | R0 | unassigned | `desktop/internal/raid/cfg`, `rules/raid/` | planned | V-040 |
| IMP-041 | REQ-SPC-02 | forever | R5 | R0 | unassigned | `desktop/internal/raid/cfg`, `rules/raid/` | planned | V-041 |
| IMP-042 | REQ-SPC-03 | forever | R5 | R0 | unassigned | `desktop/internal/raid/cfg`, `rules/raid/` | planned | V-042 |
| IMP-043 | REQ-SUP-01 | forever | R13 | R7 | unassigned | `desktop/internal/raid/score`, `rules/raid/` | planned | V-043 |
| IMP-044 | REQ-SUP-02 | forever | R13 | R7 | unassigned | `desktop/internal/raid/score`, `rules/raid/` | planned | V-044 |
| IMP-045 | REQ-SUP-03 | forever | R13 | R7 | unassigned | `desktop/internal/raid/score`, `rules/raid/` | planned | V-045 |
| IMP-046 | REQ-ENC-01 | forever | R5 | R0 | unassigned | `desktop/internal/raid/cfg`, `rules/raid/` | planned | V-046 |
| IMP-047 | REQ-ENC-02 | forever | R5 | R0 | unassigned | `desktop/internal/raid/cfg`, `rules/raid/` | planned | V-047 |
| IMP-048 | REQ-ENC-03 | forever | R5 | R0 | unassigned | `desktop/internal/raid/cfg`, `rules/raid/` | planned | V-048 |
| IMP-049 | REQ-OFF-01 | forever | R8 | R4, R6, R7 | unassigned | `desktop/raid_page.go`, `desktop/raid.html` | planned | V-049 |
| IMP-050 | REQ-OFF-02 | forever | R12 | R8 | unassigned | `desktop/internal/raid/score`, officer page | planned | V-050 |
| IMP-051 | REQ-OFF-03 | forever | R14 | R10 | unassigned | officer page | planned | V-051 |
| IMP-052 | REQ-OFF-04 | forever | R14 | R10 | unassigned | officer page | planned | V-052 |
| IMP-053 | REQ-RDR-01 | forever | R14 | R10 | unassigned | officer page | planned | V-053 |
| IMP-054 | REQ-RDR-02 | forever | R14 | R10 | unassigned | officer page | planned | V-054 |
| IMP-055 | REQ-OFF-05 | forever | R8 | R4, R6, R7 | unassigned | `desktop/raid_page.go`, `desktop/raid.html` | planned | V-055 |
| IMP-056 | REQ-WEB-01 | forever | R10 | R7 | unassigned | `desktop/internal/raid/result` | planned | V-056 |
| IMP-057 | REQ-WEB-02 | forever | R11 | R8 | unassigned | glue in `desktop/` | planned | V-057 |
| IMP-058 | REQ-WEB-02 | forever | R15 | R10, Q-04 | unassigned | `desktop/internal/raid/upload` | planned | V-058 |
| IMP-059 | REQ-WEB-03 | forever | R15 | R10, Q-04 | unassigned | `desktop/internal/raid/upload` | planned | V-059 |
| IMP-060 | REQ-WEB-05 | forever | R15 | R10, Q-04 | unassigned | `desktop/internal/raid/upload` | planned | V-060 |
| IMP-061 | REQ-WEB-06 | forever | R15 | R10, Q-04 | unassigned | `desktop/internal/raid/upload` | planned | V-061 |
| IMP-062 | REQ-WEB-04 | forever | R8 | R4, R6, R7 | unassigned | `desktop/raid_page.go`, `desktop/raid.html` | planned | V-062 |
| IMP-063 | REQ-SR-01 | forever | R16 | R2 | unassigned | `addon/RelaxRaid/Reserve.lua` | planned | V-063 |
| IMP-064 | REQ-SR-02 | forever | R17 | R16 | unassigned | `addon/RelaxRaid/{ReserveUI.lua,Loot.lua}` | planned | V-064 |
| IMP-065 | REQ-SR-03 | forever | R16 | R2 | unassigned | `addon/RelaxRaid/Reserve.lua` | planned | V-065 |
| IMP-066 | REQ-SR-04 | forever | R16 | R2 | unassigned | `addon/RelaxRaid/Reserve.lua` | planned | V-066 |
| IMP-067 | REQ-SR-05 | forever | R18 | R16 | unassigned | `addon/RelaxRaid/Reserve.lua`, `Store.lua` | planned | V-067 |
| IMP-068 | REQ-SR-06 | forever | R17 | R16 | unassigned | `addon/RelaxRaid/{ReserveUI.lua,Loot.lua}` | planned | V-068 |
| IMP-069 | REQ-SR-07 | forever | R16 | R2 | unassigned | `addon/RelaxRaid/Reserve.lua` | planned | V-069 |
| IMP-070 | REQ-SR-10 | forever | R16 | R2 | unassigned | `addon/RelaxRaid/Reserve.lua` | planned | V-070 |
| IMP-071 | REQ-SR-08 | forever | R19 | R0 | unassigned | `desktop/internal/raid/sr` | planned | V-071 |
| IMP-072 | REQ-SR-09 | forever | later | Q-04 | unassigned | n/a | deferred | V-072 |
| IMP-073 | REQ-DST-01 | forever | R9a | none | builder | `desktop/addon_update.go`, `desktop/addon_update_guild.go`, `desktop/addon_bundle_state.go`, `desktop/addon_update_lock_recovery.go`, `desktop/addon_update_transaction.go` | built, in review | V-073 |
| IMP-074 | REQ-DST-01 | forever | R9b | R1, R9a released | unassigned | `scripts/build_guild_bundle.py` | planned | V-074 |
| IMP-075 | REQ-DST-02 | forever | every PR | none | unassigned | n/a | planned | V-075 |
| IMP-076 | REQ-DST-03 | forever | R0 | none | R0 builder (bunyan) | `desktop/internal/raid/model`, `tools/raidtest.py`, `test/raid/_client/` | implemented | V-076 |
| IMP-077 | REQ-DST-03 | forever | every PR | none | unassigned | n/a | planned | V-077 |
| IMP-078 | REQ-DGN-01 | forever | D1 | R0 + model addendum 2 | R4 builder | `desktop/internal/raid/fights` | implemented | V-078 |
| IMP-079 | REQ-DGN-02 | forever | D1 | R0 + model addendum 2 | R4 builder | `desktop/internal/raid/fights` | in progress; scorer/history integration open | V-079 |
| IMP-080 | REQ-DGN-03 | forever | D2 | R8 | unassigned | `desktop/internal/raid/result` | planned | V-080 |
| IMP-081 | REQ-DGN-04 | forever | D2 | R8 | unassigned | `desktop/` page handlers | planned | V-081 |
| IMP-082 | REQ-DGN-05 | forever | D3 | W4 | unassigned | n/a (site repository) | planned | V-082 |

Statuses: `planned`, `in progress`, `implemented`, `verified`, `blocked`, `deferred`, `superseded`. `implemented` does not mean acceptance passed.

## Validation evidence

| ID | REQ / slice | Target and build | Mode | Revision | Procedure and expected result | Observed result | Status |
|---|---|---|---|---|---|---|---|
| V-001 | REQ-LOG-01 / R1 | forever; mock client | simulator | `feat/raid-logger` | `raidtest.py log core`: raid and dungeon entry, start ignored 10 s with the read 5 s and 8 s behind, never off on every path, refused setting write, APIs that throw | pass; native not run (alpha, owner's decision) | simulator pass |
| V-002 | REQ-LOG-02 / R1 | forever; mock client | simulator | `feat/raid-logger` | `raidtest.py log`: officer and member rank, rank secret or not a number, guild data late or never, no guild, choice saved and winning, panel checkbox, newer save untouched | pass; native not run (alpha, owner's decision) | simulator pass |
| V-003 | REQ-LOG-03 / R4 | forever; model values | simulator | `feat/raid-fights` | Focused tests and mutations in R4-EVIDENCE.md | Synthetic tests pass; see R4-EVIDENCE.md; private/native checks not run | synthetic pass, integration open |
| V-004 | REQ-ADD-01 / R2 | forever; pinned build 70790; mock client | simulator | `feat/raid-prep` | Flask/ring collection; unavailable and secret reads; [R2 evidence](evidence/R2.md) | pass for addon scope; native not run (alpha, owner's decision) | simulator pass |
| V-005 | REQ-ADD-02 / R2 | forever; pinned build 70790; mock client | simulator | `feat/raid-prep` | Trigger, restriction-at-dispatch, cooldown, fragment, expiry and logout tests; [R2 evidence](evidence/R2.md) | pass for addon scope; native not run (alpha, owner's decision) | simulator pass |
| V-006 | REQ-ADD-03 / R2 | forever; pinned build 70790; mock client | simulator | `feat/raid-prep` | Sender forgery, received time, retained bounds, full-store reload and newer-save identity; [R2 evidence](evidence/R2.md) | pass for addon scope; native not run (alpha, owner's decision) | simulator pass |
| V-007 | REQ-ADD-04 / R2 | forever; pinned build 70790; mock client | simulator | `feat/raid-prep` | No optional providers gives not checked; late Coach audit queried at use time; [R2 evidence](evidence/R2.md) | pass for addon scope; native not run (alpha, owner's decision) | simulator pass |
| V-008 | REQ-ADD-05 / R2 | forever; pinned build 70790; mock client | simulator | `feat/raid-prep` | Played-at-60 is sent as data and never points; configurable label/scoring acceptance belongs to R7; [R2 evidence](evidence/R2.md) | pass for addon scope; native not run (alpha, owner's decision) | addon data pass; scoring acceptance pending |
| V-009 | REQ-PAR-01 / R3 | forever; build unknown | unit | none yet | Acceptance column of REQ-PAR-01 | not run | not run |
| V-010 | REQ-PAR-02 / R3 | forever; build unknown | unit | none yet | Acceptance column of REQ-PAR-02 | not run | not run |
| V-011 | REQ-PAR-03 / R4 | forever; model values | unit | `feat/raid-fights` | Focused tests and mutations in R4-EVIDENCE.md | Synthetic tests pass; see R4-EVIDENCE.md; private/native checks not run | synthetic pass, integration open |
| V-012 | REQ-PAR-04 / R4 | forever; model values | unit | `feat/raid-fights` | Focused tests and mutations in R4-EVIDENCE.md | Synthetic tests pass; see R4-EVIDENCE.md; private/native checks not run; power, max power and type now sampled with presence preserved | synthetic pass, integration open |
| V-013 | REQ-PAR-05 / R4 | forever; model values | unit | `feat/raid-fights` | Focused tests and mutations in R4-EVIDENCE.md | Synthetic tests pass; see R4-EVIDENCE.md; private/native checks not run | synthetic pass, integration open |
| V-014 | REQ-PAR-06 / R4 | forever; model values | unit | `feat/raid-fights` | Focused tests and mutations in R4-EVIDENCE.md | Synthetic tests pass; see R4-EVIDENCE.md; private/native checks not run; class field absent from shared model; empty cast signatures remain unknown | synthetic pass, integration open |
| V-015 | REQ-PAR-07 / R10 | forever; build unknown | unit | none yet | Acceptance column of REQ-PAR-07 | not run | not run |
| V-016 | REQ-PAR-09 / R15b, R15c; W8, W9 | forever; build unknown | integration | none yet | Upload, download and decompress to identical SHA-256; refuse member/anonymous reads; expired or deleted logs are absent. Replaces the superseded R3 upload-path prohibition. | not run (phase 2) | not run |
| V-017 | REQ-SCO-01 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-01 | not run | not run |
| V-018 | REQ-SCO-02 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-02 | not run | not run |
| V-019 | REQ-SCO-03 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-03 | not run | not run |
| V-020 | REQ-SCO-04 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-04 | not run | not run |
| V-021 | REQ-SCO-05 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-05 | not run | not run |
| V-022 | REQ-SCO-06 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-06 | not run | not run |
| V-023 | REQ-SCO-07 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-07 | not run | not run |
| V-024 | REQ-SCO-08 / R12 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-08 | not run | not run |
| V-025 | REQ-SCO-09 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-09 | not run | not run |
| V-026 | REQ-SCO-10 / R5 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-10 | not run | not run |
| V-027 | REQ-SCO-11 / R12 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-11 | not run | not run |
| V-028 | REQ-SCO-12 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-12 | not run | not run |
| V-029 | REQ-SCO-12 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-12 | not run | not run |
| V-030 | REQ-SCO-13 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-13 | not run | not run |
| V-031 | REQ-SCO-14 / R7 | forever; build unknown | unit | none yet | Acceptance column of REQ-SCO-14 | not run | not run |
| V-032 | REQ-DTH-01 / R6 | forever; model values and private logs | unit/integration | `feat/raid-death` | All seven causes, feigns/pets, killing/majority damage, environmental and aggro ambiguity; [module evidence](../../test/raid/death/README.md) | Synthetic tests and mutation controls pass; gated two-log check passes; counts only; native not run | engine pass |
| V-033 | REQ-DTH-02 / R6 | forever; model values and private logs | unit/integration | `feat/raid-death` | Victim/tank ownership, whole living healer team, no owner for unavoidable/wipe/unresolved; [module evidence](../../test/raid/death/README.md) | Synthetic tests and mutation controls pass; gated two-log check passes; counts only; native not run | engine pass |
| V-034 | REQ-DTH-03 / R12 | forever; build unknown | unit | none yet | Acceptance column of REQ-DTH-03 | not run | not run |
| V-035 | REQ-DTH-03 / R6 | forever; model values and private logs | unit/integration | `feat/raid-death` | Sparse-sample suggestion, empty aggro suggestion, deterministic re-review, high/medium/low confidence; [module evidence](../../test/raid/death/README.md) | Synthetic tests and mutation controls pass; gated two-log check passes; counts only; native not run | engine pass |
| V-036 | REQ-DTH-04 / R6 | forever; model values and private logs | unit/integration | `feat/raid-death` | Sample/full-health/span boundaries, effective heal and absorb 0/1, healer death/resurrection; [module evidence](../../test/raid/death/README.md) | Synthetic tests and mutation controls pass; gated two-log check passes; counts only; native not run | engine pass |
| V-037 | REQ-DTH-05 / R12 | forever; build unknown | unit | none yet | Acceptance column of REQ-DTH-05 | not run | not run |
| V-038 | REQ-DTH-05 / R6 | forever; model values and private logs | unit/integration | `feat/raid-death` | First wipe moment, strict-after, dead-share and role toggles, nonempty roles, feigns/pets; [module evidence](../../test/raid/death/README.md) | Synthetic tests and mutation controls pass; private logs have no wipes, so summaries are fixture-proven; native not run | engine pass |
| V-039 | REQ-DTH-06 / R6 | forever; model values and private logs | unit/integration | `feat/raid-death` | Failed-boss-only summary, first-death index/absence, inclusive configurable following window; [module evidence](../../test/raid/death/README.md) | Synthetic tests and mutation controls pass; private logs have no wipes, so summaries are fixture-proven; native not run | engine pass |
| V-040 | REQ-SPC-01 / R5 | forever; build unknown | static | none yet | Acceptance column of REQ-SPC-01 | not run | not run |
| V-041 | REQ-SPC-02 / R5 | forever; build unknown | static | none yet | Acceptance column of REQ-SPC-02 | not run | not run |
| V-042 | REQ-SPC-03 / R5 | forever; build unknown | static | none yet | Acceptance column of REQ-SPC-03 | not run | not run |
| V-043 | REQ-SUP-01 / R13 | forever; build unknown | unit | none yet | Acceptance column of REQ-SUP-01 | not run | not run |
| V-044 | REQ-SUP-02 / R13 | forever; build unknown | unit | none yet | Acceptance column of REQ-SUP-02 | not run | not run |
| V-045 | REQ-SUP-03 / R13 | forever; build unknown | unit | none yet | Acceptance column of REQ-SUP-03 | not run | not run |
| V-046 | REQ-ENC-01 / R5 | forever; build unknown | unit | none yet | Acceptance column of REQ-ENC-01 | not run | not run |
| V-047 | REQ-ENC-02 / R5 | forever; build unknown | unit | none yet | Acceptance column of REQ-ENC-02 | not run | not run |
| V-048 | REQ-ENC-03 / R5 | forever; build unknown | unit | none yet | Acceptance column of REQ-ENC-03 | not run | not run |
| V-049 | REQ-OFF-01 / R8 | forever; build unknown | Windows window | none yet | Acceptance column of REQ-OFF-01 | not run | not run |
| V-050 | REQ-OFF-02 / R12 | forever; build unknown | Windows window | none yet | Acceptance column of REQ-OFF-02 | not run | not run |
| V-051 | REQ-OFF-03 / R14 | forever; build unknown | Windows window | none yet | Acceptance column of REQ-OFF-03 | not run | not run |
| V-052 | REQ-OFF-04 / R14 | forever; build unknown | Windows window | none yet | Acceptance column of REQ-OFF-04 | not run | not run |
| V-053 | REQ-RDR-01 / R14 | forever; build unknown | Windows window | none yet | Acceptance column of REQ-RDR-01 | not run | not run |
| V-054 | REQ-RDR-02 / R14 | forever; build unknown | Windows window | none yet | Acceptance column of REQ-RDR-02 | not run | not run |
| V-055 | REQ-OFF-05 / R8 | forever; build unknown | Windows window | none yet | Acceptance column of REQ-OFF-05 | not run | not run |
| V-056 | REQ-WEB-01 / R10 | forever; build unknown | unit | none yet | Acceptance column of REQ-WEB-01 | not run | not run |
| V-057 | REQ-WEB-02 / R11 | forever; build unknown | unit | none yet | Acceptance column of REQ-WEB-02 | not run | not run |
| V-058 | REQ-WEB-02 / R15 | forever; build unknown | unit | none yet | Acceptance column of REQ-WEB-02 | not run | not run |
| V-059 | REQ-WEB-03 / R15 | forever; build unknown | unit | none yet | Acceptance column of REQ-WEB-03 | not run | not run |
| V-060 | REQ-WEB-05 / R15 | forever; build unknown | unit | none yet | Acceptance column of REQ-WEB-05 | not run | not run |
| V-061 | REQ-WEB-06 / R15 | forever; build unknown | unit | none yet | Acceptance column of REQ-WEB-06 | not run | not run |
| V-062 | REQ-WEB-04 / R8 | forever; build unknown | unit | none yet | Acceptance column of REQ-WEB-04 | not run | not run |
| V-063 | REQ-SR-01 / R16 | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-01 | not run | not run |
| V-064 | REQ-SR-02 / R17 | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-02 | not run | not run |
| V-065 | REQ-SR-03 / R16 | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-03 | not run | not run |
| V-066 | REQ-SR-04 / R16 | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-04 | not run | not run |
| V-067 | REQ-SR-05 / R18 | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-05 | not run | not run |
| V-068 | REQ-SR-06 / R17 | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-06 | not run | not run |
| V-069 | REQ-SR-07 / R16 | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-07 | not run | not run |
| V-070 | REQ-SR-10 / R16 | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-10 | not run | not run |
| V-071 | REQ-SR-08 / R19 | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-08 | not run | not run |
| V-072 | REQ-SR-09 / later | forever; build unknown | simulator | none yet | Acceptance column of REQ-SR-09 | not run | not run |
| V-073 | REQ-DST-01 / R9a | forever; Go tests | unit | `feat/raid-app-folder` | `go test -run 'BundleRaid|RaidGate|BundleState'`: old bundle on the new app; gate bundle fresh and over an earlier copy; rollback; stale-lock recovery; 13 refusals with nothing changed; WTF untouched | pass; native Windows not run (alpha, owner's decision) | unit pass |
| V-074 | REQ-DST-01 / R9b | forever; build unknown | unit | none yet | Acceptance column of REQ-DST-01 | not run | not run |
| V-075 | REQ-DST-02 / every PR | forever; build unknown | unit | none yet | Acceptance column of REQ-DST-02 | not run | not run |
| V-076 | REQ-DST-03 / R0 | forever; build unknown | review | `feat/raid-seam` | Acceptance column of REQ-DST-03: every copied block has a provenance header and a `THIRD_PARTY.md` row | R0 adds no block copied from another addon; `THIRD_PARTY.md` unchanged | verified |
| V-077 | REQ-DST-03 / every PR | forever; build unknown | unit | none yet | Acceptance column of REQ-DST-03 | not run | not run |
| V-078 | REQ-DGN-01 / D1 | forever; model values | unit | `feat/raid-fights` | Focused tests and mutations in R4-EVIDENCE.md | Synthetic tests pass; see R4-EVIDENCE.md; private/native checks not run | synthetic pass, integration open |
| V-079 | REQ-DGN-02 / D1 | forever; model values | unit | `feat/raid-fights` | Focused tests and mutations in R4-EVIDENCE.md | Synthetic tests pass; see R4-EVIDENCE.md; private/native checks not run; real scorer/history writer absent | synthetic pass, integration open |
| V-080 | REQ-DGN-03 / D2 | forever; build unknown | unit | none yet | Acceptance column of REQ-DGN-03 | not run | not run |
| V-081 | REQ-DGN-04 / D2 | forever; build unknown | unit | none yet | Acceptance column of REQ-DGN-04 | not run | not run |
| V-082 | REQ-DGN-05 / D3 | forever; build unknown | unit | none yet | Acceptance column of REQ-DGN-05 | not run | not run |

Modes: `unit` (Go package tests), `simulator` (the mock client in `test/raid/_client/`, which is not the game), `static`, `Windows window`. No row is validated in the game; those checks are recorded as "not run (alpha, owner's decision)".

## Blocked, deferred or replaced work

| REQ / row | State | Cause | Next action | Effect |
|---|---|---|---|---|
| REQ-PAR-* (decoder body) | blocked | No real Forever log (G0) | R1 and R9 put a logger on officers' PCs; decode the first log | Until then the decoder is tested on synthetic lines only |
| REQ-WEB-03, 05, REQ-SR-09 | blocked | Q-04 unanswered | Site maintainer | Local export and import cover phase 2 |
| REQ-SR-06 (new raids) | blocked | No published loot table for Mount Hyjal or Barrow Deeps as of 2026-11-18 | Record drops with R18; re-check AtlasLoot after the raids open | Reserving by link or id |

## Resume checkpoint

- Updated by / at: architect, 2026-11-18
- Repository / branch: `thegamersvibe/relax-coach`, `feat/raid-seam`
- Exact HEAD revision: main `0572cf0` when planned
- Working tree: the R0 seam (model, rules, runner, mock client, CI steps, docs)
- Last completed slice: R0 built, awaiting review
- Last verified state: none
- Current slice: R0
- Next executable action: none until the build lock (DEC-37) is lifted; then R0
- Open questions / blockers: Q-03; Q-07 (the DEC-37 build lock was lifted on 2026-11-19)
- Decisions to retain: DEC-36; app release before bundle for the new folder
