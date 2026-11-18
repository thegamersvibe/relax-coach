#!/usr/bin/env python3
"""Relax Raid per-module test runner (PLAN.md 3.4). Standard library only; Python 3.12 syntax.

  python -I -B tools/raidtest.py <module>...         run those modules
  python -I -B tools/raidtest.py --all [--kind go]   run every module (optionally only some kinds)
  python -I -B tools/raidtest.py --changed <ref>     modules touched since <ref>, plus their dependents
  python -I -B tools/raidtest.py --mutants <module>  that module's mutants, each against its killed_by test

Modules are the directories test/raid/<name>/ holding a module.json:
  {"kind": "go"|"lua"|"py", "source": "<path>", "test": ["<argv>", ...], "depends": ["<module>", ...]}
Test commands run from the repository root (Go modules use "go -C desktop ..."). mutants.json holds a list of
{"file", "old", "new", "killed_by"}. A mutant must sit inside the module's source and its old text must occur
exactly once. Product files are never written: a Go mutant is swapped in with go test -overlay and runs only the
killed_by test; a Lua or Python mutant runs in a temporary copy of the module's files with "-k killed_by".
Exit 1 when a module fails, a mutant survives or is refused, or a module runs longer than the limit.
"""
import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time

KINDS = ("go", "lua", "py")
RAID_PATHS = ("desktop/internal/raid/", "addon/RelaxRaid/", "test/raid/", "tools/raidtest.py")


def repo_root(start):
    return os.path.abspath(start)


def load_modules(root):
    """{name: spec} for every test/raid/<name>/module.json, spec gaining name and dir."""
    base = os.path.join(root, "test", "raid")
    mods = {}
    if not os.path.isdir(base):
        return mods
    for name in sorted(os.listdir(base)):
        path = os.path.join(base, name, "module.json")
        if not os.path.isfile(path):
            continue
        with open(path, encoding="utf-8") as f:
            spec = json.load(f)
        if spec.get("kind") not in KINDS or not isinstance(spec.get("source"), str) or not isinstance(spec.get("test"), list):
            raise SystemExit("%s: needs kind (go|lua|py), source and test" % path)
        spec.setdefault("depends", [])
        spec["name"] = name
        spec["dir"] = "test/raid/%s/" % name
        mods[name] = spec
    for name, spec in mods.items():
        for dep in spec["depends"]:
            if dep not in mods:
                raise SystemExit("module %s depends on unknown module %s" % (name, dep))
    return mods


def under(path, prefix):
    prefix = prefix.rstrip("/")
    return path == prefix or path.startswith(prefix + "/")


def select_changed(mods, changed):
    """Modules owning a changed file, plus every module depending on them (transitively).
    A changed file in the raid tree that no module claims selects everything."""
    picked = set()
    for path in changed:
        owners = [n for n, s in mods.items() if under(path, s["source"]) or under(path, s["dir"])]
        if owners:
            picked.update(owners)
        elif any(under(path, p.rstrip("/")) for p in RAID_PATHS):
            return sorted(mods)
    grew = True
    while grew:
        grew = False
        for name, spec in mods.items():
            if name not in picked and any(d in picked for d in spec["depends"]):
                picked.add(name)
                grew = True
    return sorted(picked)


def changed_files(root, ref):
    out = subprocess.run(["git", "diff", "--name-only", ref], cwd=root, capture_output=True, text=True, check=True).stdout
    extra = subprocess.run(["git", "ls-files", "--others", "--exclude-standard"], cwd=root, capture_output=True, text=True, check=True).stdout
    return sorted({p for p in (out + extra).splitlines() if p})


def run_module(root, spec, limit):
    start = time.monotonic()
    try:
        proc = subprocess.run(spec["test"], cwd=root, capture_output=True, text=True, timeout=limit + 5)
        rc, output = proc.returncode, proc.stdout + proc.stderr
    except subprocess.TimeoutExpired as e:
        rc, output = -1, "timed out\n" + str(e.stdout or "")
    seconds = time.monotonic() - start
    ok = rc == 0 and seconds <= limit
    why = "" if ok else ("rc %d" % rc if rc else "slower than the %gs limit" % limit)
    return ok, seconds, why, output


def go_package(spec):
    src = spec["source"].rstrip("/")
    if not src.startswith("desktop/"):
        raise SystemExit("Go module %s: source must be under desktop/" % spec["name"])
    return "./" + src[len("desktop/"):]


