#!/usr/bin/env python3
"""Relax Raid per-module test runner (PLAN.md 3.4). Standard library only; Python 3.12 syntax.

  python -I -B tools/raidtest.py <module>...         run those modules
  python -I -B tools/raidtest.py --all [--kind go]   run every module (optionally only some kinds)
  python -I -B tools/raidtest.py --changed <ref>     modules touched since <ref>, plus their dependents
  python -I -B tools/raidtest.py --mutants <module>  that module's mutants, each against its killed_by test

Modules are the directories test/raid/<name>/ holding a module.json:
  {"kind": "go"|"lua"|"py", "source": "<path>", "test": ["<argv>", ...], "depends": ["<module>", ...]}
Test commands run from the repository root (Go modules use "go -C desktop ..."). A module whose command runs no
test at all fails. mutants.json holds a list of {"file", "old", "new", "killed_by"}; an empty list is "no mutants".

A mutant is checked like this, and anything that is not a clear kill or survival is a REFUSAL, which fails the run:
  - its file, resolved (symlinks and "..") must lie inside the module's source; its old text must occur exactly
    once and differ from the new text;
  - the killed_by test must exist and pass on clean code (the baseline), in the same setting the mutant runs in;
  - with the mutation, the killed_by test must actually run: a build error, a selector that matches nothing or
    a timeout is a refusal, not a kill;
  - KILLED only when that named test's FAIL line is there AND the command exited non-zero; SURVIVED when its PASS
    line is there AND the exit is zero; the baseline needs PASS and exit zero; anything else is refused;
  - a Python mutant must compile, or it is refused (a syntax error is not a kill).
Product files are never written: a Go mutant is swapped in with go test -overlay; a Lua or Python mutant runs in a
temporary copy of the module's files, its declared dependencies' files and the shared support code, with -v -k.
Exit 1 when a module fails, a mutant survives or is refused, or a module runs longer than the limit.
"""
import argparse
import json
import os
import re
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


def ran_nothing(kind, output):
    """True when the output shows the command ran no test (go: no test files / no tests to run; unittest: 0)."""
    if kind == "go":
        return bool(re.search(r"\[no test files\]|\[no tests to run\]|no tests to run", output)) and not re.search(r"^ok\s+\S+\s+[\d.]+s\s*$", output, re.M)
    m = re.findall(r"^Ran (\d+) tests?", output, re.M)
    return not m or all(int(n) == 0 for n in m)


def run_module(root, spec, limit):
    start = time.monotonic()
    try:
        proc = subprocess.run(spec["test"], cwd=root, capture_output=True, text=True, timeout=limit + 5)
        rc, output = proc.returncode, proc.stdout + proc.stderr
    except subprocess.TimeoutExpired as e:
        rc, output = -1, "timed out\n" + str(e.stdout or "")
    seconds = time.monotonic() - start
    empty = rc == 0 and ran_nothing(spec["kind"], output)
    ok = rc == 0 and seconds <= limit and not empty
    why = "; ".join(r for r, bad in (("rc %d" % rc, rc != 0), ("ran no tests", empty),
                                      ("slower than the %gs limit" % limit, seconds > limit)) if bad)
    return ok, seconds, why, output


class Refused(Exception):
    pass


def inside(path, base):
    path, base = os.path.realpath(path), os.path.realpath(base)
    return path == base or path.startswith(base.rstrip(os.sep) + os.sep)


def go_named(output, name):
    """'pass' / 'fail' / None for a named Go test in -v output."""
    if re.search(r"^\s*--- FAIL: %s \(" % re.escape(name), output, re.M):
        return "fail"
    if re.search(r"^\s*--- PASS: %s \(" % re.escape(name), output, re.M):
        return "pass"
    return None


def py_named(output, name):
    """'pass' / 'fail' / None for unittest -v output: the named test's own result line."""
    results = re.findall(r"^(%s) \(.*?\)\s*(?:\.\.\.|\n.*?\.\.\.)\s*(ok|FAIL|ERROR|skipped.*)$" % re.escape(name), output, re.M)
    if not results:
        return None
    states = {r[1] for r in results}
    if "FAIL" in states or "ERROR" in states:
        return "fail"
    return "pass" if states == {"ok"} else None


