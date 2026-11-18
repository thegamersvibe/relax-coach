#!/usr/bin/env bash
# Build release artifacts only. Never commits, tags, pushes or publishes.
set -euo pipefail
cd "$(dirname "$0")/.."
V=${1:-1.7.2}
mkdir -p dist
(cd desktop && go test ./... && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H windowsgui" -o ../dist/RelaxCoach.exe .)
python3 scripts/build_guild_bundle.py --exe dist/RelaxCoach.exe --version "$V"
python3 test/test_guild_bundle.py
python3 -I -B test/test_release_gate.py
# Reviewed app-first exception: 1.7.0 adds Media/Reactions/flight.mp3, which
# older apps cannot install. Use the last published tag, never an override,
# and require its real installer to refuse cleanly instead of skipping it.
if [[ "$V" == "1.7.0" ]]; then
    # Exclude v1.7.0 itself so a rebuild after tagging still checks the previous app.
    OLD_APP=$(git describe --tags --abbrev=0 --match 'v*' --exclude 'v1.7.0' HEAD)
    scripts/check_old_app_refuses_bundle.sh "$OLD_APP" "dist/RelaxCoach-Bundle-$V.zip" "$V"
else
    # Every other version keeps the existing acceptance check and override.
    OLD_APP=${OLD_APP:-$(git describe --tags --abbrev=0 --match 'v*' HEAD)}
    scripts/check_old_app_accepts_bundle.sh "$OLD_APP" "dist/RelaxCoach-Bundle-$V.zip" "$V"
fi
printf 'Local release assets built. Review dist/SHA256SUMS.txt before publishing.\n'
