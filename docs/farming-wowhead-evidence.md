# Desktop Wowhead fallback and item links

This is implementation evidence, not release notes. No bundled item-name data,
addon changes, dependency changes, or version changes are part of this branch.

## Forever endpoint evidence

Two read-only GET requests from the development machine on 2026-11-20, with an
app-identifying User-Agent (no cookies or authentication):

1. <https://www.wowhead.com/forever> returned HTTP 200. The title was
   `WoW Forever News and Guides`; its page configuration's `dataEnv` object
   contained `"env":16,"tree":16`. The Forever newsletter's `popupEnvs` also
   contained `[16]`.
2. <https://nether.wowhead.com/tooltip/item/2772?dataEnv=16&locale=0> returned
   HTTP 200 JSON. Its keys were `icon`, `name`, `quality`, `spells`, `tooltip`;
   `name` was a nonempty string and `quality` was the integer `1`.
   The actual item name and tooltip body are deliberately not reproduced here.

Thus the desktop uses **dataEnv 16**, locale 0, not a guessed Retail or Classic
mapping. Tests use only httptest servers, invented names and synthetic saves;
none of the automated tests contacts Wowhead.

## Runtime and privacy contract

- Main initializes one desktop-owned worker, cancelled/joined on normal exit. Zero-value
  test Apps and console `-once` runs remain offline. Network and cache writes run
  off the report path; new names become visible on the ordinary five-second
  native report refresh. No remote JS/CSS is loaded in the WebView.
- Only canonical, positive history item IDs with packed string values are
  eligible. AH Profit save/bundled files, Coach client cache, and installed
  GearQuest names all take precedence. Admission and rendering use the same
  local merge. Refreshes recheck eligibility without growing an active batch;
  duplicate IDs across houses are requested once. Local ID quality still wins,
  including quality zero and quality-only local entries.
- GET sends the ID, fixed environment/locale, and the identifying header
  `RelaxCoach/<Version> (+https://github.com/thegamersvibe/relax-coach)`.
  No characters, account names, price data or saves are sent. Redirects and
  cookies are disabled for this client. Tooltip JSON is decoded into only
  `name` and `quality`; HTML, stats, icons and spells are not retained.
- Cache: `os.UserConfigDir()/ForeverCoach/wowhead-names-16.json` (on Windows,
  `%AppData%\ForeverCoach\wowhead-names-16.json`). The root is an object keyed by
  decimal item ID. Positive entries contain `name`, optional numeric `quality`,
  and Unix-seconds `fetched_at`. A 404 or invalid HTTP 200 entry contains only
  `missing: true` and `fetched_at`. There is no player/account key, response body,
  or upload path. Each changed batch writes one synced private temporary file
  and replacement, including partial progress before failure or cancellation.
  An unchanged refresh does not write the cache.
  Read/write failures are nonfatal; malformed, oversized, expired or invalid
  saved entries are ignored. Retention evicts the oldest entry at its cap.
- `config.json` stores the boolean `wowheadNames` through the existing serialized
  read/modify/write helper, preserving all other settings. The absent setting
  defaults on; unreadable settings fail closed. Settings exposes
  **Look up missing item names online (Wowhead)**. Off cancels the current batch
  and prevents new requests; still-valid cached names remain usable. Preference
  replies use the report-generation guard, so stale successes or failures cannot
  restore an earlier account. After any reply the checkbox is reconciled with
  the latest report's stored setting; a failed save for the current account still
  shows its error. The account selector stays usable during a save.
- Sell-now, rising and falling item names link to
  `https://www.wowhead.com/forever/item=<id>`. The existing delegated link handler
  prevents WebView navigation and calls `openExternal`; both the bridge and OS
  opener validate the literal `https://www.wowhead.com/forever/item=<digits>`
  shape. Invalid item paths/hosts, encoded digits, ports, queries and fragments
  are refused. Wowhead guide routes retain main's documentation behavior on
  `www.wowhead.com` and `wowhead.com`, including port 443, escaped paths, queries
  and fragments; the existing HTTPS, credentials and dot-segment guards remain.
  Project-release and Icy Veins links are unchanged.
  `ShellExecuteW` opens the default browser. No PowerShell was added.
  Auction statistics have names but no IDs; gathering kinds have hashes rather
  than item IDs. These cannot get reliable item links without changing the donor
  format, so no IDs are guessed.

### Named bounds

| Constant | Value |
| --- | --- |
| `wowheadMaxPerRefresh` | 200 requests |
| `wowheadRequestInterval` | 1 second between request starts, across batches |
| `wowheadRequestTimeout` | 10 seconds per request |
| `wowheadNameTTL` | 30 days |
| `wowheadNegativeTTL` | 24 hours |
| `wowheadBackoffMin` / `wowheadBackoffMax` | 1 minute / 30 minutes, exponential |
| `wowheadMaxResponseBytes` | 256 KiB |
| `wowheadMaxCacheBytes` / `wowheadMaxCacheItems` | 4 MiB / 10,000 entries |

