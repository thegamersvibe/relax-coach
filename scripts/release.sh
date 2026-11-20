#!/usr/bin/env bash
# Build release artifacts only. Never commits, tags, pushes or publishes.
set -euo pipefail
cd "$(dirname "$0")/.."
V=${1:-1.8.1}
[[ "$V" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { printf 'Invalid version\n' >&2; exit 1; }
# Reuse the builder's app-owned floor; a missing/unparseable floor is fatal.
RAID=$(python3 -B -c 'import runpy,sys; print("yes" if runpy.run_path("scripts/build_guild_bundle.py")["raid_release"](sys.argv[1]) else "no")' "$V")
BUNDLE="dist/RelaxCoach-Bundle-$V.zip"
if [[ "$RAID" == yes ]]; then
    [[ ! -e "$BUNDLE" && ! -L "$BUNDLE" ]] || { printf 'Refusing stale legacy bundle: %s\n' "$BUNDLE" >&2; exit 1; }
    BUNDLE="dist/RelaxCoach-RaidBundle-$V.zip"
fi
mkdir -p dist
(cd desktop && go test ./... && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H windowsgui" -o ../dist/RelaxCoach.exe .)
python3 scripts/build_guild_bundle.py --exe dist/RelaxCoach.exe --version "$V"
if [[ "$RAID" == yes ]]; then
    [[ -f dist/SHA256SUMS.txt ]] || { printf 'Missing release checksums\n' >&2; exit 1; }
    if [[ -e "dist/RelaxCoach-Bundle-$V.zip" || -L "dist/RelaxCoach-Bundle-$V.zip" ]] || grep -qF -- "  RelaxCoach-Bundle-$V.zip" dist/SHA256SUMS.txt; then
        printf 'Refusing legacy bundle asset or checksum in a raid release\n' >&2; exit 1
    fi
fi
python3 test/test_guild_bundle.py
python3 -I -B test/test_release_gate.py
# Reviewed app-first exception: 1.8.0 adds RelaxRaid and renames the asset.
# Old apps see an app-only release; their installer also refuses the new root.
if [[ "$V" == "1.8.0" ]]; then
    # A rebuild after tagging must still check the previous published app.
    OLD_APP=$(git describe --tags --abbrev=0 --match 'v*' --exclude 'v1.8.0' HEAD)
    scripts/check_old_app_refuses_bundle.sh "$OLD_APP" "$BUNDLE" "$V"
else
    # Every other version keeps the existing acceptance check and override.
    OLD_APP=${OLD_APP:-$(git describe --tags --abbrev=0 --match 'v*' HEAD)}
    scripts/check_old_app_accepts_bundle.sh "$OLD_APP" "$BUNDLE" "$V"
fi
printf 'Local release assets built. Review dist/SHA256SUMS.txt before publishing.\n'
