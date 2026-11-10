# Relax Coach — guild bundle

Two **separate WoW Forever addons**, plus a native Windows report app:

- **Relax Coach** — leveling, equipment, talent, dungeon and session advice. Works without Journey.
- **Relax Journey** — the rebranded Journey guide UI and the included guide-content pack. Can be enabled independently of Coach.
- **RelaxCoach.exe** — a real Windows app window with account selection, saved-character reports and explicit app and addon/content updates.

## Download

**[Download release v1.4.5](https://github.com/thegamersvibe/relax-coach/releases/tag/v1.4.5)**:

- `RelaxCoach-Guild-1.4.5.zip` — both independent addons, Journey guide content **4.00.06**, the Windows app and optional installer/restore scripts.
- `RelaxCoach.exe` — app-only download, for updating the application separately.
- `SHA256SUMS.txt` — checksums for these release assets.

**Already on app 1.4.1 or 1.4.2?** Use **Check App Update → Update App** first. After the app restarts as **1.4.5**, close WoW and use **Check for updates → Update Addon** to install Coach, Journey and the guides. Current bundles require app **1.4.3**. App 1.4.0 needs one manual EXE replacement with Relax Coach closed before it can update itself.

There is deliberately no Coach-only `RelaxCoach-1.4.5.zip`: older apps must not report a completed addon update while leaving the guide content behind. The previous app-only v1.4.1 release remains unchanged.

## Install or upgrade the guild bundle

1. Close WoW and extract the guild ZIP outside the game folder.
2. For an upgrade, back up the three folders listed below outside the game, then remove only those old folders so obsolete files do not remain.
3. **Merge** the bundled `Interface` folder into `_forever_`. Never delete or replace your whole existing `Interface` folder.
4. Run `RelaxCoach.exe` from a writable folder. Select your game folder and account. The app remembers your explicit account choice and never switches automatically.
5. In WoW, enable **Relax Coach** and **Relax Journey** separately. Use `/coach` and `/journey`. Use `/reload` or logout before refreshing the desktop report.

**Optional automated installation:** the included PowerShell installer verifies files, creates backups and installs the app/desktop shortcut:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\Install-RelaxCoach.ps1 -Game "C:\Program Files (x86)\World of Warcraft\_forever_"
```

Use your actual game path. Manual copying does not perform the installer's checksum, backup or shortcut steps. If your game directory denies writes, run the optional installer as administrator; do not disable antivirus.

The app needs Microsoft Edge WebView2 Runtime. The executable is unsigned. If security software flags it, verify the download/checksum or build from source; do not exclude your entire game folder.

### Installing over normal Journey

This is an **in-place replacement**, not a second side-by-side Journey install. The installer backs up and replaces:

- `Interface\AddOns\ForeverCoach`
- `Interface\AddOns\Journey`
- `Interface\Content\Journey-Content`
- `Interface\AddOns\RelaxQuestPlanner` (bundles from 1.5.0; an existing planner folder, such as a manually installed test version, is backed up first)

It **does not overwrite or delete `WTF`**. The technical addon IDs, `ForeverCoachDB`, `Journey_Account` and `Journey_Character` remain unchanged so existing saves can be reused. Save preservation is verified; compatibility with every newer/older upstream Journey save format is not guaranteed.

Use the included `Restore-RelaxCoach.ps1 -Backup <backup-folder>` with WoW closed to restore the previous addon/content/app files. The install output records the backup folder. Avoid letting another addon manager overwrite `Journey` unless you intend to return to upstream Journey.

## Updates

- **Update App** (app v1.4.1+): check explicitly, then confirm the update. It verifies the executable checksum, restarts only Relax Coach, and supports backup/rollback. WoW can stay running; addons, guides and saves are untouched.
- **One-time bootstrap from an older app:** close only Relax Coach and replace its `RelaxCoach.exe` with the latest release asset. Subsequent app releases can use **Update App**.
- **Update Addon** (app v1.4.3+ for current bundles): installs **Coach + Relax Journey + the bundled guide content together**. It verifies the release ZIP and manifest, stages all three fixed directories, retains backups and rolls back a failed transaction. From **1.5.0** the bundle is named `RelaxCoach-Bundle-<version>.zip` and also installs **Relax Quest Planner** as a fourth folder; apps older than 1.5.0 report it as an app-only release, so use **Update App** first. Close WoW first. Unrelated addons, `WTF`, settings and the running app are unchanged. The bundled EXE and scripts are not executed by this action.
- **More addons** (RankUp, Forever Maps): optional addons from their own GitHub repositories (`buiden/RankUp`, `buiden/ForeverMaps`). Each has its own **Check** and **Install/Update** button on the Updates page. It installs only that addon's folder from its latest stable release, verifies the release `SHA256SUMS.txt`, checks the archive and TOC title/version, refuses reinstalls/downgrades and folders it can't identify, and backs up the previous version in `RelaxCoachBackups`. Close WoW first. A release must contain `<Folder>-<version>.zip` (one top-level `RankUp/` or `ForeverMaps/` folder) and `SHA256SUMS.txt`.
- **The addons remain independent:** updating them together does not make either one a required dependency of the other.
- **Future guide pushes:** publish a tested new guild release and increment its Coach/package version, even for guide-only changes. The updater consumes release assets, not raw Git pushes. Missing required assets are reported as unavailable—not as proof that your content is current.
- The app reads saves on disk. Use `/reload` or log out in WoW, then **Refresh**. Refresh cannot force WoW to write its saves.

## Coverage and known limits

- Optional installed providers: GearQuest Forever for comparable item scores, ForeverDungeonJournal for dungeon information. Unknown or incomparable gear is shown as unscored, not assigned an invented score.
- Included Journey code: `4.00.04`; Relax package/Coach version: `1.4.5`; guide-content version: `4.00.06`. The original signed catalog and guide bytes are retained. The content's existing validity date is **18 December 2026**; this release does not remove expiry checks.
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