def run_named(cmd, cwd, kind, name, limit):
    """Run one named test; returns 'pass' / 'fail' and the output, or raises Refused.
    'pass' needs the test's own PASS/ok line AND exit 0; 'fail' its FAIL/ERROR line AND a non-zero exit. A result line
    that the exit code contradicts (a TestMain that exits 0 anyway, say), or no result line at all (a TestMain that
    never calls m.Run), is a refusal."""
    try:
        proc = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=limit + 5)
    except subprocess.TimeoutExpired:
        raise Refused("timed out (a timeout is not a kill)")
    out = proc.stdout + proc.stderr
    if kind == "go":
        if "[build failed]" in out or "[setup failed]" in out:
            raise Refused("does not build:\n" + out[-300:])
        state = go_named(out, name)
    else:
        state = py_named(out, name)
    if state is None:
        raise Refused("test %s did not run (rc %d):\n%s" % (name, proc.returncode, out[-300:]))
    if (state == "pass") != (proc.returncode == 0):
        raise Refused("test %s reported %s but the command exited %d" % (name, state, proc.returncode))
    return state, out


def go_package(spec):
    src = spec["source"].rstrip("/")
    if not src.startswith("desktop/"):
        raise SystemExit("Go module %s: source must be under desktop/" % spec["name"])
    return "./" + src[len("desktop/"):]


def run_mutant(root, spec, mods, m, limit):
    """(killed, detail). Raises Refused for anything that is neither a clear kill nor a clear survival."""
    if not all(isinstance(m.get(k), str) for k in ("file", "old", "new", "killed_by")) or not (m["file"] and m["old"] and m["killed_by"]):
        raise Refused("mutant needs file, old, new (may be empty) and killed_by")
    target = os.path.join(root, m["file"])
    if not inside(target, os.path.join(root, spec["source"])) or not under(m["file"], spec["source"]):
        raise Refused("%s is outside module source %s" % (m["file"], spec["source"]))
    if m["old"] == m["new"]:
        raise Refused("the mutation changes nothing (old == new)")
    with open(target, encoding="utf-8") as f:
        text = f.read()
    count = text.count(m["old"])
    if count != 1:
        raise Refused("old text occurs %d times in %s (must be exactly once)" % (count, m["file"]))
    mutated = text.replace(m["old"], m["new"], 1)
    name = m["killed_by"]
    with tempfile.TemporaryDirectory(prefix="raidtest-") as tmp:
        if spec["kind"] == "go":
            base = ["go", "-C", "desktop", "test", "-count=1", "-v", "-run", "^(" + name + ")$", go_package(spec)]
            state, _ = run_named(base, root, "go", name, limit)
            if state != "pass":
                raise Refused("baseline: %s fails on clean code" % name)
            swap = os.path.join(tmp, os.path.basename(target))
            with open(swap, "w", encoding="utf-8") as f:
                f.write(mutated)
            overlay = os.path.join(tmp, "overlay.json")
            with open(overlay, "w", encoding="utf-8") as f:
                json.dump({"Replace": {os.path.realpath(target): swap}}, f)
            cmd = base[:5] + ["-overlay", overlay] + base[5:]
            state, out = run_named(cmd, root, "go", name, limit)
        else:
            # the module, its declared dependencies (transitively) and the shared support code
            need, seen = [spec], set()
            rels = {"test/raid/_client", "tools"}
            while need:
                sp = need.pop()
                if sp["name"] in seen:
                    continue
                seen.add(sp["name"])
                rels.update({sp["source"].rstrip("/"), sp["dir"].rstrip("/")})
                need.extend(mods[d] for d in sp["depends"] if d in mods)
            for rel in rels:
                src = os.path.join(root, rel)
                dst = os.path.join(tmp, rel)
                if os.path.isdir(src):
                    shutil.copytree(src, dst, dirs_exist_ok=True)
                elif os.path.isfile(src):
                    os.makedirs(os.path.dirname(dst), exist_ok=True)
                    shutil.copy2(src, dst)
            cmd = list(spec["test"]) + ["-v", "-k", name]
            state, _ = run_named(cmd, tmp, spec["kind"], name, limit)
            if state != "pass":
                raise Refused("baseline: %s fails on clean code (in the mutant copy)" % name)
            if m["file"].endswith(".py"):
                try:
                    compile(mutated, m["file"], "exec")
                except SyntaxError as e:
                    raise Refused("the mutant does not compile: %s" % e)
            with open(os.path.join(tmp, m["file"]), "w", encoding="utf-8") as f:
                f.write(mutated)
            state, out = run_named(cmd, tmp, spec["kind"], name, limit)
    return state == "fail", out[-400:]


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
                killed, detail = run_mutant(root, spec, mods, m, args.limit)
            except (Refused, OSError) as e:
                print("REFUSED %s: %s" % (label, e))
                failed += 1
                continue
            print("%s %s %.1fs" % ("KILLED" if killed else "SURVIVED", label, time.monotonic() - start))
            if not killed:
                failed += 1
                print(detail)
        if not mutants:
            print("RESULT: no mutants")
            return 0
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
