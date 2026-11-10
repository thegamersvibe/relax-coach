# Relax Quest Planner — 0.2.0

Quest planner for **WoW Forever** (Interface 16001), shipped in the Relax Coach
bundle from **v1.5.0**. It plans dungeon quest chains and individual quest goals
without accepting, completing or turning in quests for you.

**Validation status:** version 0.2.0 has passed the offline test suites only. It
has not been validated in the native Forever client yet; please report anything
that misbehaves in game.

## Install

From **Relax Coach v1.5.0**, Relax Quest Planner ships inside the combined Relax
Coach bundle (`RelaxCoach-Bundle-<version>.zip`) together with Coach, Relax
Journey and the guide content. It is no longer a separate patch zip and needs no
Journey files merged by hand.

- **Windows app:** use **Check App Update → Update App** first if the app is older
  than 1.5.0, then close WoW and use **Check for updates → Update Addon**. An
  existing `Interface/AddOns/RelaxQuestPlanner` folder (for example an earlier
  test version you copied in) is moved into the update's backup in
  `RelaxCoachBackups`, not deleted.
- **Manual install:** close WoW and merge the bundle's `Interface` folder into the
  game folder, or run the bundle's `Install-RelaxCoach.ps1`, which also backs up
  an existing planner folder. Its `.toc` must be directly inside
  `Interface/AddOns/RelaxQuestPlanner/`.

Leave `WTF`/SavedVariables and unrelated addons alone. Enable Relax Quest Planner
and Journey at character selection. Enable **QuestieDB** for full data;
**ForeverDungeonJournal** provides a limited fallback. No third-party quest
database is bundled with this addon. Log in and type `/rqp`. New routes open in
Journey by default. If Journey or its integration is missing, route planning
still works; guide picking is disabled.

## Commands

- `/rqp` or `/rqp show` — show the planner; `/rqp hide` hides it. Escape closes windows.
- `/rqp dungeon [name]` — open the dungeon picker, or plan the named dungeon.
- `/rqp goal <questID>` — plan a quest and its prerequisites. You can also focus
  the quest box and shift-click a quest link.
- `/rqp guide` — shows the top 3 guides; click one to jump to your first undone step.
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