def run_mutant(root, spec, m, limit):
    """(killed, detail). Refusals raise ValueError."""
    for key in ("file", "old", "new", "killed_by"):
        if not isinstance(m.get(key), str):
            raise ValueError("mutant needs file, old, new and killed_by")
    if not under(m["file"], spec["source"]):
        raise ValueError("%s is outside module source %s" % (m["file"], spec["source"]))
    target = os.path.join(root, m["file"])
    with open(target, encoding="utf-8") as f:
        text = f.read()
    count = text.count(m["old"])
    if count != 1:
        raise ValueError("old text occurs %d times in %s (must be exactly once)" % (count, m["file"]))
    mutated = text.replace(m["old"], m["new"], 1)
    with tempfile.TemporaryDirectory(prefix="raidtest-") as tmp:
        if spec["kind"] == "go":
            swap = os.path.join(tmp, os.path.basename(target))
            with open(swap, "w", encoding="utf-8") as f:
                f.write(mutated)
            overlay = os.path.join(tmp, "overlay.json")
            with open(overlay, "w", encoding="utf-8") as f:
                json.dump({"Replace": {os.path.abspath(target): swap}}, f)
            cmd = ["go", "-C", "desktop", "test", "-count=1", "-overlay", overlay, "-run", "^(" + m["killed_by"] + ")$", go_package(spec)]
            cwd = root
        else:
            # copy only the module's own files and the shared support code; the product tree is not touched
            for rel in {spec["source"].rstrip("/"), spec["dir"].rstrip("/"), "test/raid/_client", "tools"}:
                src = os.path.join(root, rel)
                dst = os.path.join(tmp, rel)
                if os.path.isdir(src):
                    shutil.copytree(src, dst, dirs_exist_ok=True)
                elif os.path.isfile(src):
                    os.makedirs(os.path.dirname(dst), exist_ok=True)
                    shutil.copy2(src, dst)
            with open(os.path.join(tmp, m["file"]), "w", encoding="utf-8") as f:
                f.write(mutated)
            cmd, cwd = list(spec["test"]) + ["-k", m["killed_by"]], tmp
        try:
            proc = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=limit + 5)
            killed = proc.returncode != 0
            detail = (proc.stdout + proc.stderr)[-400:]
        except subprocess.TimeoutExpired:
            killed, detail = True, "timed out (counts as killed)"
    return killed, detail


def main(argv=None):
    ap = argparse.ArgumentParser(description="Relax Raid per-module test runner")
    ap.add_argument("modules", nargs="*")
    ap.add_argument("--all", action="store_true")
    ap.add_argument("--kind", help="with --all or --changed: comma-separated kinds (go, lua, py)")
    ap.add_argument("--changed", metavar="REF")
    ap.add_argument("--mutants", metavar="MODULE")
    ap.add_argument("--limit", type=float, default=30.0, help="seconds allowed per module (default 30)")
    ap.add_argument("--root", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    ap.add_argument("--list", action="store_true", help="print the selected modules and exit")
    args = ap.parse_args(argv)
    root = repo_root(args.root)
    mods = load_modules(root)

    if args.mutants:
        spec = mods.get(args.mutants)
        if not spec:
            print("unknown module %s" % args.mutants)
            return 1
        path = os.path.join(root, spec["dir"], "mutants.json")
        with open(path, encoding="utf-8") as f:
            mutants = json.load(f)
        failed = 0
        for i, m in enumerate(mutants, 1):
            label = "%s #%d (%s)" % (args.mutants, i, m.get("killed_by"))
            try:
                start = time.monotonic()
                killed, detail = run_mutant(root, spec, m, args.limit)
            except ValueError as e:
                print("REFUSED %s: %s" % (label, e))
                failed += 1
                continue
            print("%s %s %.1fs" % ("KILLED" if killed else "SURVIVED", label, time.monotonic() - start))
            if not killed:
                failed += 1
                print(detail)
        print("RESULT: %d/%d mutants killed" % (len(mutants) - failed, len(mutants)))
        return 1 if failed else 0

    kinds = set(args.kind.split(",")) if args.kind else set(KINDS)
    if kinds - set(KINDS):
        print("unknown kind: %s" % ",".join(sorted(kinds - set(KINDS))))
        return 1
    if args.all:
        names = [n for n in mods if mods[n]["kind"] in kinds]
    elif args.changed:
        names = [n for n in select_changed(mods, changed_files(root, args.changed)) if mods[n]["kind"] in kinds]
    else:
        unknown = [n for n in args.modules if n not in mods]
        if unknown or not args.modules:
            print("unknown or no modules: %s (known: %s)" % (", ".join(unknown), ", ".join(mods)))
            return 1
        names = args.modules
    if args.list:
        print("\n".join(names))
        return 0
    failed = 0
    for name in names:
        ok, seconds, why, output = run_module(root, mods[name], args.limit)
        print("%-4s %-20s %6.1fs%s" % ("ok" if ok else "FAIL", name, seconds, "" if ok else "  " + why))
        if not ok:
            failed += 1
            print(output)
    print("RESULT: %d/%d modules passed" % (len(names) - failed, len(names)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
