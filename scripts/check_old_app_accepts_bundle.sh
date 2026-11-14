#!/usr/bin/env bash
# Release check: the addon updater of an older published app must accept the
# real built bundle, so players who press Update Addon before Update App still
# get it. Runs that app's own installer code (git archive of the tag) against
# the bundle through a local HTTPS fixture; nothing is downloaded or published.
# Usage: scripts/check_old_app_accepts_bundle.sh <old-app-tag> <bundle.zip> <version>
set -euo pipefail
cd "$(dirname "$0")/.."
OLD=${1:?old app tag, e.g. v1.5.6}
BUNDLE=$(realpath "${2:?bundle zip}")
VERSION=${3:?bundle version}
git rev-parse -q --verify "$OLD^{commit}" >/dev/null || { echo "unknown tag $OLD (git fetch --tags)" >&2; exit 1; }
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
git archive "$OLD" desktop | tar -x -C "$WORK"
cat > "$WORK/desktop/zz_old_app_probe_test.go" <<'EOF'
package main

import (
	"context"
	"os"
	"testing"
)

// The published app's own installer (this tree) applies the real bundle.
func TestOldAppAcceptsRealBundle(t *testing.T) {
	payload, err := os.ReadFile(os.Getenv("RC_BUNDLE"))
	if err != nil {
		t.Fatal(err)
	}
	version := os.Getenv("RC_VERSION")
	game := guildGame(t)
	result, err := guildUpdaterForVersion(t, payload, version).apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil || result.Version != version {
		t.Fatalf("app %s refused bundle %s: %+v %v", os.Getenv("RC_OLD"), version, result, err)
	}
	t.Logf("app %s installed bundle %s (%d files)", os.Getenv("RC_OLD"), version, result.UpdatedFiles)
}
EOF
(cd "$WORK/desktop" && RC_BUNDLE="$BUNDLE" RC_VERSION="$VERSION" RC_OLD="$OLD" go test -count=1 -run '^TestOldAppAcceptsRealBundle$' -v .)
printf 'Old app %s accepts %s.\n' "$OLD" "$(basename "$BUNDLE")"