429/5xx stop the batch. Retry-After delta seconds and HTTP dates extend backoff
when longer, capped at 30 minutes; overflowing all-digit delays also use that cap.
Other HTTP failures and offline/read failures stop/back off without discarding
previous cache entries. A 404 or invalid HTTP 200 is negative-cached for 24 hours
and does not stop the batch. Invalid includes malformed JSON, wrong field types,
invalid names/qualities and bounded reads exceeding the response-size limit.

## Initial implementation red-to-green evidence

- `TestFarmWowheadLinks`: RED, hot/sell names had no Forever links; GREEN after
  adding anchors around the existing quality-coloured name spans.
- `TestWowheadLookupAndMinimalCache`: missing interface first, then assertion
  RED with the compile-only shell (`requests=0`), then GREEN with real httptest
  HTTP and a reopened local cache. Persisted keys are asserted exactly.
- Bounds/rate: RED at 201 requests instead of 200 and two immediate requests;
  GREEN with the shared limiter. A real timer test complements a fake clock
  that checks every request interval, including the next refresh boundary.
- TTL/negative caching: RED with stale positives never refreshed and 404s
  immediately re-requested; GREEN at the exact 30-day/24-hour boundaries.
- Backoff: five RED cases bypassed backoff on refresh (429, 500, 502, 503;
  delta/date/malformed/zero Retry-After); GREEN after the batch/backoff gate.
- Validation: RED for invalid quality, placeholder names, oversized responses,
  redirects, corrupt fresh entries, oversized entry counts and retention overflow;
  GREEN after validation and bounded client/cache handling. Offline and malformed
  response tests retain the original cache bytes.
- App integration: RED because missing history IDs were never queued and enabling
  the toggle did not start lookup; GREEN with a held fake response proving refresh
  returns without waiting, opt-out cancellation, and all local source priorities.
- Serialized-byte bound: RED at 8,108,895 bytes for 10,000 valid maximum-length
  HTML-sensitive test names; GREEN with non-HTML JSON encoding and a write cap.
- jsdom RED against the pre-feature Go-rendered fixtures: `tests 23`, `pass 20`,
  `fail 3`, `skipped 0`. Missing item links and both missing settings controls
  failed assertions, not harness setup. UI tests exercise hrefs, prevented
  navigation, the external-open binding, persisted off state, write failure
  restoration and absence of remote script/stylesheet tags.

## Review correction red-to-green evidence

- The committed preference-race probe in `native_farm_ui_test.cjs` first failed
  both reply orders: A replaced B. With the generation guard it passes before
  and after B's reply. Failure replies are covered too; the follow-up RED exposed
  a stale toggle restoration and stuck saving message. All four ordering/outcome
  combinations pass, with the selector usable throughout the preference save.
- `TestWowheadInvalidResponseDoesNotStarveBatch`: all 15 invalid-response cases
  first stopped at the bad ID (`bad=1 good=0`). GREEN checks continued work over
  40 refreshes, a response-free persisted marker, restart and exact 24-hour expiry.
  The inherited malformed-response test now permits only that negative marker;
  existing positive values and timestamps, and offline cache bytes, remain pinned.
- `TestNativeWowheadOpenerExactShape`: RED admitted non-item Wowhead URLs at the
  real native bridge's recording browser service. GREEN rejects all invalid
  shapes, including empty query/fragment delimiters. That correction also
  refused existing Wowhead guide links; the compatibility correction below
  restores their prior allowance. Project-release and Icy Veins controls pass.
- `TestWowheadRetryAfterCapped`: six RED cases delayed 24 hours, saturated the
  duration, or fell back to one minute on overflow. All eight cases now pass,
  including HTTP dates, huge decimals, the exact cap and leading-zero seconds.
- `TestWowheadBatchPersistsOnce`: RED observed 200 atomic replacements for a full
  batch and two for both partial exits. GREEN observes one replacement for each
  changed batch, zero for no progress, complete positive/negative entries, and no
  extra writes on unchanged refresh. Open file handles prevent inode reuse from
  hiding replacements. The writer still performs one write and one sync per file.
- Targeted final race run of all Wowhead tests and native opener/allowlist tests:
  `ok github.com/thegamersvibe/relax-coach/desktop 5.196s`, exit 0.
- Full `go test -race -count=1 -timeout 20m ./...`, under the shared heavy-job
  lock: `ok github.com/thegamersvibe/relax-coach/desktop 518.128s`, exit 0.
  All eight tested packages passed; the model test-helper package has no tests.
