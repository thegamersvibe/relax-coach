#!/usr/bin/env python3
"""Copy the canonical raid JSON files for go:embed; run via go generate in desktop/raidrules.

The byte/inventory test runs before regeneration in CI, so stale copies fail.
No normalization or schema conversion: every copied byte is canonical.
"""
from pathlib import Path
import shutil


def generate(root):
    source = root / "rules" / "raid"
    destination = root / "desktop" / "raidrules" / "raid_rules"
    files = sorted(source.glob("*.json"))
    if not files:
        raise SystemExit("no canonical raid rules")
    # Read all inputs first; an unreadable source must not erase the old copy.
    data = {p.name: p.read_bytes() for p in files}
    if destination.exists():
        shutil.rmtree(destination)
    destination.mkdir(parents=True)
    for name, content in data.items():
        (destination / name).write_bytes(content)
    print(f"Copied {len(data)} raid rule files (byte-for-byte)")


if __name__ == "__main__":
    generate(Path(__file__).resolve().parents[1])
