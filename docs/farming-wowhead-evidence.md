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
  and Unix-seconds `fetched_at`. A 404 entry contains only `missing: true` and
  `fetched_at`. There is no player/account key, response body, or upload path.
  Writes use a synced private temporary file and replacement, like config.json.
  Read/write failures are nonfatal; malformed, oversized, expired or invalid
  saved entries are ignored. Retention evicts the oldest entry at its cap.
- `config.json` stores the boolean `wowheadNames` through the existing serialized
  read/modify/write helper, preserving all other settings. The absent setting
  defaults on; unreadable settings fail closed. Settings exposes
  **Look up missing item names online (Wowhead)**. Off cancels the current batch
  and prevents new requests; still-valid cached names remain usable.
- Sell-now, rising and falling item names link to
  `https://www.wowhead.com/forever/item=<id>`. The existing delegated link handler
  prevents WebView navigation and calls `openExternal`; the native HTTPS
  allowlist and `ShellExecuteW` open the default browser. No PowerShell was added.
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
when longer; the server's delay is not capped at the exponential limit. Other
HTTP failures, offline failures and malformed responses also stop/back off,
without changing the existing cache. 404 is the only negative-cache status.

## Red-to-green evidence

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

## Verification status

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
