# R8 officer page: rulings resolved

The page follows the corrected officer mock-up and the raid-1 rulings. It does
not add scoring arithmetic, historical baselines or inferred preparation facts.

## Resolved decisions

1. **Embedded configuration.** The nine `rules/raid/*.json` files are copied by
   `go generate` in `desktop/` (or `python3 scripts/gen_raid_rules.py`) and compiled
   into the executable. `cfg.LoadFS(fs.FS)` is additive; `Load(dir)` delegates to
   it and remains uncached. The app loads once at startup, retains either the
   result or the error, and gives callers detached snapshots. An error is shown
   on the admitted Raids page with no night or opening controls. There is no
   override beside the executable, in app data or in an update channel: one app
   version means one config. CI compares the complete embedded inventory and
   bytes to the canonical rules before regeneration. A tuning change requires
   an app release until a separately designed rule-update channel exists.
2. **Night and fight detail.** The night table precedes the fights and displays
   Output, Activity, Mechanics, Utility and Preparation. Its role weights come
   from config; its values come directly from `score.NightRoleParts`, rounded
   once to tenths. An absent value is `not measured`; there is no night coverage
   or baseline column. No night role means the engine's no-scored-fights or
   evenly-split-role reason replaces the table. Deaths remain a measure under
   Mechanics, not a sixth part. The score package changed only its API comment.
   The header counts boss pulls, wipes and kills separately. Weights use percent;
   absent ranks do not become rank zero, and negative zero displays as zero.
3. **Live opening and comparison.** An admitted officer can open the newest log
   from the selected game's Logs folder, choose a file through the existing
   native picker, or paste a path. Multiple nights have a selector. Every action
   rereads the selected account and retains its lock through reading/rendering.
   A picker releases that lock while open, then rechecks admission and selection
   generation before accepting a path. Switching game/account clears selection.
   Officer files are read anew from the chosen path, bounded and read-only,
   passed through `compare.ParseOfficer` directly to `Compare`, and never kept
   in app settings or a night file. A parser refusal removes the comparison and
   leaves the night visible. Cancellation leaves the displayed page unchanged.
   The comparison follow-up is already in the preserved R8 ancestry; the page
   passes its reasons and exact values through without recreating its rules.
4. **Labels with no data.** Raid 1 omits `new at 60` and `no flask`-style usage
   claims. Preparation displays reported stock by category and better items in
   bags, not inferred use. The wipe follow-up window comes from config. Prep
   timestamps remain raw server seconds with their meaning; an earliest stored
   receipt is not necessarily the first receipt. Flags are the engine's list.

## Stage-1 lock

The separate addon lane writes rank evidence; this page only reads. One selected
account's `RelaxRaid.lua` is opened through the existing contained,
identity-checked account reader. There is no other-account fallback or stale
unlock. Limits are 16 MiB, 1,000,000 literal entries, 50 **raw rank entries**,
64-byte full names, optional guild text at most 96 bytes without control bytes,
and positive integral server seconds at most 4,294,967,295.

Every rank key must be a literal Lua string in `Name-Realm` form. A malformed key,
repeated key anywhere in the save, or excessive raw entry count locks the whole
page before maps can erase that evidence. Invalid row values cannot admit but
need not poison another valid officer. Non-officer rows may omit `readAt`.
Unknown root fields remain additive; other save readers keep their behavior.

An officer's reading must be at most 14 days old or one day ahead of the PC's
UTC clock. `raidOfficerMaxAge` remains the tunable starting limit. Missing, bad,
stale and far-future times do not admit. Tests cover exact boundaries, timezone
and expiry without refresh. This is a convenience lock, not authorization.

Locked accounts have no Raids navigation or page DOM. Account/folder changes
invalidate held page and picker replies. A guarded route refusal also latches
its message and verdict for that account: a late allowed refresh cannot restore
the page or replace the message. A later manual refresh may unlock only after
another successful guarded route check, with both context and verdict-generation
checks. Neither save nor prediction is written.

## Display safety

Every name, including officer, logger, player, owner, enemy, spell and comparison
names, is HTML-escaped inside `bdi`. Display-only stripping covers U+200B–U+200F,
U+202A–U+202E, U+2066–U+2069, U+061C, U+FEFF, U+2060–U+2064, U+00AD and U+180E.
Engine names and matching inputs are untouched. Go and jsdom tests exercise all
name locations with markup, quotes, Arabic and these controls. Both template
parsers register the same helpers.

## Verification and remaining boundaries

Run desktop race tests, the existing jsdom suite after Go fixture generation,
raid module gates, and the cfg, cfgembed, raidlock, raidpage and compare mutants.
`RELAX_RAID_REAL_LOGS` opts into opening private logs through the wired native
route with the embedded config. Output is counts only, never source paths,
player names, identifiers or raw lines. Public fixtures are synthetic.

No unresolved raid-1 page rulings remain. Stage-2 server authorization and a
rule-update channel remain separate future lanes. Cross-builds and offline DOM
tests do not establish Windows picker or live-client acceptance.

Not tested on a live client: not run (alpha, owner's decision)
