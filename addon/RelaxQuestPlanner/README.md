# Relax Quest Planner — 0.4.0

Quest planner for **WoW Forever** (Interface 16001), shipped in the Relax Coach
bundle from **v1.5.0**. It plans dungeon quest chains and individual quest goals
without accepting, completing or turning in quests for you.

**Validation status:** version 0.4.0 has passed the offline test suites only.
**Not tested in game: alpha, owner's decision.** Please report anything that
misbehaves in the native Forever client.

## What's new in 0.4.0

- **Dungeon chains:** installed ForeverDungeonJournal pages now supply dungeon
  membership even when QuestieDB already knows the quests. Wings merge through
  the existing dungeon aliases. Hidden entries are lead-ins, not extra targets.
- **Tailored routes:** completed, faction, class, race and unavailable filters
  apply to journal candidates; level-locked quests stay under later. Conflicting
  faction sources keep the quest with a **check faction** note.
- **Less clutter:** non-journal quests that merely use a dungeon stay behind
  **N more quests use this dungeon (class, crafting or set quests)**. Click the
  count to read the list; quests already in your log remain in the route.
- **Outside first:** **Before you go**, **Inside <dungeon>**, **After the run** and
  **Next run** separate preparation, inside pickups/objectives/turn-ins, outside
  follow-ups and a return trip. Known QuestieDB NPC/object zones win over journal
  text; disagreements and unconfirmed locations are labelled. Talk/delivery
  quests use their endpoints rather than their dungeon category, so outside
  breadcrumbs do not force an unnecessary second run. Preparation notes
  list journal-required items, professions and attunements without adding gates.
- Journal chain order is a hint, not a new prerequisite. It never changes quest
  state or creates an action cycle; existing prerequisite claims remain intact.
- Journey keeps the engine's action order and carries block headings and notes.
  No automatic quest actions, new network sends or donor records are shipped.
  With no usable journal page, routes retain the 0.3.0 behavior; a missing journal
  is labelled **limited data**. Individual quest-goal routing is unchanged.
