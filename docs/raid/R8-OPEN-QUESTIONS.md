# R8 officer page: stopped portions

The approved mock-up is the target. These gaps are not permission to invent
new data, engine rules, deployment paths or night-level scoring arithmetic.

1. **Night opening / config deployment.** `cfg.Load(dir)` reads external JSON.
   There is no runtime raid-config location or installation wiring in this base;
   the bundle builder does not ship `rules/raid`. Where are those replaceable
   files installed/read? The native route currently opens an explicit unavailable
   notice for admitted officers, not a fabricated night. No file picker or
   automatic log read is claimed. The stream-to-page pipeline is exercised by
   synthetic and opt-in private-log tests and is ready to take a caller's config.
2. **Player detail layout.** The mock-up has a standalone Deaths part and a
   night-level part table. The engine has output/activity/mechanics/utility/
   preparation; deaths are inside mechanics. Rounded, covered evidence is per
   fight. `NightRoleParts` supplies exact comparison values, not covered/rounded
   night Parts, and its contract says comparison-only. The renderer therefore
   shows the real per-fight parts, weights, baselines, measures and raw evidence;
   it does not synthesize the mock-up's night table. Confirm this layout or
   supply an engine-owned night-part presentation contract.
3. **Comparison engine follow-up.** At the pinned base, `ParseOfficer` does not
   skip a UTF-8 BOM or trim names before matching/duplicate checks. `Compare`
   returns a best part even with zero/wrong-sign impact, and a no-scored-fights
   role has the generic no-role-comparison reason. These differ from the later
   brief rulings. No compare package edit is included. Basic file errors,
   agreements, disagreements, no-rank/no-match rows and omitted top/bottom names
   render. The lead assigned the engine fix to a separate worktree and will
   rebase this work onto it. This page passes through the engine's part fields
   and reasons without reimplementing the rules; exact rational values are
   rounded once to tenths for display. Fewer than ten ranked players get an
   explicit top-five/bottom-five overlap note. A selected prediction file is
   read anew, bounded and read-only; opening it from the live UI awaits item 1.
4. **Labels with no supplied data.** `new_at_60_seconds` is unset. There is no
   verified aura-to-consumable map for a claim such as "no flask used". Neither
   label is invented. Prep clocks remain raw server seconds with their meaning;
   a received stamp is not necessarily first receipt. The mock-up's 20-second
   wipe follow-up window is illustrative: the renderer uses the configured
   summary window. Unknown causes stay unknown; only the approved boss-melee
   unresolved case gets "aggro: puller or tank".

## Implemented stage-1 lock

The rank writer is a separate additive addon lane; this branch only reads.
One selected-account `RelaxRaid.lua` is opened through the existing contained,
identity-checked account reader. There is no other-account fallback or stale
unlock. Every row must validate before any officer unlocks the page. Limits:
16 MiB file, 1,000,000 literal entries, 256 character rank rows, 64-byte full
names, optional guild string at most 256 bytes, positive integral server seconds
at most 4,294,967,295. Unknown results/fields and wrong types lock the page.
Unknown root fields remain additive. Neither save nor prediction is written.

No Raids DOM is rendered for a locked account. Account/folder transitions remove
it before a reply; late page replies cannot restore it. The native route checks
the selected save afresh, regardless of what the UI last displayed.

## Offline checks

`go test -race -count=1 ./...` in `desktop`; the existing jsdom UI suite after
Go fixture generation; `tools/raidtest.py --mutants raidlock` and `raidpage`.
`RELAX_RAID_REAL_LOGS` enables page rendering of private logs and prints counts
only, never names, identifiers, paths or raw lines. UI fixtures are synthetic.

Not tested on a live client: not run (alpha, owner's decision)
