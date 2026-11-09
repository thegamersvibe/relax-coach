"""Fail closed unless every expected upstream CI job succeeded."""
import json
import os
import sys

EXPECTED = {"coach-lua", "desktop-go", "bundle-contract"}


def main():
    try:
        jobs = json.loads(os.environ["NEEDS_JSON"])
    except (KeyError, ValueError):
        jobs = None
    passed = (
        isinstance(jobs, dict)
        and set(jobs) == EXPECTED
        and all(isinstance(row, dict) and row.get("result") == "success"
                for row in jobs.values())
    )
    if not passed:
        print("Required CI failed: every expected job must succeed.", file=sys.stderr)
        return 1
    print("Required CI passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
