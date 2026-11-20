# Relax Coach

A leveling coach for **World of Warcraft: Forever**, with a Windows application for reports and addon updates.

This is a **legacy Coach-only archive**, not the current combined guild download. For Coach, Journey and guide content together, use `RelaxCoach-RaidBundle-1.8.2.zip` on [release v1.8.2](https://github.com/thegamersvibe/relax-coach/releases/tag/v1.8.2). A current release need not contain a Coach-only ZIP.

## What's new in v1.8.2 (combined bundle)

- **Relax Raid 0.3.0, Death Recap:** after any death, including battlegrounds, arenas and combat, a window shows the hits that killed you (the game keeps only the last few, usually about five, and no heals), who killed you in PvP, and the last three deaths as tabs. `/recap`, the **[You died: details]** chat link or **Detailed recap** under the death pop-up. Local only; nothing is sent or scored.
- **Guild rank saved** per character for a later officer-only page in the app: a convenience lock, not security.
- **Farming (app):** missing item names from Wowhead (item IDs only sent, cached on this PC, **Settings → Look up missing item names online** to turn off) and Wowhead links on item names.
- **Update on app 1.8.0 or 1.8.1:** **Updates → Update everything** (or **Update App** first), then restart WoW once for the new Relax Raid files.
- Coach and the app are **1.8.2**; Relax Raid **0.3.0**; Relax Journey (`4.00.04`), guide content (`4.00.12`) and Relax Quest Planner (`0.4.0`) are unchanged.
- **Not tested in game: alpha, owner's decision.**

## What's new in v1.8.1 (combined bundle)

- **Relax Raid 0.2.0, preparation reports:** in a raid group your Relax Raid sends everyone in your raid running Relax Raid a short report on your own character (class, level, consumables, better gear you own per Relax Coach, time played at your level, at level 60 only) when you join, at ready checks and after boss fights; hidden addon channel only, never chat, never in combat. Guild members' copies are kept (every guild member logs raids by default; the last 3 nights, within fixed size limits) and the officers' tools read them. It is information, not a score; there is no switch to stop sending it. The game's time-played lines may show in chat when you join a raid, and after a login or `/reload` while in one.
- **Hot items (app, Farming page):** item names and quality colours instead of numbers (update Relax Coach and visit the auction house to name more), a new **Down since yesterday** list, and at least 10 units listed with **listed / usual** shown.
- **Update on app 1.8.0:** **Updates → Update everything** (or **Update App** first), then restart WoW once for the new Relax Raid files.
- Coach and the app are **1.8.1**; Relax Raid **0.2.0**; Relax Journey (`4.00.04`), guide content (`4.00.12`) and Relax Quest Planner (`0.4.0`) are unchanged.
- **Not tested in game: alpha, owner's decision.**

## What's new in v1.8.0 (combined bundle)

- **Relax Raid 0.1.0, the raid logger:** turns the game's combat log and Advanced Combat Logging on when you enter a raid. Raids log for every guild member; dungeons only for officer ranks. **Don't delete your combat log** (several hundred MB a night); the guild needs it for raid reports. `/rraid status`, `/rraid logger on|off|default`.
- **App first:** apps before 1.8.0 need **Check App Update → Update App** first; the download is now `RelaxCoach-RaidBundle-1.8.0.zip`. Exit WoW and start it again after the update (new addon folder).
- **Guide content 4.00.12** (127 guides, adds Riverglades 43/44). **Alliance guides 30–60 stop on 18 December 2026** until a newer content update ships; Horde guides are not affected.
- **Relax Quest Planner 0.4.0:** dungeon chains tailored to each character; dungeon steps come last, split into **Before you go**, **Inside <dungeon>**, **After the run** and **Next run**.
- **Hot items (app, Farming page):** per auction house, up to 10 items clearly up since the previous recorded day (and, once there is enough history, against their usual price), from AH Profit's saved scans, with a small chart and a spike marker for real spikes.
- Coach and the app are **1.8.0**; Relax Journey (`4.00.04`) is unchanged.
- **Not tested in game: alpha, owner's decision.**

## What's new in v1.7.2 (combined bundle)

- **Gear advice works again on current GearQuest** (it reads GearQuest's new packed layout as well as the old one; a future format change is reported as such).
- **No battleground or honor rewards** in gear advice, and a suggestion must be worth the trip: about 1% of your set nearby, 3% for a dungeon, 5% for a zone more than 6 levels behind you.
- **Level 60:** no route wording, no vendor items without a known seller or place, and no raid loot until Forever opens raids (first raid 9 December).
- Coach and the app are **1.7.2**; Relax Journey (`4.00.04`), guide content (`4.00.08`) and Relax Quest Planner (`0.3.0`) are unchanged.
- **Not tested in game: alpha, owner's decision.** Offline tests do not validate native Forever behavior or the Windows update flow.

## What's new in v1.7.1 (combined bundle)

- **Relax Quest Planner 0.3.0 — Quest and item review:** open **Review** in the planner header or type `/rqp review`. It is read-only, shows **KEEP** / **UNCERTAIN-KEEP** reasons and source details, and never deletes items or abandons quests. Missing data is not permission to remove anything.
- **On app 1.7.0:** use **Updates → Update everything**, or **Check App Update → Update App** first and then **Updates → Check → Update**; app 1.7.0 updates itself before it installs the newer addons. On older apps, use **Check App Update → Update App** first as before. Follow the app's restart advice for the new planner files.
- Coach and the app are **1.7.1**; Relax Journey (`4.00.04`) and guide content (`4.00.08`) are unchanged. The flight sound already shipped in 1.7.0.
- **Not tested in game: alpha, owner's decision.** Offline tests do not validate native Forever behavior or the Windows update flow.

## Install

Download the latest available Coach ZIP from [the releases list](https://github.com/thegamersvibe/relax-coach/releases); app-only releases do not contain an addon ZIP. For the app, use `RelaxCoach.exe` from [the latest release](https://github.com/thegamersvibe/relax-coach/releases/latest). Older apps need this one-time executable replacement with Relax Coach closed; app v1.4.1+ has separate **Check App Update** and **Update App** controls. App updates can run while WoW stays open.

- Copy `ForeverCoach` into your game's `_forever_\Interface\AddOns` folder. It appears in game as **Relax Coach**. The internal folder/save IDs intentionally remain unchanged to retain existing settings.
- Run **RelaxCoach.exe** for the Windows app. Choose **Coach Report** or **Updates** inside the application window (app 1.5.1 and later also has **Get started**, which installs the current bundle from the app). Microsoft WebView2 Runtime is required; Windows installations commonly already include it.
- Set the game folder when prompted. Your game may be on any drive.
- **Using CurseForge, Wago/WowUp or Journey Guide Manager?** Remove Journey from its list or turn off its auto-update: it can overwrite Relax Journey. When the Journey folder is not Relax Journey, the app says so (Updates and Get started) and, if Journey is loaded, Coach mentions it once per session in chat. **Repair** installs Relax Journey again and keeps the other copy in `RelaxCoachBackups`; it needs a published Relax Coach release (Check must find one). If you keep the normal Journey on purpose, you can ignore the note.
- **On app 1.7.x:** use **Check App Update → Update App** first, then **Updates → Check → Update** (or **Updates → Update everything**) for the 1.8.0 raid bundle; apps before 1.8.0 report it as app-only until they have updated themselves, because it adds the Relax Raid folder. Apps older than 1.7.0 also need **Update App** first.
- App 1.4.1 supports **Update App**; older apps need one manual EXE replacement. From 1.8.0 the combined bundle is `RelaxCoach-RaidBundle-<version>.zip` and needs **app 1.8.0**; older apps report it as an app-only release until **Update App** has run (1.5.0 to 1.7.2 used `RelaxCoach-Bundle-<version>.zip`). The bundle installs and updates Coach, Journey, Relax Quest Planner, Relax Raid and guide content together, with checksum verification, backup and rollback. From app 1.5.2 WoW can stay open; the app then says whether `/reload` is enough or WoW must be restarted (new files), and asks you to close WoW only if a file is in use (usually nothing is changed then; if the way back is blocked as well, the earlier folders stay in `RelaxCoachBackups` and **Repair**/**Update** finishes it after WoW is closed). Older apps and the PowerShell installer need WoW closed. Unrelated addons, SavedVariables and the running app are not replaced by this action. From app 1.6.0, **Update everything** on the Updates page does the app, the bundle and the installed More addons in that order with one click (the app first, so it always knows the files it installs), and after an app update the new app updates the addons by itself when WoW is closed.

The Windows executable is unsigned. Follow your usual antivirus review process; do not disable protection globally. The addon works without the Windows app.

## In game

- `/coach` — open/close the advice window.
- `/coach top` — show the top three findings.
- `/coach spec <id|auto>` — select your intended specialization.
- `/coach macros` — create/update matching `FC ` macro templates for your class, out of combat. Ctrl is the primary modifier. Matching template names may be overwritten: rename customized macros first. Drag new macros to your own action bars.

Coach provides gear, build, training, class-quest and leveling advice plus session estimates. Current talent data contains 28 standard plans; specialty/hybrid gaps remain. Usability and movement-derived metrics are estimates, not a combat log.

## Report

Use `/reload` or logout in WoW to write current data, then **Refresh** in the app. Refresh rereads the save; it cannot force WoW to save. Coach's `ForeverCoach.lua` and AH Profit's `AHProfit.lua` SavedVariables, AH Profit's bundled data files, and optional installed GearQuest generated name/quality tables are parsed (read-only, never executed); AH Profit's data appears on the **Farming** page when the account has both saves.

Legacy folder discovery settings remain supported (`FOREVER_COACH_GAME` and `%AppData%\ForeverCoach\config.json`).

## Optional integrations

- **GearQuest Forever** and the module for your class: gear-source checks.
- **Journey / Relax Journey**: current guide/step context.
- **ForeverDungeonJournal**: dungeon preparation and quest context.

None is required for Coach to load. The smaller **Coach-only archive** contains only our Coach addon and application. It does not include Journey source, guides, or local Journey modifications, but a current app's explicit combined addon update can install them. The combined guild archive separately includes Relax Journey and guide content.

## Development and releases

The technical source paths remain `addon/ForeverCoach` and `desktop`.

- `python3 scripts/build_rules_lua.py`
- `uv run --with lupa python test/harness.py` (also run with provider fixtures)
- `cd desktop && go vet ./... && go test -race -timeout 20m ./...`

The legacy Coach-only asset is `RelaxCoach-<version>.zip`; it is deliberately not published for 1.4.2 and later. Current app addon updates require `RelaxCoach-RaidBundle-<version>.zip` (1.8.0 and later; it adds Relax Raid, so apps before 1.8.0 must use Update App first). Releases 1.5.0 to 1.7.2 used `RelaxCoach-Bundle-<version>.zip`. Releases 1.4.2 to 1.4.x used `RelaxCoach-Guild-<version>.zip`. App-only releases provide just `RelaxCoach.exe` and `SHA256SUMS.txt`; addon and combined guild ZIPs are included only when those packages are being released. The addon updater consumes published stable releases, not raw pushes to `main`; publishing a tested new release makes it available to players. Keep Journey and guide content out of the smaller Coach-only archive. Never include personal SavedVariables in either archive or this repository.

MIT-licensed Coach code by **TheGamersVibe**. Original upstream projects retain their own authorship and licenses.