- Fresh Go-generated fixtures and CI's `node --test desktop/*_ui_test.cjs`, also
  under the shared lock: `tests 162`, `pass 162`, `fail 0`, `cancelled 0`,
  `skipped 0`, exit 0. Both complete jobs ran in the background and were polled
  to completion; the UI job waited for a free slot before running.
- The unchanged original preference-race probe independently passed both orders:
  during the switch B remained selected with an empty farm; after it, B's farm
  remained displayed. `tests 2`, `pass 2`, `fail 0`, exit 0.
- `go vet ./...` and the Windows amd64 cross-build both exited 0. No dependency
  files changed. The private-token scan found no matches in changed/new source
  files across the feature stack; this fix does not change README.md.

## Preference truth and guide compatibility correction

- The held-reply jsdom tests use the unchanged stored value after a failed
  opt-out. Four assertions failed before the fix; all seven focused preference
  tests now pass. Returning to the original account before rejection still
  reports the save error. A separate error-suppression mutant fails that check.
- `TestWowheadStoredOffMakesNoRequests` verifies two native refreshes make zero
  loopback requests with stored opt-out, then enabling the same fixture makes
  one request. It passes unchanged production code; forcing the worker on in a
  private overlay fails with `requests=1`.
- All eight `TestNativeWowheadTalentGuideLinks` cases first failed at the browser
  service, after successfully traversing `enrich` and the report template. They
  pass with the fix and with main's original validator in a private overlay.
  The exact item rule and all prior non-guide refusals remain covered; only the
  guide expectation in the previous refusal matrix moves to the allowed set.
- Targeted race tests: `ok github.com/thegamersvibe/relax-coach/desktop 6.021s`.
  The unchanged reviewer's preference probe also passes: `tests 10`, `pass 10`,
  `fail 0`, `skipped 0`.
- Full locked `go test -race -count=1 -timeout 20m ./...`: all eight tested
  packages pass, exit 0; desktop result is `616.224s`. The model test-helper
  package has no tests. Linux/Windows vet and the Windows amd64 cross-build
  exit 0. Fresh-fixture UI tests under the same lock and CI glob pass:
  `tests 163`, `pass 163`, `fail 0`, `cancelled 0`, `skipped 0`, exit 0.
  These are offline tests, not native Windows acceptance.

## Initial implementation verification

The results below predate the review corrections; they are retained as historical
evidence, not claimed as final-tree runs for the correction commit.

Commands ran with the repository's Go toolchain, locked jsdom dependencies,
Python 3.14 and the pinned CI Python requirements. Full desktop tests, race tests
and UI suites held the shared heavy-job lock. Generated fixtures/logs and the
cross-built executable are local ignored test output, not part of this commit.

- Targeted final `go test -count=1 -run '^TestWowhead|^TestFarmWowhead' .`:
  `ok github.com/thegamersvibe/relax-coach/desktop 1.657s`, exit 0.
- Full `go test -count=1 -timeout 20m ./...`:
  `ok github.com/thegamersvibe/relax-coach/desktop 458.464s`; all tested internal
  packages also passed, exit 0. This run started before the final serialized-byte
  hardening; the final-tree race run below includes that additional test/fix.
- Final-tree full `go test -race -count=1 -timeout 20m ./...`:
  `ok github.com/thegamersvibe/relax-coach/desktop 732.040s`; all tested internal
  packages also passed, exit 0, with no race reports.
- `go vet ./...`: exit 0 after the final race run.
- With fresh Go fixtures and CI's `NODE_PATH`, `NATIVE_HTML_OUT`, `FARM_HTML_OUT`,
  `node --test desktop/*_ui_test.cjs`: `tests 158`, `pass 158`, `fail 0`,
  `cancelled 0`, `skipped 0`, exit 0.
- `python -I -B test/item_names_test.py` through isolated uv / Python 3.14 /
  `.github/requirements-ci.txt`: `Ran 27 tests in 1.396s`, `OK`, exit 0.
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags
  '-H windowsgui' -o <ignored-test-output>/RelaxCoach.exe .`: exit 0; output
  has the PE executable `MZ` header. This was a cross-build, not a Windows run.
- Changed Go files are gofmt-clean; `git diff --check` is clean. The requested
  private-token scan is clean across changed/new tracked-source candidates.
  README changes are confined to the Farming feature paragraph, with shipped
  release/version sections byte-identical to the base.

An initial foreground UI invocation exhausted the tool's wait window while
queued for the shared lock. The explicit background UI RED and GREEN runs above
subsequently completed; a lock wait was not reported as a test pass. The rest
of the repository's Lua/packaging CI matrix was not run for this app-only change.

Not tested on a live client: not run (alpha, owner's decision).
Offline tests and Windows cross-compilation are not native Windows or Forever
acceptance. No install, push, PR, tag or publication is performed by this work.
