# App updates

The native Windows **Updates** page has two independent capabilities:

- **Check App Update → Update App** checks the running app's version, then explicitly downloads, verifies, backs up and restarts Relax Coach. WoW may stay open. No game folder, account selection or Git installation is required.
- **Check → Install / Update** (the Coach, Journey, Relax Quest Planner and guide content card, and the **Get started** page) installs or updates the Relax Coach bundle folders in the selected game folder in one verified transaction. It says **Install** while Coach is not installed there and **Update** afterwards. WoW can stay open (from app 1.5.2): the result says **Type /reload in game** when every folder existed and no new file path came, or **Exit WoW completely and start it again** when a folder or file path is new (decided from the old and new file lists, ignoring case). If Windows refuses a move because a file is in use, the transaction is rolled back and the app asks to close WoW and try again; that is the only case that needs WoW closed. If Windows also refuses a move back, the rollback cannot finish: the earlier folders stay in the transaction under `RelaxCoachBackups` (never deleted), the lock is kept, and the message starts "Close WoW, then click Check and Repair (or Update)" with the short backup location. In the same app session that lock is recovered at once (moved-aside folders go back where nothing is in their place), so Repair or Update completes after WoW is closed; after restarting the app, the usual 15-minute stale-lock rule applies. More addons work the same way. Before app 1.5.1 these buttons were labelled **Check for updates → Update Addon**.

Checking does not install anything. The app explains the restart before the Update App click. A failed check or installation requires a fresh check. `-no-update` disables both update capabilities. Closing the window normally cancels work before handoff; do not force-terminate an update once replacement has started.

## First run: copying the app

When `RelaxCoach.exe` runs from anywhere other than `%USERPROFILE%\RelaxCoach\RelaxCoach.exe` (the folder `Install-RelaxCoach.ps1` uses), the app offers, on the player's click only, to copy itself there and add a **Relax Coach** desktop shortcut. The copy is written to a new temporary file in that folder (exclusive create), read back and compared by SHA256, and only then moved to `RelaxCoach.exe` in one step that never replaces an existing file; a crash leaves at most the temporary file. The folder must be a plain directory, not a link. An existing installed copy or shortcut is never replaced: the app then points at the installed copy instead. The shortcut is written with the Windows Shell Link COM object (`IShellLinkW` + `IPersistFile`) on the UI thread; no PowerShell, shell or second process runs. If the shortcut cannot be written, the verified copy stays and the message says so. The running app is not relaunched; the player starts the copy from the shortcut next time and may delete the downloaded file. The two RPC methods, `appPlacement()` and `installAppCopy()`, take no arguments; paths come from the running executable, the user profile and the Desktop known folder, never from JavaScript.

## First manual installation

An older app without these controls cannot update itself. Download `RelaxCoach.exe` and `SHA256SUMS.txt` from the project's official GitHub release, verify the executable's exact SHA256 entry, close Relax Coach, retain a copy of the old executable, and replace it manually once. Thereafter use the native controls. The presence of this documentation does not mean a new public release has been published.

Keep the app in a folder writable by your Windows user. The updater does not request elevation or install a service. If permissions or another running Relax Coach instance prevent replacement, it fails safely instead of changing permissions or terminating unrelated processes.

## Trust boundary

Only the stable, non-draft latest release at
`https://api.github.com/repos/thegamersvibe/relax-coach/releases/latest`
is considered. The asset names must be exactly `RelaxCoach.exe` and `SHA256SUMS.txt`, from that same release in `thegamersvibe/relax-coach`. Equal/older versions, prereleases, ambiguous assets and unsupported versions cannot be installed. Release freshness is checked again when applying.

Downloads use HTTPS, the existing exact GitHub/CDN redirect allowlist, bounded requests, a 1 MiB release document, 64 KiB checksum file and 128 MiB executable maximum. The exact checksum entry must match; the payload must parse as a Windows amd64 PE32+ GUI executable, not a DLL. Checksums from the same HTTPS release channel provide integrity checking, **not an independent publisher signature**. This app is not represented as code-signed.

JavaScript receives display-only offers. Download URLs and checked releases stay in Go. The only app-update RPC methods are `checkApp()` and `applyApp(expectedVersion)`, plus the parameterless native startup acknowledgement `appReady()`. Existing per-window token, document-source, navigation, external-link and CSP controls remain in force. No arbitrary path, command, executable or URL execution RPC is added.

## Replacement and recovery

The updater captures the original executable hash when the native app starts. It stages an exact verified copy of that original as its local worker, **not** the downloaded candidate. Its adjacent private layout is:

```text
RelaxCoach.exe
.RelaxCoach.exe.updates/
  active.lock
  last-result.json
  <32 lowercase hex transaction ID>/
    transaction.json
    worker.exe
    candidate.exe       # becomes the installed target
    backup.exe          # retained original; never deleted by cleanup
    backed-up.json
    installed.json
    committed.json      # or rolled-back.json / failed.json / recovery.json
    failed.exe          # unhealthy candidate, if rollback was needed
```

