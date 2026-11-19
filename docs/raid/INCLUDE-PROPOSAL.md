# Include my addon: `RelaxRaid`

The proposal `AGENTS.md` requires before a new addon root is bundled. Headings follow the
"Include my addon" list in `CONTRIBUTING.md` and the pull request template. This document
proposes; it does not bundle anything. R0 adds no addon folder.

## Scope

- Contribution lane: Include my addon.
- What is proposed: a new, optional addon folder `addon/RelaxRaid/`, installed as
  `Interface/AddOns/RelaxRaid`, for the Relax Raid tools (combat logging switch, preparation
  reports, later soft reserve). Design: `docs/raid/REQUIREMENTS.md`; build plan slices R1, R2,
  R9a, R9b.
- Out of scope here: the addon's code (R1 onward), the updater change (R9a) and the bundle
  change (R9b).
- R1 adds the folder to the repository only. The bundle builder does not include it until R9b;
  `test/raid/core` checks that.

## Source, revision, license and redistribution

- Source: this repository, written for Relax Coach; not vendored from another addon.
- License: the repository's MIT license. Any block later copied from another addon carries a
  provenance header and a `THIRD_PARTY.md` row (REQ-DST-03); none exists today.

## Maintainer, folder, dependencies and saved variables

- Maintainer: the Relax Coach custodians (`Tareksoh`, `buiden`).
- Folder: `RelaxRaid` (`Interface/AddOns/RelaxRaid`).
- Dependencies: none. It does not require Coach, Journey or the Planner, and they do not
  require it.
- Saved variables: `RelaxRaidDB` (account: the logger's collected nights) and
  `RelaxRaidCharDB` (per character: the logger setting).

## Forever client and evidence

- Target: WoW Forever (Interface 16001). Every game API is confirmed in the client listing
  before use; every handler and timer runs under `pcall`.
- Tested on a native client: **not run (alpha, owner's decision)**. Offline tests run the Lua
  in the harness under `test/raid/` with the mock client `test/raid/_client/client.py`.

## Default behaviour, triggers, communication, data sharing, performance and collisions

- Optional: installed only with the bundle that carries it (R9b); nothing changes for players
  who do not use it.
- Communication: one addon message prefix, `RLXR1`, inside the raid group; the first payload
  byte is the message type, the second its version. Sent only out of combat (joining the raid,
  ready check, after an encounter) and queued otherwise; messages dropped by the client's
  addon restriction are counted, never retried in a loop.
- Data shared: a preparation report per character (DESIGN.md 10.1) to the raid's logger. Raw
  combat logs never leave the PC.
- Performance: the combat log switch is the client's own; parsing happens in the desktop app,
  not in the game.
- Collisions: none known; the folder, prefix and saved-variable names are new.

## Install, update, uninstall, rollback and old/new versions

- The app must ship first. Today's app refuses an archive with an unknown root
  (`desktop/addon_update_guild.go:117`, "archive path is outside the fixed guild allowlist").
  R9a teaches the app the optional `RelaxRaid` root, its state, uninstall and rollback, and is
  released before any bundle carries the folder. R9b's bundle builder refuses to include the
  folder unless that app floor is met.
- Update: through the existing bundle path, with checksum verification, backup and rollback,
  like the other roots.
- Uninstall removes the `RelaxRaid` folder and keeps `RelaxRaidDB` and `RelaxRaidCharDB`.
- Old bundle on a new app, and new bundle on an old app: both covered by R9a's tests. An old
  app keeps refusing a bundle with the new root, without changing any installed files.

## Verification

- Native Forever client and Windows companion: **not run (alpha, owner's decision)**.
- CI runs the Relax Raid modules through `tools/raidtest.py`; CI is not native game validation.
