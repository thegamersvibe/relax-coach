#!/usr/bin/env bash
# Build release artifacts only. Never commits, tags, pushes or publishes.
set -euo pipefail
cd "$(dirname "$0")/.."
V=${1:-1.4.3}
mkdir -p dist
(cd desktop && go test ./... && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H windowsgui" -o ../dist/RelaxCoach.exe .)
python3 scripts/build_guild_bundle.py --exe dist/RelaxCoach.exe --version "$V"
python3 test/test_guild_bundle.py
printf 'Local release assets built. Review dist/SHA256SUMS.txt before publishing.\n'