The root name follows the installed executable's actual basename. The transaction record is immutable and bounded to 16 KiB. Paths are derived from the current executable and the fixed layout, not from a network manifest. An exclusive lock refuses concurrent updates. Symlinks, junctions/reparse paths, hard-linked files and changed hashes are refused. The Windows staging DACL is private to the current user. No recursive cleanup is used: backup, journal and worker recovery material are deliberately retained for inspection.

The worker authenticates the parent over an owner-only local named pipe, acquires a handle to that actual parent process, and verifies its image path and creation time. A transaction-fingerprint handshake completes **before** the old UI requests normal closure on its UI thread. The worker waits for the held parent handle to signal exit; it does not kill the old app by PID.

Before replacement the old executable is moved to a verified backup. Every transition checks the immutable transaction, ownership lock, relevant hashes and path types again. The candidate is launched directly with supported GUI arguments preserved (`-game` and `-no-update`; no diagnostic modes or arbitrary command line). A new native shell must finish rendering a successful Go report RPC, then acknowledge from the embedded JavaScript after rendered frames. Health is bound to the transaction, executable hash, compiled version and exact newly launched child process, not a marker file or a claimed PID alone.

Startup has a 45-second health deadline. The worker's initial handoff has a 15-second deadline and parent exit has a 60-second deadline. On startup failure, it stops only its owned child handle, retains the failed candidate, copies the verified backup back to the target, and relaunches the old app. Failure to publish the final success result also takes this rollback path, even after healthy startup; a failed publication removes an owned commit marker when identity verification and filesystem cleanup succeed. If either fails, the error is retained and uncertain files are left for inspection. Once restoration is verified, a journal or last-result reporting error cannot prevent the old-app relaunch attempt or release of the owned recovery lock. Reporting and relaunch errors remain in the worker error; if reporting storage is unavailable, inspect the retained files rather than relying on the Updates page.

If the first backup move fails, the worker rechecks the original target, candidate, worker, transaction and the same ownership lock before relaunching the unchanged old app. It records a terminal failure and releases that lock only while the initial state remains verified, allowing a fresh attempt. Failed lock creation writes and failed result-publication temporary files are cleaned up only when their created file identity still matches; existing or replaced files are not removed. If relaunch fails, open the restored or unchanged executable manually.

If an intervening file modification, uncertain ownership or sharing error makes recovery unsafe, the updater preserves the backup and lock for manual recovery rather than overwriting the changed file. The Updates page shows the last readable result, including rollback and backup information, on launch/refresh.

The app updater does not write game folders, accounts, saves, Journey, guides, configuration or shortcuts (the separate first-run copy above writes only the copy and the shortcut). Normal report/config behavior is otherwise unchanged. It does not promise atomic recovery from power loss or disk failure.

### Manual recovery

Close only Relax Coach instances involved in the update. Inspect the adjacent transaction journal and `backup.exe`; do not remove a lock while a worker may still be running. If automatic rollback was refused because a target changed, preserve that target separately before restoring a verified backup. Keep the transaction directory until the restored app opens successfully. If uncertain, retain all files and report the journal/error rather than deleting recovery material.

## Developer verification

Primary interfaces:

```go
CheckAppRelease(ctx context.Context, current string) (AppReleaseInfo, error)
ApplyAppRelease(ctx context.Context, release AppReleaseInfo) (AppUpdateResult, error)
```

`AppUpdateResult.RestartReady` is set only after the trusted worker confirms readiness. Native response posting must succeed before the UI-thread close is dispatched.

Internal process modes (not user-facing commands):

- `-app-update-worker <transaction-id>`: exact two arguments; derives its transaction from its own `worker.exe` directory, verifies original version/hash and the connected parent handle.
- `-app-update-start <transaction-id> [supported GUI arguments]`: verifies the installed executable and compiled version against that transaction. Only a later real native `appReady()` may acknowledge health.

No shipped transport override, environment-based update bypass, fault switch, downloaded script, alternate update channel or new dependency is provided. Unit seams are the updater's Go HTTP client, the transaction engine's process/move operations, exclusive file creation and status rename operations, and native service functions. Private test builds can override `Version` using `-X main.Version=...`; do not publish such fixtures.

From `desktop`, run Go race tests and vet, generate the real native HTML fixture with `NATIVE_HTML_OUT` before running **all** `native*_ui_test.cjs` suites, and cross-vet/build Windows:

```sh
go test -race -count=1 ./...
go vet ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c -o /private/evidence/app-update-tests.exe
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags '-H windowsgui' -o /private/evidence/RelaxCoach.exe
```

Keep test binaries, generated reports, configuration and real-save fixtures outside the source tree. Supply the existing real-fixture environment explicitly when exercising those suites. Cross-compilation and DOM simulation are **not** a Windows update pass. Before installation/publication, separately run Windows pipe/process/junction tests and isolated actual-app success and forced-startup-failure rollback exercises, verify file hashes and preserved configuration/runtime manifests, and obtain independent whole-feature review.
