# Relax Coach — guild bundle

Four **separate WoW Forever addons**, plus a native Windows report app:

- **Relax Coach** — leveling, equipment, talent, dungeon and session advice. Works without Journey.
- **Relax Journey** — the rebranded Journey guide UI and the included guide-content pack. Can be enabled independently of Coach.
- **Relax Quest Planner** (from v1.5.0) — dungeon quest chains, quest prerequisites and a guide picker (`/rqp`). Works without Coach; Journey is optional.
- **Relax Raid** (from v1.8.0) — the guild raid logger: turns on the game's combat log in raids (`/rraid`) and, from v1.8.1, sends a short preparation report to the raid. Works without the other addons.
- **RelaxCoach.exe** — a real Windows app window with account selection, saved-character reports and explicit app and addon/content updates.

## Download

**[Download release v1.8.1](https://github.com/thegamersvibe/relax-coach/releases/tag/v1.8.1)**:

- `RelaxCoach-RaidBundle-1.8.1.zip` — the four independent addons (Coach, Journey, Relax Quest Planner 0.4.0, Relax Raid 0.2.0), Journey guide content **4.00.12**, the Windows app and optional installer/restore scripts. From v1.8.0 the combined download is named `RelaxCoach-RaidBundle-<version>.zip` because it adds the Relax Raid folder; v1.5.0 to v1.7.2 used `RelaxCoach-Bundle-<version>.zip`, and earlier releases `RelaxCoach-Guild-<version>.zip`.
- `RelaxCoach.exe` — app-only download, for updating the application separately.
- `SHA256SUMS.txt` — checksums for these release assets.

**On app 1.8.0?** Use **Updates → Update everything**, or **Check App Update → Update App** first and then **Updates → Check → Update**. Because this release includes a new app, app 1.8.0 updates itself before it installs the newer addons. Relax Raid gets new files, so exit WoW completely and start it again after the update (`/reload` is not enough).

**On app 1.7.x?** Run **Check App Update → Update App** first, then **Updates → Check → Update** (or use **Updates → Update everything**). Apps before 1.8.0 do not know the new Relax Raid folder: they report this release as app-only and do not install it until the app has updated itself. After the addon update, exit WoW completely and start it again, because Relax Raid is a new addon folder.

**On an app older than 1.7.0?** Run **Check App Update → Update App** first, then **Update Addon** (**Updates → Check → Update** on recent apps). App 1.6.0/1.6.1 also offers **Updates → Update everything**, which does the app first. From 1.8.0 the bundle has a new name (`RelaxCoach-RaidBundle-<version>.zip`), so older apps report the release as app-only until the app has updated itself. Update the app even if an earlier app-update failure lets you try the addon update directly.

**On app 1.5.0 to 1.5.6?** Use **Check App Update → Update App** first (app 1.6.0 brings Update everything, the installed copy with shortcuts and the automatic addon update; app 1.5.6 is needed to install the campfire songs; the new look and the launch update notice come with app 1.5.5; the Farming page with 1.5.3; installing while WoW is open with 1.5.2; Get started and the repair with 1.5.1), then **Updates → Check → Update**.

**On app 1.4.x?** Use **Check App Update → Update App** first, then close WoW and use **Check for updates → Update Addon**. Apps before 1.5.0 cannot install the bundle (from 1.5.0) and report it as an app-only release until the app is updated. App 1.4.0 needs one manual EXE replacement with Relax Coach closed before it can update itself.

There is deliberately no Coach-only `RelaxCoach-1.8.1.zip`: older apps must not report a completed addon update while leaving the guide content behind. The previous app-only v1.4.1 release remains unchanged.

## What's new in v1.8.1

- **Relax Raid 0.2.0 — raid preparation reports.** In a raid group, your Relax Raid sends a short report about your own character to the raid: class and level, the consumables in your bags (by type and count), better gear you already own (from Relax Coach's equipment advice when Coach is installed, otherwise "not checked"), and your time played at your level (at level 60 only; below 60 it is sent as unreadable). It is sent when you join the raid, at a ready check and after each boss fight; never in combat, during a boss fight, or in battlegrounds and arenas. It goes over the hidden addon channel to your raid only and never posts in chat. To read your time played, Relax Raid asks the game when you join a raid, and again after a login or `/reload` while in one, so the game's time-played lines may appear in your chat then. It goes to everyone in your raid running Relax Raid; guild members' copies are kept (every guild member logs raids by default) and the officers' tools read them. It is information, not a score. There is no switch to stop sending it.
- **Kept copies have fixed limits:** the last 3 raid nights, 80 report windows and 40 players per window, at most 100,000 bytes per character per night and 4,000,000 bytes per night in total. A report that has not changed is not stored twice. Reports saved by a newer Relax Raid are left untouched.
- **Hot items: names, colours, supply and fallers (app, Farming page).**
  - Items show their **names and quality colours** instead of item numbers. Relax Coach now notes the names of the items in AH Profit's price history (at login and after you close the auction house, paused in combat, up to 10,000 items). If you see **(update Relax Coach and visit the auction house to name more)**, update Relax Coach, open and close the auction house in game, then `/reload` and press **Refresh** in the app.
  - A new **Down since yesterday** list sits next to **Up since yesterday**: up to 10 items each, rises in green and falls in red. **dump** marks a fall with at least twice the usual number listed.
  - Both lists now need at least 10 units listed on the scan day and show **listed / usual**; a rise on a collapsed supply is left out.
- Coach and the Windows app move to **1.8.1**; Relax Raid moves to **0.2.0**. Relax Journey (`4.00.04`), guide content (`4.00.12`) and Relax Quest Planner (`0.4.0`) are unchanged.
- **Update:** on app 1.8.0 use **Updates → Update everything**, or **Check App Update → Update App** first and then **Updates → Check → Update**; then restart WoW once (new Relax Raid files). Apps before 1.8.0 update the app first, as for 1.8.0.
- **Not tested in game: alpha, owner's decision.** Offline tests do not validate native Forever behavior or the Windows update flow.

## What's new in v1.8.0

- **New addon: Relax Raid 0.1.0 — the raid logger.** When you enter a raid, Relax Raid turns on the game's combat log and Advanced Combat Logging for you, so the guild has a full record of every raid night. By default raids log for **every guild member**, five-player dungeons only for the officer ranks, and characters outside a guild do not log; your own `/rraid logger` choice always wins.
- **Don't delete your combat log.** It is large (several hundred MB a raid night) and is the raw material for later raid reports. Keep `WoWCombatLog` files until the guild has collected them.
- **`/rraid`** (or `/relaxraid`): `/rraid status` shows whether this character logs and whether logging is on now; `/rraid logger on`, `off` or `default` changes it for this character. `off` only stops Relax Raid from starting the log; it never turns off a log that is already running. The same switch is in **Options → AddOns → Relax Raid**. If the game could not turn logging on, Relax Raid tells you to type `/combatlog` yourself.
- **App first:** this release adds the Relax Raid folder, so apps before 1.8.0 must run **Check App Update → Update App** first, then **Updates → Check → Update** (or **Updates → Update everything**). The download is now `RelaxCoach-RaidBundle-1.8.0.zip`.
- **Guide content 4.00.12:** 74 of the 201 guide files updated; the catalog now lists 127 guides (was 125), adding Riverglades 43 (Alliance) and 44 (Horde). Relax Journey code stays `4.00.04`.
- **Alliance guides 30–60 stop on 18 December 2026.** The 44 Alliance guides for levels 30 to 60 carry the content's validity date of 18 December 2026; after that date Journey no longer opens them until a newer guide-content update ships. Horde guides and Alliance guides below 30 have no such date.
- **Relax Quest Planner 0.4.0 — dungeon chains:** dungeon quest chains are tailored to each character (finished, faction, class and race filters; level-locked quests stay under later), using ForeverDungeonJournal pages when that addon is installed. Dungeon steps come last in the route, split into **Before you go**, **Inside <dungeon>**, **After the run** and **Next run**, and class, crafting or set quests that merely use a dungeon sit behind a count you can click. It plans only; it never accepts, completes or turns in quests. Details in the [planner README](https://github.com/thegamersvibe/relax-coach/blob/v1.8.0/addon/RelaxQuestPlanner/README.md).
- **Hot items on the Farming page (app):** for each auction house, up to 10 items that are clearly up: usually at least 20% above the previous recorded day (yesterday, or the day before if a day is missing) and at least 20% above their usual price. While an auction house has too little history for a usual price, the list compares one day with the previous one only. Each item has a small chart, and a **spike** marker when the price is well above usual and the highest seen. The list comes from AH Profit's own saved scans; without a scan today it shows the last scan day's list with its date. Guild-only members see a note until AH Profit keeps a guild price history.
- Coach and the Windows app move to **1.8.0**. Relax Journey (`4.00.04`) is unchanged.
- **Not tested in game: alpha, owner's decision.** Offline tests do not validate native Forever behavior or the Windows update flow.

## What's new in v1.7.2

- **Gear advice works again on current GearQuest.** GearQuest now ships its pick lists in a packed layout; Coach 1.7.1 could not read it and said GearQuest had no data for your class. Coach reads both the packed and the old layout. If GearQuest changes its format again, Coach and the app say so ("GearQuest changed its data format") instead of asking you to enable it.
- **No battleground rewards in gear advice.** Items from battleground quartermasters and honor vendors (for example Outrunner's Bow from Illiyana Moonblaze in Ashenvale, which needs Silverwing Sentinels reputation) are no longer suggested.
- **A trip must be worth it.** A suggestion must gain enough against your whole equipped set: about 1% for something nearby or on your route, 3% for a dungeon, 5% for a zone behind you (more than 6 levels below you) or off your route. Small upgrades far away are dropped. Percentages are now the gain over the item you wear, and are shown once at least 8 slots can be scored.
- **Level 60:** no route wording, no vendor items with no known seller or place, and raid loot only for raids open on Forever. None are open yet (the first raid opens 9 December), so raid drops, raid-boss loot and raid-kill reward quests are not suggested for now.
- Coach and the Windows app move to **1.7.2**. Relax Journey (`4.00.04`), guide content (`4.00.08`) and Relax Quest Planner (`0.3.0`) are unchanged. Rules data moves to **1.3.0**.
- **Update:** on app 1.7.0 or 1.7.1 use **Updates → Update everything**, or **Check App Update → Update App** first and then **Updates → Check → Update**.
- **Not tested in game: alpha, owner's decision.** Offline tests do not validate native Forever behavior or the Windows update flow.

## What's new in v1.7.1

- **Relax Quest Planner 0.3.0 — Quest and item review:** open **Review** in the planner header or type `/rqp review`. The separate window reviews current quests and quest-associated items in your carried bags; no active route, Journey or Coach is required.
- **Read-only:** rows show **KEEP** or **UNCERTAIN-KEEP**, with reasons and source details. Missing or incomplete data is not permission to remove anything. The window never deletes items, abandons quests or takes action for you.
- **Update:** on app 1.7.0 use **Updates → Update everything**, or update the app first (**Check App Update → Update App**) and then the addons. App 1.7.0 asks for the app update first because this release includes a new app. The release gate also checks that the published app **v1.7.0's installer accepts this bundle**, the fallback if an app update fails. On older apps, use **Check App Update → Update App** first as before. Follow the app's restart advice for the new planner files.
- Coach and the Windows app move to **1.7.1**. Relax Journey (`4.00.04`) and guide content (`4.00.08`) are unchanged. The flight sound already shipped in 1.7.0; it is not new in this release.
- **Not tested in game: alpha, owner's decision.** Offline tests do not validate native Forever behavior or the Windows update flow.

## What's new in v1.7.0

- Expanded Fun Mode, with an independent `/coach fun flight on|off` sound switch (on by default). `/coach fun status` shows both controls; `/coach fun off` remains the master mute.
- **App first:** apps older than 1.7.0 must run **Check App Update → Update App** first, then **Update Addon** (**Updates → Check → Update** on recent apps). This bundle adds one media file; older apps refuse it. After the addon update, follow the app's restart advice for the new file.
- **Release check:** for **1.7.0 only**, the last published app must refuse the real bundle for exactly the new media path and accept the new `RelaxCoach.exe`. Both existing-backup and no-prior-backup installs are checked: all installed files and existing backups stay unchanged; only a newly created, empty `RelaxCoachBackups` directory is allowed, with no leftover stage, transaction or lock. Every other version keeps the old-app acceptance check. The old installer's fallback message names the unknown file and links a manual app download, rather than saying "Update the app first"; see [App updates](desktop/APP_UPDATES.md#app-first-release-170) for the exact text.
- Relax Journey (`4.00.04`), guide content (`4.00.08`) and Relax Quest Planner (`0.2.2`) are unchanged.
- **Validation limit:** native Forever playback, event timing and Windows update behavior have not been tested for this change.

## What's new in v1.6.1

- **Relax Quest Planner 0.2.2:** quest-state checks and changed-route replanning stay active while the planner is hidden, including map pins and the Journey handoff. Opening the planner checks current state without rebuilding an unchanged route; turning auto-pin on also checks current state.
- Duplicate or out-of-order level-up events no longer lower the planner's session level while the client level API lags.
- Quest turn-in schedules extra state checks after 1 and 3 seconds. These are best effort: a silent completion after those checks can remain unseen until another game event or opening the planner. There is no periodic recovery timer.
- **More addons:** LayerEnemies is available as an optional **Alpha** entry, downloaded from its own repository only when you choose to install it. It is not part of the Relax Coach bundle.
- **Update:** on app 1.6.0 use **Updates → Update everything**, or update the app first and then the addons. After the addon update, follow the app's `/reload` or restart advice.
- Coach's addon only changes its version. Relax Journey (`4.00.04`), guide content (`4.00.08`), signed catalog, media and bundle file set are unchanged.
- **Validation limit:** these are correctness fixes, not a performance improvement claim. Offline tests do not verify native Forever event timing, combat/taint or the live Journey cursor; those remain untested for this change.

## What's new in v1.6.0

- **Install on this PC:** Get started, step 1 copies Relax Coach to its own folder and adds Start menu and Desktop shortcuts. If a shortcut goes missing, **Settings → Repair shortcuts** puts it back; an existing shortcut that points somewhere else is only replaced after you confirm.
- **Update everything:** one button on Updates updates the app first, then the Relax Coach addons, then the More addons you have installed. It only updates what you have, never installs something new, and stops at the first problem with a note of what was done.
- **Automatic after an app update:** when the app has just updated itself, it updates your addons by itself once, if WoW is closed. If WoW is open, Update everything is marked instead.
- **App first:** when a release comes with a new app, the app updates itself before it installs the new addons. If the app cannot update itself, the message links the manual download.
- The window fits your screen, and only one copy of Relax Coach runs at a time: opening it again brings the open window to the front.
- Plainer wording, readable release notes, and better keyboard and screen-reader support.
- **On app 1.5.6:** **Check App Update → Update App** gets you all of this; **Updates → Check → Update** also works for the addons, which have the same files as 1.5.6.
- Relax Journey (`4.00.04`), the guide content (`4.00.08`) and Relax Quest Planner (`0.2.1`) are unchanged.

## What's new in v1.5.6

- Campfire: two new songs that take turns, and the campfire reaction can play again after 30 minutes instead of 1 hour.
- **On app 1.5.5 or older:** click **Check App Update → Update App** first, then **Updates → Check → Update**. Older apps refuse this addon update (it has new song files their safety list doesn't know) and leave your addons untouched.
- Relax Journey (`4.00.04`), the guide content (`4.00.08`) and Relax Quest Planner (`0.2.1`) are unchanged.

## What's new in v1.5.5

- **New look ("night campfire"):** the Windows app has a new dark theme with a "Do these next" panel first, an icon sidebar, a compact account bar and the report in columns on wide windows.
- **Update notice at launch:** when you open the app, it checks GitHub for a newer Relax Coach release and highlights **Updates** if one exists. It only reads the release information: nothing downloads or installs until you click **Update App**. If you're offline, it stays quiet. Start the app with `-no-update` to turn release checks off.
- **Application manifest:** `RelaxCoach.exe` now declares that it runs as the normal user (no administrator rights, Windows 10 or newer). That should mean fewer antivirus false alarms, but it's no guarantee: the app is still unsigned.
- **On app 1.5.0 to 1.5.4:** click **Check App Update → Update App** once to get this release. The launch notice only exists from 1.5.5 on, so older apps don't tell you about it themselves.
- Coach's addon only changes its version; Relax Journey (`4.00.04`), the guide content (`4.00.08`) and Relax Quest Planner (`0.2.1`) are unchanged.

## What's new in v1.5.4

- **Relax Quest Planner 0.2.1: Pick my guide works again with newer QuestieDB.** QuestieDB 1.0.16 (installed with Questie 12.8.0) lists a few quests that have no name, such as quests Questie marks as not in the game. Pick my guide then failed with "Guide scan failed: …Core/Engine.lua:70: attempt to index local 'r' (a nil value)", and the route panel showed it as a route issue. The planner now skips any quest it can't name.
- Update with **Updates → Check → Update** (on app 1.5.0 to 1.5.3, **Check App Update → Update App** first), then `/reload` in game.
- Coach's addon only changes its version; Relax Journey (`4.00.04`) and the guide content (`4.00.08`) are unchanged.

## What's new in v1.5.3

- **Farming page (Windows app):** when the selected account has AH Profit's save next to Coach's, a **Farming** page appears with five sections:
  - **Sell now:** your bank items at your auction house's lowest price after the cut, against the vendor price where it is known, the most valuable first.
  - **Gathering:** the nodes you and your guild picked, per map, with when they were last picked and by whom.
  - **Auctions:** what you have listed and what it brings after the cut, and sold/expired counts per item.
  - **Gold:** earned, spent and net for today, 7, 30 and 180 days, plus the last 30 days by source (vendor, repairs, auction house, mail, quests, loot…) and by character.
  - **Sessions:** your stopped farming sessions with zone, time, gold net and gold per hour.
- **Read-only:** the page reads AH Profit's SavedVariables and its bundled data files as data; it never runs them and never writes to them.
- **Gold and Sessions need AH Profit 0.6.0 or newer** (its gold ledger and session archive). With an older AH Profit those two sections say to update it; Sell now, Gathering and Auctions work with the AH Profit you have.
- **Getting AH Profit 0.6.0:** **Updates → More addons → AH Profit → Check → Install / Update** installs AH Profit's latest release. Then play once with it enabled and `/reload` (or log out) so it writes its save.
- Coach's addon only changes its version; Relax Journey (`4.00.04`), the guide content (`4.00.08`) and Relax Quest Planner (`0.2.0`) are unchanged.

## What's new in v1.5.2

- **Guide content 4.00.08:** 47 of the 201 guide files updated; the catalog now lists 125 guides (was 123). Relax Journey code stays `4.00.04`; the content's validity date is still **18 December 2026**.
- **A Journey that is not Relax Journey:** CurseForge, Wago/WowUp or Journey Guide Manager can overwrite Relax Journey. The app now says when the Journey folder is not Relax Journey (Updates and Get started) and offers **Repair**, which installs Relax Journey again and keeps the other copy in the backup; Repair needs a published Relax Coach release. If Journey is loaded, Coach mentions it once per session in chat. Remove Journey from those updaters or turn off its auto-update; if you keep the normal Journey on purpose, ignore the note. A Journey.toc the app cannot read is reported as unknown, with nothing offered.
- **WoW can stay open:** Install and Update (Get started, Updates and More addons) no longer ask you to close WoW first. Afterwards the app says **Type /reload in game** when only existing files changed, or **Exit WoW completely and start it again** when a new addon folder or file was added. If Windows refuses to replace a file that is in use, the app asks you to close WoW: usually the install is undone and nothing is changed; if Windows also blocks undoing it, the earlier folders stay safe in `RelaxCoachBackups` and **Repair** (or **Update**) finishes the job after WoW is closed. The ZIP and the PowerShell installer still need WoW closed.
- Relax Quest Planner (`0.2.0`) is unchanged.

## What's new in v1.5.1

- **First install from the app:** a new player needs only `RelaxCoach.exe`. The app opens on **Get started**: choose the game folder, close WoW, click **Install**. It installs Coach, Journey with its guide content and Relax Quest Planner through the same verified transaction as updates, and creates `Interface\AddOns` if it is missing. The bundle and **More addons** buttons now say **Install** or **Update** depending on what is installed. See *From the app* below.
- **Optional app copy:** when the app runs from somewhere else (for example Downloads), it offers to copy itself to `%USERPROFILE%\RelaxCoach` and add a **Relax Coach** desktop shortcut. It never replaces an existing copy or shortcut.
- **Repair at the same version:** if some Relax Coach folders are missing or an install was interrupted (the app closed or the PC lost power), **Check** offers **Install** again for the current release and finishes it; the folders that were there are kept in `RelaxCoachBackups`. A complete install is never reinstalled.
- **Interrupted installs no longer lock the updater:** a lock left by a crash mid-install is recovered on the next attempt after 15 minutes; folders that were moved aside are put back where they are missing, and nothing is deleted.
- **Higher spell ranks count as learned (Coach):** a trained higher rank no longer shows "not learned" for the rank-1 spell (for example Sunder Armor on a level 35 warrior), for every class. Confirmed in the Forever client.
- **More addons (Windows app):** **AH Profit** (`buiden/AHProfit`) joins RankUp and Forever Maps on the Updates page: vendor price, auction house price and AH profit on item tooltips. Shared and bundled prices are estimates until you scan your own auction house.
- Relax Quest Planner (`0.2.0`), Relax Journey (`4.00.04`) and the guide content (`4.00.06`) with its signed catalog are unchanged.

## What's new in v1.5.0

- **Level-up checklist (Coach):** after a level-up, a small window lists what to do now: an unspent talent point, up to two training or class-quest items, up to two gear upgrades and the next dungeon, with **Open Coach**, **Pick my guide** (when Relax Quest Planner is loaded) and **Got it**. It shows once per level, waits until combat ends, is on by default and can be controlled with `/coach levelup [show|on|off]`.
- **More addons (Windows app):** the Updates page can install and update two optional addons from their own GitHub repositories, **RankUp** (`buiden/RankUp`) and **Forever Maps** (`buiden/ForeverMaps`). See *More addons* under Updates for the checks it makes.
- **Relax Quest Planner 0.2.0 ships in the bundle:** plans dungeon quest chains and quest goals with their prerequisites, and picks a fitting Journey guide (`/rqp`, see `addon/RelaxQuestPlanner/README.md`). It never accepts, completes or turns in quests for you. **Update Addon** installs it as a fourth folder; an existing `RelaxQuestPlanner` folder (for example a test version you copied in) is moved into the backup, not deleted. Relax Journey gains a planner button on the guide header (left-click opens/closes the planner, right-click opens dungeon quests). The planner has passed the offline test suites only; it has not been validated in the native Forever client yet.
- **Review fixes:** Coach and planner fixes from independent code reviews, including secret-value checks before comparisons, fewer per-event and per-call allocations, tolerance for incomplete GearQuest data and best-item candidates without a name, and planner fixes for route refresh, Journey route synchronisation, skips, level limits and error reporting.
- The Journey guide content (`4.00.06`) and its signed catalog are unchanged.

## Install or upgrade

### From the app (app 1.5.1 and later)

A new player needs only `RelaxCoach.exe`: no ZIP to extract, no PowerShell.

1. Download `RelaxCoach.exe` from the release and run it. It needs Microsoft Edge WebView2 Runtime (current Windows includes it). The executable is unsigned: if security software flags it, verify the checksum in `SHA256SUMS.txt` or build from source; do not exclude your game folder.
2. The app opens on **Get started**. Step 1, **Install Relax Coach on this PC**: click **Install and add shortcuts**. The app copies itself to `%USERPROFILE%\RelaxCoach\RelaxCoach.exe`, adds **Relax Coach** shortcuts on the Desktop and in the Start menu, and reopens from there; you can then delete the downloaded file. It never replaces an existing installed copy: a second download offers **Open the installed Relax Coach** instead. If a shortcut is missing later, or points somewhere else, **Settings → Repair shortcuts** fixes it (it asks before replacing one). From app 1.6.0; older apps offered **Copy to my user folder** in a box at the bottom of the window.
3. Step 2: click **Choose folder…** and select your `_forever_` game folder (or paste it under **Game folder**).
4. Step 3: click **Install** (WoW can stay open). The app takes the stable GitHub release, verifies the bundle's SHA256 checksum and manifest, and installs **Relax Coach**, **Relax Journey** with its guide content and **Relax Quest Planner** in one transaction. A folder of the same name that is already there (for example a normal Journey) is moved into `RelaxCoachBackups`, not deleted; a failed install is undone where Windows allows it, and the earlier folders are never deleted. A missing `Interface\AddOns` folder is created. From app 1.5.2 WoW can stay open: afterwards the app says **Type /reload in game** when only existing files changed, or **Exit WoW completely and start it again** when a new addon folder or file was added. It asks you to close WoW only if Windows refuses to replace a file that is in use. Usually the install is then undone and nothing is changed; if Windows also blocks the way back, the earlier folders are kept in `RelaxCoachBackups` and the app says so: close WoW, then **Check** and **Repair** (or **Update**). Older apps require WoW to be closed.
5. Start WoW (if it was open while installing, exit it completely and start it again so the new addons load), enable **Relax Coach**, **Relax Journey** and **Relax Quest Planner** on the AddOns screen, log in and type `/reload` once. Use `/coach`, `/journey` and `/rqp`; your report appears under **Coach Report**.

**Using CurseForge, Wago/WowUp or Journey Guide Manager?** Remove Journey from its list or turn off its auto-update: it can overwrite Relax Journey. When the Journey folder is not Relax Journey, the app says so (Updates and Get started) and, if Journey is loaded, Coach mentions it once per session in chat. **Repair** installs Relax Journey again and keeps the other copy in `RelaxCoachBackups`; it needs a published Relax Coach release (Check must find one). If you keep the normal Journey on purpose, you can ignore the note.

Later releases: **Updates → Check → Update**. The optional **More addons** (RankUp, Forever Maps, AH Profit, LayerEnemies — Alpha) use the same **Check → Install / Update** buttons and the same checks. If some Relax Coach folders are missing, or an install was interrupted (the app closed or the PC lost power), **Check** offers **Install** again even when Coach is already on the latest version: it finishes the interrupted install, puts every folder in place through the same verified transaction and keeps the folders that were there in `RelaxCoachBackups`. A complete install is never reinstalled.

### Fallback: bundle ZIP and PowerShell

The ZIP and `Install-RelaxCoach.ps1` always need WoW closed. Use this with an app older than 1.5.1, without network access in the app, or to restore missing folders by hand. `Install-RelaxCoach.ps1` only works on a client that already has `Interface\AddOns` and `WTF` folders (it stops with "Not a game folder" otherwise), so on a completely fresh client, before the first game start, use the app's **Get started → Install** instead; it creates `Interface\AddOns` itself.

1. Close WoW and extract the bundle ZIP outside the game folder.
2. For an upgrade, back up the folders listed below outside the game, then remove only those old folders so obsolete files do not remain.
3. **Merge** the bundled `Interface` folder into `_forever_`. Never delete or replace your whole existing `Interface` folder.
4. Run `RelaxCoach.exe` from a writable folder. Select your game folder and account. The app remembers your explicit account choice and never switches automatically.
5. In WoW, enable **Relax Coach**, **Relax Journey** and **Relax Quest Planner** separately. Use `/coach` and `/journey`. Use `/reload` or logout before refreshing the desktop report.

**Optional automated installation:** the included PowerShell installer verifies files, creates backups and installs the app/desktop shortcut:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\Install-RelaxCoach.ps1 -Game "C:\Program Files (x86)\World of Warcraft\_forever_"
```

Use your actual game path. Manual copying does not perform the installer's checksum, backup or shortcut steps. If your game directory denies writes, run the optional installer as administrator; do not disable antivirus.

### Installing over normal Journey

This is an **in-place replacement**, not a second side-by-side Journey install. The app's **Install** / **Update** and the PowerShell installer back up and replace:

- `Interface\AddOns\ForeverCoach`
- `Interface\AddOns\Journey`
- `Interface\Content\Journey-Content`
- `Interface\AddOns\RelaxQuestPlanner` (bundles from 1.5.0; an existing planner folder, such as a manually installed test version, is backed up first)

It **does not overwrite or delete `WTF`**. The technical addon IDs, `ForeverCoachDB`, `Journey_Account` and `Journey_Character` remain unchanged so existing saves can be reused. Save preservation is verified; compatibility with every newer/older upstream Journey save format is not guaranteed.

Use the included `Restore-RelaxCoach.ps1 -Backup <backup-folder>` with WoW closed to restore the previous addon/content/app files after a PowerShell install. The install output records the backup folder. After an install from the app, the previous folders are in the `RelaxCoachBackups` folder the app names in its result. Avoid letting another addon manager overwrite `Journey` unless you intend to return to upstream Journey.

## Updates

- **Update everything** (app 1.6.0+): one button at the top of **Updates**. It updates the app first (Relax Coach restarts and carries on by itself), then the Relax Coach addons, then the **More addons** you already installed, each with its own checked, verified and backed-up transaction; it stops at the first failure and says what was done and what is left. It never installs anything you have not installed, and leaves a first install, a **Repair** or a Journey that is not Relax Journey to its own button. After an app update (also one started from an older app's **Update App**) the new app updates the addons by itself when WoW is closed and the addons only need a normal update; otherwise it highlights **Update everything** and says why (for example "WoW is open"). From 1.6.0 the app installs an addon release newer than itself only after updating itself, so the app always knows the files it installs.
- **Update App** (app v1.4.1+): check explicitly, then confirm the update. It verifies the executable checksum, restarts only Relax Coach, and supports backup/rollback. WoW can stay running; addons, guides and saves are untouched.
- **One-time bootstrap from an older app:** close only Relax Coach and replace its `RelaxCoach.exe` with the latest release asset. Subsequent app releases can use **Update App**.
- **Install / Update** for the Relax Coach addons (labelled **Check for updates → Update Addon** before app 1.5.1; app 1.8.0+ for current bundles): installs **Coach + Relax Journey + the bundled guide content together**. The button says **Install** while Coach is not installed in the selected game folder and **Update** afterwards; both use the same transaction. It verifies the release ZIP and manifest, stages the fixed directories, retains backups and undoes a failed transaction (if a file in use blocks that, the earlier folders stay in the backup and **Repair**/**Update** finishes it after WoW is closed). From **1.5.0** the bundle is named `RelaxCoach-Bundle-<version>.zip` and also installs **Relax Quest Planner** as a fourth folder; apps older than 1.5.0 report it as an app-only release, so use **Update App** first. From **1.8.0** it is named `RelaxCoach-RaidBundle-<version>.zip` and adds **Relax Raid** as a fifth folder; apps older than 1.8.0 report it as an app-only release, so use **Update App** first. From app 1.5.2 WoW can stay open: afterwards the app says **Type /reload in game** when only existing files changed, or **Exit WoW completely and start it again** when a new addon folder or file was added. It asks you to close WoW only if Windows refuses to replace a file that is in use. Usually the install is then undone and nothing is changed; if Windows also blocks the way back, the earlier folders are kept in `RelaxCoachBackups` and the app says so: close WoW, then **Check** and **Repair** (or **Update**). Older apps require WoW to be closed. Unrelated addons, `WTF`, settings and the running app are unchanged. The bundled EXE and scripts are not executed by this action.
- **More addons** (RankUp, Forever Maps, AH Profit, LayerEnemies — Alpha): optional addons from their own GitHub repositories (`buiden/RankUp`, `buiden/ForeverMaps`, `buiden/AHProfit`, `buiden/LayerEnemies`). Each has its own **Check** and **Install / Update** button on the Updates page, with the same wording as the Relax Coach addons. It installs only that addon's folder from its latest stable release, verifies the release `SHA256SUMS.txt`, checks the archive and TOC title/version, refuses reinstalls/downgrades and folders it can't identify, and backs up the previous version in `RelaxCoachBackups`. WoW can stay open (app 1.5.2+), with the same /reload or restart advice. A release must contain `<Folder>-<version>.zip` (one top-level `RankUp/`, `ForeverMaps/`, `AHProfit/` or `LayerEnemies/` folder) and `SHA256SUMS.txt`.
- **The addons remain independent:** updating them together does not make either one a required dependency of the other.
- **Future guide pushes:** publish a tested new guild release and increment its Coach/package version, even for guide-only changes. The updater consumes release assets, not raw Git pushes. Missing required assets are reported as unavailable—not as proof that your content is current.
- The app reads saves on disk. Use `/reload` or log out in WoW, then **Refresh**. Refresh cannot force WoW to write its saves.
- **Farming page (AH Profit):** the app reads ForeverCoach and AH Profit SavedVariables, and AH Profit's bundled data files (`FarmData.lua`, `History.lua`, `Data.lua` in its addon folder), all read-only: parsed as data, never executed, never written. When the selected account has both the Coach save (`ForeverCoach.lua`) and `AHProfit.lua`, a **Farming** page appears: bank items at your auction house's price after the cut (and against a vendor where the vendor price is known), gathering nodes per map, and your auctions' sold/expired counts. It uses the same account as the Coach report, never another one, and shows the auction house of your most recently saved character first (others are one choice away). A bank character's auction house is known only once Coach has saved that character; until then its items are listed without a price. Prices follow AH Profit's own order (your scan under 2 hours old, a newer guild scan, your older scan, then AH Profit's bundled prices), with guild prices more than 12 hours old or timed ahead of your clock ignored; at most 150 rows per auction house are listed. Saves over 16 MB are not read. Gathering node names are matched by AH Profit's own name hash (the save stores the hash, not the name), so a node missing from AH Profit's list could, rarely, show the name of a listed node with the same hash; names whose hashes collide within the list are not shown. With an AH Profit that writes saved-data version 3, the page also shows **Gold** (earned, spent and net for today, 7, 30 and 180 days, and the last 30 days by source and by character, all characters of the account; days are UTC days, as AH Profit counts them) and **Sessions** (the farming sessions you stopped, newest first: character, zone, time, coin net and per hour, nodes; at most 100), and uses the save's item names. With an older AH Profit the page says to update it; a save from a newer AH Profit still shows the other sections, with a note.
  Item names: AH Profit's save and bundled files first, then Coach's client cache, then installed GearQuest data. For history IDs still unnamed, the desktop app can fill gaps from Wowhead's Forever tooltip service; local names always win. Remaining unknowns show item IDs. Quality colours use the best-known quality by item ID, from Coach first, then GearQuest, and from Wowhead when it supplies a missing name; unknown quality keeps the default colour. Rows without an item ID cannot be matched to a quality. Coach fills missing history-item names at login and after the auction house closes, pauses in combat, and keeps at most 10,000 entries (no new entries when full); save with `/reload` or logout before refreshing the app. While names are missing, **X of Y item names known** counts distinct valid history item IDs for this account (Y) and those named by any source (X). If the account has no `ForeverCoachDB.itemNames` cache, it adds **(update Relax Coach and visit the auction house to name more)**; an existing cache, even empty or full, has no suffix. When history is empty or all its IDs are named, the line is hidden. This is name coverage, not a live queue or a promise that every item can resolve.
  **Online fallback (on by default):** sends item IDs only to Wowhead, not characters, accounts, prices or saves. It runs in the background (at most 200 requests per refresh, at most one per second), stops and backs off on server/network errors, and honours Retry-After. Only names and quality, plus fetch times or not-found markers, are cached on this PC in the app's configuration directory (`ForeverCoach/wowhead-names-16.json`); no tooltip text or stats, no bundled data and no cache uploads. Names expire after 30 days; not-found entries after 24 hours. Turn requests off under **Settings → Look up missing item names online (Wowhead)**; the choice is remembered and already-cached names can still display until they expire. Names with known item IDs in **Sell now**, **Up since yesterday** and **Down since yesterday** are links to Wowhead Forever, opened in your default browser rather than inside the app. Name-only auction statistics and hashed gathering kinds have no item IDs to link. No Wowhead scripts are embedded in the app.
- **Hot items (Farming page):** per auction house, separate **Up since yesterday** and **Down since yesterday** lists, each with up to 10 items, never padded. Items must move at least 20% since the previous recorded day and at least 20% against their usual price, ranked by the smaller move. Usual means the median of earlier recorded days in AH Profit's 21-day own-scan history: at least 5 earlier days, a usual price of at least 5 silver and usual listed supply of at least 5 units. Both lists require at least 10 units listed on the scan day and show **listed (scan day) / usual**. Risers also need at least half their usual supply; rejected thin-supply rises are counted separately, and the existing 10-times price cap remains. Fallers have no thin-supply filter or 10-times cap, but must cost at least 1 copper; **dump** marks at least twice the usual listed supply. Risers retain the **spike** marker for a price well above usual and above every earlier high. While no item in a house has enough earlier days, ranking uses only the previous-day change; supply medians use the earlier days available, no price spike is marked, and dump still follows supply. An item with no earlier day is not compared or counted as thin supply. Each item has a small chart and signed percentages, green for rises and red for falls, with names and quality colours from the same sources as bank items. The count is the scan day's highest listed units, not a live count or a count of sellers. Without a scan today both lists use the last scan day, labelled with its date. Guild-only members see a note until AH Profit keeps guild price history; unnamed items are shown by id.

## Death recap

Relax Raid adds a local **Death Recap**, available everywhere, including combat.
It reads the game's recorded damage hits (usually about five, at most twelve),
not a ten-second combat-log window; heals and duel losses have no recap. The
window shows readable damage, source, spell, time before death, health, overkill
and avoidable flags. Its hostile-player shares use **all retained damage for this
death** as the denominator; pets remain separate and missing identity is labelled unknown.
The guild avoidable list is generated from enabled raid mechanics (currently
empty); the game's own avoidable flag still works. Nothing is sent or scored.

Use `/recap`, `/recap 2`, `/recap 3`, the local **[You died: details]** chat link,
or **Detailed recap** beside the death popup. Blizzard's buttons are unchanged.
The three newest deaths have tabs. Close with X, Close or Escape; drag the title
to move the window and use `/recap reset` to reset its character-specific position.
Options are under **Options > AddOns > Relax Raid**, or `/recap auto on|off`,
`/recap autoclose on|off`, `/recap keep on|off`. Automatic opening and closing on
release/revive default on; keeping recaps after logout defaults off. Kept data
contains player names, stays local in the existing character settings, and is
capped at three deaths with twelve rows each. Even with keep off, one numeric
last-recap timestamp is always saved to reject earlier-life hits after reload.
No new SavedVariables are added.
If another addon carries this shared module, the highest version owns one window
and the saved history; both addons can register hooks. This change includes only
the Relax Raid hook, not LayerEnemies tracking. If the game supplies no data, the
window says so; missing popup callbacks leave `/recap` and the chat link working.
Rows at or below the newest previously recorded recap timestamp are never
assigned to a new death; mixed snapshots keep only newer rows. If none remain,
it shows **no recap data for this death** until newer rows arrive within two
seconds, even after a quick release (without reopening the window). If retry
timers are missing or fail, the recap is finished and saved immediately.
Unreadable hits or invalid damage blank the shares and show **some hits unreadable**. The attacker
block shows three sources plus **+X more** so the window fits a 600-pixel-tall
screen; all retained hits remain scrollable and all readable damage counts.
See [API evidence and offline validation](docs/raid/DEATH-RECAP.md).

## Guild-rank readings

Relax Raid also keeps each character's latest settled guild-rank reading, readable guild name and server time in the account's existing save. It refreshes on login, world entry and guild updates; a still-loading rank leaves the previous reading alone. These local readings support a **convenience lock, not security**: saves can be edited or become stale. This addon change supplies the readings only; the app's officer page and lock are a separate change.

## Coverage and known limits

- Optional installed providers: GearQuest Forever for comparable item scores, ForeverDungeonJournal for dungeon information. Unknown or incomparable gear is shown as unscored, not assigned an invented score.
- Included Journey code: `4.00.04`; Relax package/Coach version: `1.8.1`; guide-content version: `4.00.12`. The original signed catalog and guide bytes are retained. The content's existing validity date is **18 December 2026**; this release does not remove expiry checks.
- This is not a claim of complete coverage for every custom Forever class/spec/item or every leveling route. Some route entries are stubs.
- An intermittent blank native window occurred during development. The final local candidate passed ten restart/Refresh cycles and a more-than-three-minute minimize/restore test, but the earlier root cause was not conclusively identified. If the window is blank, close and reopen it and report the issue.

## Privacy and credits

Release bundles contain addon/program/guide files, **not player WTF saves, account credentials, local reports or private test evidence**. Reading and rendering your saves happens locally. Version checks and explicit updates contact GitHub.

Relax branding and Coach are maintained by **TheGamersVibe**. Coach is MIT-licensed (`LICENSE`). Journey and its guide content retain their original authorship, upstream credits and separate license; see [`addon/Journey/license.txt`](addon/Journey/license.txt) and [Journey](https://www.getjourney.gg/). The root MIT license does not relicense Journey or its content.

## Development

```sh
python3 scripts/build_rules_lua.py
uv run --with lupa python test/harness.py
(cd desktop && go test -race -timeout 20m ./... && go vet ./...)
# Windows build; Go must support the version specified by desktop/go.mod:
(cd desktop && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags '-H windowsgui' -o ../dist/RelaxCoach.exe .)
python3 scripts/build_guild_bundle.py --exe dist/RelaxCoach.exe
python3 test/test_guild_bundle.py
```

The packaging scripts build local artifacts only; they do not publish, push or tag.
