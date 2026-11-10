# Relax Coach

A leveling coach for **World of Warcraft: Forever**, with a Windows application for reports and addon updates.

This is a **legacy Coach-only archive**, not the current combined guild download. For Coach, Journey and guide content together, use `RelaxCoach-Bundle-1.5.0.zip` on [release v1.5.0](https://github.com/thegamersvibe/relax-coach/releases/tag/v1.5.0). A current release need not contain a Coach-only ZIP.

## Install

Download the latest available Coach ZIP from [the releases list](https://github.com/thegamersvibe/relax-coach/releases); app-only releases do not contain an addon ZIP. For the app, use `RelaxCoach.exe` from [the latest release](https://github.com/thegamersvibe/relax-coach/releases/latest). Older apps need this one-time executable replacement with Relax Coach closed; app v1.4.1+ has separate **Check App Update** and **Update App** controls. App updates can run while WoW stays open.

- Copy `ForeverCoach` into your game's `_forever_\Interface\AddOns` folder. It appears in game as **Relax Coach**. The internal folder/save IDs intentionally remain unchanged to retain existing settings.
- Run **RelaxCoach.exe** for the Windows app. Choose **Coach Report** or **Update Addon** inside the application window. Microsoft WebView2 Runtime is required; Windows installations commonly already include it.
- Set the game folder when prompted. Your game may be on any drive.
- **Upgrade the app first.** App 1.4.1 supports **Update App**; older apps need one manual EXE replacement. Current bundles require **app 1.4.3** before **Update Addon**. The combined guild package is used to update Coach, Journey and guide content, with checksum verification, backup and rollback. Close WoW first. Unrelated addons, SavedVariables and the running app are not replaced by this action.

The Windows executable is unsigned. Follow your usual antivirus review process; do not disable protection globally. The addon works without the Windows app.

## In game

- `/coach` — open/close the advice window.
- `/coach top` — show the top three findings.
- `/coach spec <id|auto>` — select your intended specialization.
- `/coach macros` — create/update matching `FC ` macro templates for your class, out of combat. Ctrl is the primary modifier. Matching template names may be overwritten: rename customized macros first. Drag new macros to your own action bars.

Coach provides gear, build, training, class-quest and leveling advice plus session estimates. Current talent data contains 28 standard plans; specialty/hybrid gaps remain. Usability and movement-derived metrics are estimates, not a combat log.

## Report

Use `/reload` or logout in WoW to write current data, then **Refresh** in the app. Refresh rereads the save; it cannot force WoW to save. Only Coach's `ForeverCoach.lua` SavedVariables are parsed, never executed.

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
- `cd desktop && go vet ./... && go test -race ./...`

The legacy Coach-only asset is `RelaxCoach-<version>.zip`; it is deliberately not published for 1.4.2 and later. Current app addon updates require `RelaxCoach-Bundle-<version>.zip` (1.5.0 and later; it adds Relax Quest Planner, so apps before 1.5.0 must use Update App first). Releases 1.4.2 to 1.4.x used `RelaxCoach-Guild-<version>.zip`. App-only releases provide just `RelaxCoach.exe` and `SHA256SUMS.txt`; addon and combined guild ZIPs are included only when those packages are being released. The addon updater consumes published stable releases, not raw pushes to `main`; publishing a tested new release makes it available to players. Keep Journey and guide content out of the smaller Coach-only archive. Never include personal SavedVariables in either archive or this repository.

MIT-licensed Coach code by **TheGamersVibe**. Original upstream projects retain their own authorship and licenses.
