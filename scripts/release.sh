#!/usr/bin/env bash
# Build release artifacts only. Never commits, tags, pushes or publishes.
set -euo pipefail
cd "$(dirname "$0")/.."
V=${1:-1.5.6}
mkdir -p dist
(cd desktop && go test ./... && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H windowsgui" -o ../dist/RelaxCoach.exe .)
python3 scripts/build_guild_bundle.py --exe dist/RelaxCoach.exe --version "$V"
python3 test/test_guild_bundle.py
# The previous published app (default: the newest v* tag in this history, or
# OLD_APP=vX.Y.Z) must accept the real bundle, so Update Addon before Update App
# still works for players who have not updated the app yet.
OLD_APP=${OLD_APP:-$(git describe --tags --abbrev=0 --match 'v*' HEAD)}
scripts/check_old_app_accepts_bundle.sh "$OLD_APP" "dist/RelaxCoach-Bundle-$V.zip" "$V"
printf 'Local release assets built. Review dist/SHA256SUMS.txt before publishing.\n'
