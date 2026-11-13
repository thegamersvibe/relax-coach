# Contributing to Relax Coach

Small, reviewed changes beat surprise rewrites. AI-assisted contributions are
welcome; the person submitting the PR remains responsible for the result.

## Who can merge and publish?

**Tareksoh and buiden are the only merge/release custodians.** Tareksoh is the
primary release coordinator; buiden is the backup. Both remain organization
owners. Other contributors use fork PRs with Read/Triage access, not upstream
Write/Maintain (those roles also permit release operations).

The `main` ruleset requires a PR, one independent approving review, applicable
code-owner approval, approval of the most recent reviewable push, resolved
conversations and an up-to-date successful **Required CI** check from GitHub
Actions. New commits dismiss stale approvals. Direct pushes, force-pushes and
branch deletion are not the normal contribution path; there is no routine bypass.
The PR author cannot supply their own approval. With these two custodians, the
other custodian reviews the PR. Keep review about the final diff, not the model
that wrote it.

`CODEOWNERS` routes reviews; it is not a permission boundary. Owners retain
administrative power to change settings. The ruleset is enforced while active,
not an irreversible restriction on an organization owner.

## Pick a contribution lane

### Change Coach

Open **Change Coach** with the problem, intended behavior, out-of-scope areas
and verification plan. Agree scope with a custodian before a substantial change.
Use a short-lived branch such as `fix/bag-event-edge-case` or
`feat/session-summary`. Fix one thing; separate refactors and feature additions.

### Include my addon

Open **Include my addon** before copying or vendoring anything. Provide:

- Source repository and exact revision/version; license and redistribution rights.
- A named maintainer, addon folder names, dependencies and SavedVariables names.
- The actual Forever client/build tested and reproducible evidence.
- Default enabled/muted behavior, triggers/cooldowns, external communication,
  data sharing, performance risks and collision risks.
- Install/update/uninstall/rollback behavior, ownership of updates and save
  preservation. Identify compatibility with older desktop/updater versions.

Approval to evaluate an addon is not approval to bundle it. The current updater
validates a fixed set of addon roots and manifest contracts. Supporting a new
root needs a separately reviewed updater migration and old/new-client tests.
Until then, a reviewed optional link is safer than pretending it is managed.

## Hermes quick start

Clone your fork (custodians may use an upstream topic branch), select the clone
root as the Hermes working directory, and start a fresh session there. Read
`AGENTS.md` and this file explicitly at the start. A higher-priority `.hermes.md`
or personal `AGENTS.override.md` can replace automatic loading, so do not assume
the project instructions loaded just because Hermes is running.

Paste this into your own Hermes session:

> Read AGENTS.md and CONTRIBUTING.md. Work only on the approved issue in a topic
> branch in my fork. Inspect the current branch and preserve unrelated work.
> Run the relevant checks, review the diff, and open a PR against main with test
> evidence. Stop before merging, installing, changing permissions or publishing.

Use your own GitHub identity and credentials. Do not copy another person's
Hermes profile, credentials, memory or private project logs into this repository.
Agent instructions help coordination; actual access comes from GitHub.

## Local checks

Use an isolated environment. The commands below are for a POSIX shell (Linux,
macOS or WSL), from the clone root. Match the Go version in `desktop/go.mod`,
Python 3.14 and Node 26 used by CI. Go race tests need a C compiler.

```sh
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -r .github/requirements-ci.txt

env -u SV_OUT python -I -B test/harness.py
python -I -B test/reactions_test.py
python -I -B .github/tests/test_check_results.py

# The Go native-shell test writes the fixture consumed by the offline UI tests.
export NATIVE_HTML_OUT="$PWD/dist/native-test.html"
mkdir -p dist
(cd desktop && go test -race -count=1 ./... && go vet ./...)
npm ci --prefix .github --ignore-scripts
NODE_PATH="$PWD/.github/node_modules" node --test desktop/*_ui_test.cjs

# Build/test locally only: nothing is installed, uploaded or published.
(cd desktop && GOOS=windows GOARCH=amd64 CGO_ENABLED=0   go build -trimpath -ldflags '-H windowsgui' -o ../dist/RelaxCoach.exe .)
python scripts/build_guild_bundle.py --exe dist/RelaxCoach.exe
python -I -B test/test_guild_bundle.py
```

Windows resources (icon, version, manifest) live in `desktop/winres/winres.json`. After changing the version or icon, regenerate the committed resource object, and keep the plain build above: no UPX or other packers and no obfuscation, because antivirus heuristics flag packed Go binaries.

```
(cd desktop && go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --arch amd64 --out resource)
```

The workflow is the executable source of truth for CI commands. Its stable
`Required CI` gate fails if an expected job fails, is cancelled, is skipped or
is absent. No path filters skip documentation-only PRs. Checks use disposable
GitHub-hosted runners, read-only tokens, no persisted checkout credentials and
no release/deployment job. External fork workflows require maintainer approval
before execution; inspect the diff before approving. Never run an unreviewed
fork's code on a personal or self-hosted runner.

These checks are simulations and build/package tests, **not native acceptance**.
For runtime changes, record actual Forever client/build, Windows companion
version, steps/results and any Lua errors. Mark untested areas honestly. Use
synthetic/sanitized fixtures; do not attach real player saves to a public issue.

## PR, merge and release are separate

1. Open a focused PR using the template. Link the agreed scope, summarize the
   behavior, tests, native-test status, default changes and rollback impact.
2. Address review and wait for green required checks. A fresh push needs fresh
   approval; do not weaken protection to get unstuck.
3. A custodian merges only the reviewed change. A merge does **not** publish it.
4. A release needs a separate explicit custodian decision, native evidence when
   relevant, version/package review, updater compatibility checks and rollback
   verification. Do not move existing tags or replace published assets casually.

The CI workflow creates private ephemeral build files only. It never installs
into a user's game or updates the published release.