- Public synthetic Lua 5.1 fixtures cover the character-shaped routes. The
  private current-data suite requires QuestieDB source tables not present in the
  installed packed addon; native testing is **not run (alpha, owner's decision)**.

## What's new in 0.3.0

- **Quest and item review:** `/rqp review` opens a separate, read-only window with
  **KEEP** / **UNCERTAIN-KEEP** reasons and source details. It never deletes items
  or abandons quests; missing data is not permission to remove anything.
- Shipped in Relax Coach **1.7.1**. On app **1.7.0** use **Updates → Update
  everything**, or **Update App** first and then **Updates → Check → Update**;
  the app updates itself before the addons. Then follow the app's restart advice
  for the new planner files. Older apps need **Update App** first as before.

## What's new in 0.2.2

- Keep quest-state checks and changed-route replanning active while the planner
  is hidden, including map pins and the Journey handoff.
- Recheck current state when opening the planner without rebuilding an unchanged
  route. Turning auto-pin on also checks current state.
- Keep the highest observed session level when duplicate or out-of-order
  level-up events arrive while the client level API lags.
- Retry quest-state checks 1 and 3 seconds after turn-in. These are best effort:
  silent changes after the retries can remain unseen until another game event or
  opening the planner. There is no periodic recovery timer.

These are correctness fixes, not a performance improvement claim.

## Quest and item review

Open **Review** in the planner header or type **`/rqp review`** for **Quest and
item review**. **`/rqp cleanup`** remains an alias. This separate window needs no
active route, Journey or Coach. It reads only backpack + four
carried bags and current quests. It lists quest-class/quest-associated items and
grey quests (also unresolved greyness); other items, keyring, reagent bag, bank,
equipment, mail and alts **were not assessed**. Use Quests / Quest items, page
controls and click a row for scrollable **Details**. Hover the status for the full
read-success/failure list. No candidate count or disposal recommendation exists.

Only two labels are possible:

- **KEEP**: a known open/future quest use, starter, repeatable, dungeon/higher-level
  path, Keep pin or positive item-use fact. All stacks of an ID stay together.
- **UNCERTAIN-KEEP**, with exactly one of:
  - **New in Forever, no use data** — outside the imported vanilla ID ranges.
  - **Quest state could not be resolved** — incomplete/unreadable state or graph.
  - **No use found in our data** — all represented quest uses recorded turned in,
    no imported positive-use fact, inside the vanilla list, with readable inputs.
    These rows sort first. This is information, **not permission to remove it**.

Details identify exact IDs, linked quests, relation types, sources/versions and
snapshot time. They explicitly do not certify destroy side effects, reacquisition
or uses added after data build 70888. Furen's Instructions (6842) is covered by the
synthetic completed-history regression as **UNCERTAIN-KEEP / No use found in our
data**, never disposable. Native quest-log and bag decisions remain entirely
manual; this feature cannot act on quests/items or send messages.

**Keep pinned** adds protection; Unpin only removes that pin. Only schema version,
up to 100 quest + 100 item pins per character, and window position are stored in
`RelaxQuestPlannerDB.cleanup`. Newer preference schemas are left untouched. No
verdict, inventory, provider record or character identity is persisted. Closing
cancels scanning; hidden events only mark it dirty. Explicit Refresh rebuilds the
session index; visible player-state events coalesce for 250 ms and retake only
the snapshot without restarting an in-progress static index. Combat pauses scans,
and stale snapshot generations cannot publish. A snapshot changing without a
corresponding event during scanning asks for Refresh rather than looping endlessly.

QuestieDB contract 4 corrected getters are optional. Provider absence/failure is
visible limited data, not an empty-success result. Unknown revision is not treated
as a version guarantee: explicit refresh and addon/world changes invalidate the
session index; quest membership changes invalidate only the player snapshot.
Dungeon and future protections do not use navigation mode, skipped quests or
today's selected route. All known faction/class paths are
retained conservatively, even when eligibility cannot be established. Optional
supplements add protection; negative FDJ placeholders are never queried as real IDs.

**Data provenance:** `Core/CleanupUses.lua` contains only generated positive facts
from Keep or Toss: Forever 1.7.0 (Aquary), not its code or verdict logic. See
`THIRD_PARTY.md`. Main data build 1.60.1.70888, known-use notes 1.60.1.70790, vanilla
ID ranges 1.13.7.38704. Absence cannot clear an item; non-hunter pet-food evidence
keeps a generic other-class-use protection without showing a pet-food reason.
Profession names are read from the client, falling back to exact profession IDs.

**Offline verification:** pinned `lupa==2.8`, explicitly `lupa.lua51`, with authored
API/provider stubs based on Gethe/wow-ui-source `bebcb6bf309cf41b41bd74f8ca651e6a105b2676`
(Forever 1.60.1.70938). No native tests/captures are requested or needed for this
scope. Actual client API behavior, full-bag frame time and layout at the owner's UI
scale remain unverified. No cleanup profiles or capture command are built.

```sh
# Inside an isolated checkout with .venv installed from .github/requirements-ci.txt
export TMPDIR="$PWD/.git/tmp"
mkdir -p "$TMPDIR"
export CLEANUP_USES_ZIP="$TMPDIR/KeepOrToss-1.7.0.zip"
curl --fail --location -o "$CLEANUP_USES_ZIP" https://mediafilez.forgecdn.net/files/9575/258/KeepOrToss-1.7.0.zip
.venv/bin/python -I -B tools/gen_cleanup_uses.py "$CLEANUP_USES_ZIP" --check
.venv/bin/python -I -B -m unittest discover -s test/planner -p 'cleanup_*_test.py'
```

The generator verifies the pinned archive hash **before** parsing; its first-party
literal parser never executes donor modules. CI checks literal-only data after a
fixed namespace binding, every provenance header, counts and byte-identical
regeneration. Static tests reject destructive/action/send API names (including
aliases) in feature and integration files. In-memory mutants must fail when
"no use found" becomes a disposal label/wording or imported tool evidence is lost.
The bundle manifest already admits these Lua files; no app/updater change is needed.

## Install

From **Relax Coach v1.5.0**, Relax Quest Planner ships inside the combined Relax
Coach bundle (`RelaxCoach-Bundle-<version>.zip`; from v1.8.0
`RelaxCoach-RaidBundle-<version>.zip`) together with Coach, Relax
Journey and the guide content. It is no longer a separate patch zip and needs no
Journey files merged by hand.

- **Windows app:** use **Check App Update → Update App** first if the app is older
  than 1.7.0, then use **Updates → Check → Update** (labelled
  **Check for updates → Update Addon** in app 1.5.0). From app 1.5.1 a new
  player can also use **Get started → Install** with only `RelaxCoach.exe`. An
  existing `Interface/AddOns/RelaxQuestPlanner` folder (for example an earlier
  test version you copied in) is moved into the update's backup in
  `RelaxCoachBackups`, not deleted.
- **Manual install:** close WoW and merge the bundle's `Interface` folder into the
  game folder, or run the bundle's `Install-RelaxCoach.ps1`, which also backs up
  an existing planner folder. Its `.toc` must be directly inside
  `Interface/AddOns/RelaxQuestPlanner/`.

Leave `WTF`/SavedVariables and unrelated addons alone. Enable Relax Quest Planner
and Journey at character selection. Enable **QuestieDB** for full data;
**ForeverDungeonJournal** adds dungeon membership, chain order and location
notes (and supplies a limited fallback without QuestieDB). No third-party quest
database is bundled with this addon. Log in and type `/rqp`. New routes open in
Journey by default. If Journey or its integration is missing, route planning
still works; guide picking is disabled.

## Commands

- `/rqp` or `/rqp show` — show the planner; `/rqp hide` hides it. Escape closes windows.
- `/rqp dungeon [name]` — open the dungeon picker, or plan the named dungeon.
- `/rqp goal <questID>` — plan a quest and its prerequisites. You can also focus
  the quest box and shift-click a quest link.
- `/rqp guide` — shows the top 3 guides; click one to jump to your first undone step.
- `/rqp review` — open Quest and item review (read-only); `/rqp cleanup` is an alias.
- `/rqp mode [strict|game|all]` — choose a level filter. With no argument, cycle
  Game → Strict → All → Game. Strict hides quests more than five levels below
  you; Game hides grey quests; All includes lower-level quests. Required low-level
  prerequisites remain marked as your choice; level-locked quests appear later.
- `/rqp map` — pin the current step when a location is known.
- `/rqp skip` — skip the current quest for this character, not abandon it.
- `/rqp unskip` — restore all skipped quests, including in the active Journey route.
- `/rqp autopin` — toggle automatic map pins.
- `/rqp autojourney` — toggle opening newly selected routes in Journey.
- `/rqp error` — show the last recorded error; `/rqp help` lists commands.

Replans and skip changes refresh Journey only while its active/last guide is the
planner route. A newer manual guide choice is preserved. Route updates requested
in combat wait until combat ends.

For a bug report, include the addon version, actual Forever client/build, level,
faction, chosen dungeon/quest and mode, reproduction steps, and `/rqp error`
output. Avoid sharing account identifiers, full saves or private guide content.
To roll back, close WoW and restore the previous folders from the update backup
in `RelaxCoachBackups` (or use the bundle's `Restore-RelaxCoach.ps1` after a scripted
install); retain your saved variables and other addons.
