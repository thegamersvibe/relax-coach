#!/usr/bin/env bash
# Reviewed 1.7.0 app-first release gate. Run the published app's OWN installer
# and native bridge against the real bundle via its local HTTPS test fixture.
# No skip/mode switch: pin the sole unknown path, error, UI wording and
# installed files. Only a new EMPTY backup root is allowed; no stage/lock.
# The same old app must also accept the actual built dist/RelaxCoach.exe.
# Usage: scripts/check_old_app_refuses_bundle.sh <old-app-tag> <bundle.zip> 1.7.0
set -euo pipefail
cd "$(dirname "$0")/.."
[[ $# == 3 && "$3" == "1.7.0" ]] || { printf 'Refusal gate is pinned to 1.7.0 only.\n' >&2; exit 1; }
OLD=$1
BUNDLE=$(realpath "$2")
EXE=$(realpath dist/RelaxCoach.exe)
git rev-parse -q --verify "refs/tags/$OLD^{commit}" >/dev/null || { echo "unknown tag $OLD (git fetch --tags)" >&2; exit 1; }
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
git archive "refs/tags/$OLD" desktop | tar -x -C "$WORK"
cp scripts/old_app_refusal_test.go "$WORK/desktop/zz_old_app_refusal_test.go"
(cd "$WORK/desktop" && RC_BUNDLE="$BUNDLE" RC_EXE="$EXE" RC_VERSION=1.7.0 RC_OLD="$OLD" go test -count=1 -run '^TestOldAppRefus' -v .)
printf 'Old app %s refuses the 1.7.0 bundle without file changes and accepts the replacement executable; app update required.\n' "$OLD"
