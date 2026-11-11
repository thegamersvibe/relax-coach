# Relax Coach — guild bundle

Three **separate WoW Forever addons**, plus a native Windows report app:

- **Relax Coach** — leveling, equipment, talent, dungeon and session advice. Works without Journey.
- **Relax Journey** — the rebranded Journey guide UI and the included guide-content pack. Can be enabled independently of Coach.
- **Relax Quest Planner** (from v1.5.0) — dungeon quest chains, quest prerequisites and a guide picker (`/rqp`). Works without Coach; Journey is optional.
- **RelaxCoach.exe** — a real Windows app window with account selection, saved-character reports and explicit app and addon/content updates.

## Download

**[Download release v1.5.1](https://github.com/thegamersvibe/relax-coach/releases/tag/v1.5.1)**:

- `RelaxCoach-Bundle-1.5.1.zip` — the three independent addons (Coach, Journey, Relax Quest Planner 0.2.0), Journey guide content **4.00.06**, the Windows app and optional installer/restore scripts. From v1.5.0 the combined download has this new name; earlier releases used `RelaxCoach-Guild-<version>.zip`.
- `RelaxCoach.exe` — app-only download, for updating the application separately.
- `SHA256SUMS.txt` — checksums for these release assets.

**On app 1.5.0?** Use **Check App Update → Update App** first (Get started and the repair below come with app 1.5.1), then **Updates → Check → Update**.

**On app 1.4.x?** Use **Check App Update → Update App** first, then close WoW and use **Check for updates → Update Addon**. Apps before 1.5.0 cannot install the bundle (from 1.5.0) and report it as an app-only release until the app is updated. App 1.4.0 needs one manual EXE replacement with Relax Coach closed before it can update itself.

There is deliberately no Coach-only `RelaxCoach-1.5.1.zip`: older apps must not report a completed addon update while leaving the guide content behind. The previous app-only v1.4.1 release remains unchanged.

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
2. Optional: when the app runs from somewhere else (for example Downloads), it offers to copy itself to `%USERPROFILE%\RelaxCoach\RelaxCoach.exe` and add a **Relax Coach** desktop shortcut. Click **Copy to my user folder**, or **Not now** to keep using it where it is. It never replaces an existing installed copy or shortcut.
3. The app opens on **Get started**. Click **Choose folder…** and select your `_forever_` game folder (or paste it under **Game folder**).
4. Click **Install** (WoW can stay open). The app takes the stable GitHub release, verifies the bundle's SHA256 checksum and manifest, and installs **Relax Coach**, **Relax Journey** with its guide content and **Relax Quest Planner** in one transaction. A folder of the same name that is already there (for example a normal Journey) is moved into `RelaxCoachBackups`, not deleted; a failed install rolls back. A missing `Interface\AddOns` folder is created. From app 1.5.2 WoW can stay open: afterwards the app says **Type /reload in game** when only existing files changed, or **Exit WoW completely and start it again** when a new addon folder or file was added (WoW only finds new files at start). It asks you to close WoW only if Windows refuses to replace a file that is in use; the install is then rolled back and nothing is changed. Older apps require WoW to be closed.
5. Start WoW (exit it completely first if it was open: new addons only load at game start), enable **Relax Coach**, **Relax Journey** and **Relax Quest Planner** on the AddOns screen, log in and type `/reload` once. Use `/coach`, `/journey` and `/rqp`; your report appears under **Coach Report**.

Later releases: **Updates → Check → Update**. The optional **More addons** (RankUp, Forever Maps, AH Profit) use the same **Check → Install / Update** buttons and the same checks. If some Relax Coach folders are missing, or an install was interrupted (the app closed or the PC lost power), **Check** offers **Install** again even when Coach is already on the latest version: it finishes the interrupted install, puts every folder in place through the same verified transaction and keeps the folders that were there in `RelaxCoachBackups`. A complete install is never reinstalled.

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

- **Update App** (app v1.4.1+): check explicitly, then confirm the update. It verifies the executable checksum, restarts only Relax Coach, and supports backup/rollback. WoW can stay running; addons, guides and saves are untouched.
- **One-time bootstrap from an older app:** close only Relax Coach and replace its `RelaxCoach.exe` with the latest release asset. Subsequent app releases can use **Update App**.
- **Install / Update** for the Relax Coach addons (labelled **Check for updates → Update Addon** before app 1.5.1; app 1.5.0+ for current bundles): installs **Coach + Relax Journey + the bundled guide content together**. The button says **Install** while Coach is not installed in the selected game folder and **Update** afterwards; both use the same transaction. It verifies the release ZIP and manifest, stages the fixed directories, retains backups and rolls back a failed transaction. From **1.5.0** the bundle is named `RelaxCoach-Bundle-<version>.zip` and also installs **Relax Quest Planner** as a fourth folder; apps older than 1.5.0 report it as an app-only release, so use **Update App** first. From app 1.5.2 WoW can stay open: afterwards the app says **Type /reload in game** when only existing files changed, or **Exit WoW completely and start it again** when a new addon folder or file was added (WoW only finds new files at start). It asks you to close WoW only if Windows refuses to replace a file that is in use; the install is then rolled back and nothing is changed. Older apps require WoW to be closed. Unrelated addons, `WTF`, settings and the running app are unchanged. The bundled EXE and scripts are not executed by this action.
- **More addons** (RankUp, Forever Maps, AH Profit): optional addons from their own GitHub repositories (`buiden/RankUp`, `buiden/ForeverMaps`, `buiden/AHProfit`). Each has its own **Check** and **Install / Update** button on the Updates page, with the same wording as the Relax Coach addons. It installs only that addon's folder from its latest stable release, verifies the release `SHA256SUMS.txt`, checks the archive and TOC title/version, refuses reinstalls/downgrades and folders it can't identify, and backs up the previous version in `RelaxCoachBackups`. WoW can stay open (app 1.5.2+), with the same /reload or restart advice. A release must contain `<Folder>-<version>.zip` (one top-level `RankUp/`, `ForeverMaps/` or `AHProfit/` folder) and `SHA256SUMS.txt`.
- **The addons remain independent:** updating them together does not make either one a required dependency of the other.
- **Future guide pushes:** publish a tested new guild release and increment its Coach/package version, even for guide-only changes. The updater consumes release assets, not raw Git pushes. Missing required assets are reported as unavailable—not as proof that your content is current.
- The app reads saves on disk. Use `/reload` or log out in WoW, then **Refresh**. Refresh cannot force WoW to write its saves.

## Coverage and known limits

- Optional installed providers: GearQuest Forever for comparable item scores, ForeverDungeonJournal for dungeon information. Unknown or incomparable gear is shown as unscored, not assigned an invented score.
- Included Journey code: `4.00.04`; Relax package/Coach version: `1.5.1`; guide-content version: `4.00.06`. The original signed catalog and guide bytes are retained. The content's existing validity date is **18 December 2026**; this release does not remove expiry checks.
- This is not a claim of complete coverage for every custom Forever class/spec/item or every leveling route. Some route entries are stubs.
- An intermittent blank native window occurred during development. The final local candidate passed ten restart/Refresh cycles and a more-than-three-minute minimize/restore test, but the earlier root cause was not conclusively identified. If the window is blank, close and reopen it and report the issue.

## Privacy and credits

Release bundles contain addon/program/guide files, **not player WTF saves, account credentials, local reports or private test evidence**. Reading and rendering your saves happens locally. Version checks and explicit updates contact GitHub.

Relax branding and Coach are maintained by **TheGamersVibe**. Coach is MIT-licensed (`LICENSE`). Journey and its guide content retain their original authorship, upstream credits and separate license; see [`addon/Journey/license.txt`](addon/Journey/license.txt) and [Journey](https://www.getjourney.gg/). The root MIT license does not relicense Journey or its content.

## Development

```sh
python3 scripts/build_rules_lua.py
uv run --with lupa python test/harness.py
(cd desktop && go test -race ./... && go vet ./...)
# Windows build; Go must support the version specified by desktop/go.mod:
(cd desktop && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags '-H windowsgui' -o ../dist/RelaxCoach.exe .)
python3 scripts/build_guild_bundle.py --exe dist/RelaxCoach.exe
python3 test/test_guild_bundle.py
```

The packaging scripts build local artifacts only; they do not publish, push or tag.
