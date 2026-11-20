"""Tests of tools/raidtest.py, each on a throwaway tree in a temporary directory.

  python -I -B test/raid/_runner/test_raidtest.py
"""
import hashlib
import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

RUNNER = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..", "tools", "raidtest.py"))


def runner_module():
    spec = importlib.util.spec_from_file_location("raidtest", RUNNER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

PASSING = """import unittest
import pathlib
class T(unittest.TestCase):
    def test_value(self):
        src = pathlib.Path("{src}").read_text()
        self.assertIn("VALUE = 1", src)
unittest.main()
"""


class Tree:
    def __init__(self, tmp):
        self.root = tmp

    def write(self, rel, text):
        path = os.path.join(self.root, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        return path

    def module(self, name, source, depends=(), test=None, mutants=None):
        test = test or [sys.executable, "-I", "-B", "test/raid/%s/t.py" % name]
        self.write("test/raid/%s/module.json" % name,
                   json.dumps({"kind": "py", "source": source, "test": test, "depends": list(depends)}))
        if mutants is not None:
            self.write("test/raid/%s/mutants.json" % name, json.dumps(mutants))

    def run(self, *args):
        p = subprocess.run([sys.executable, "-I", "-B", RUNNER, "--root", self.root] + list(args),
                           capture_output=True, text=True, timeout=120)
        return p.returncode, p.stdout + p.stderr

    def git(self, *args):
        subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"] + list(args),
                       cwd=self.root, check=True, capture_output=True)


def three_modules(t):
    for name, src in (("a", "lib/a/"), ("b", "lib/b/"), ("c", "lib/c/")):
        t.write(src + "x.py", "VALUE = 1\n")
        t.write("test/raid/%s/t.py" % name, PASSING.format(src=src + "x.py"))
    t.module("a", "lib/a/")
    t.module("b", "lib/b/", depends=["a"])
    t.module("c", "lib/c/")


def sha(path):
    with open(path, "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()


GO_MOD = "module github.com/thegamersvibe/relax-coach/desktop\n\ngo 1.21\n"
GO_SRC = "package probe\n\nfunc Value() int { return 1 }\n"
GO_TEST = """package probe

import (
	"testing"
	"time"
)

func TestValue(t *testing.T) {
	if Value() != 1 {
		t.Fatal("value")
	}
}

func TestSlow(t *testing.T) { time.Sleep(8 * time.Second) }

func TestRed(t *testing.T) { t.Fatal("red on clean code") }
"""


def go_module(t, mutants, test_src=GO_TEST):
    """A Go module 'g' over desktop/internal/raid/probe/ in a throwaway tree. Go must be on PATH: no skip."""
    if not shutil.which("go"):
        raise AssertionError("go is not on PATH: the Go runner cases cannot run (they do not skip)")
    t.write("desktop/go.mod", GO_MOD)
    t.write("desktop/internal/raid/probe/probe.go", GO_SRC)
    if test_src is not None:
        t.write("desktop/internal/raid/probe/probe_test.go", test_src)
    t.write("test/raid/g/module.json", json.dumps({"kind": "go", "source": "desktop/internal/raid/probe/",
        "test": ["go", "-C", "desktop", "test", "-count=1", "./internal/raid/probe/"], "depends": []}))
    t.write("test/raid/g/mutants.json", json.dumps(mutants))


class RunnerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="raidtest-tree-")
        self.addCleanup(self.tmp.cleanup)
        self.t = Tree(self.tmp.name)

    def test_go_root_package_mutant(self):
        self.t.write("desktop/go.mod", GO_MOD)
        self.t.write("desktop/probe.go", GO_SRC)
        self.t.write("desktop/probe_test.go", GO_TEST)
        self.t.write("test/raid/g/module.json", json.dumps({
            "kind": "go", "source": "desktop/",
            "test": ["go", "-C", "desktop", "test", "-run", "^TestValue$", "."]}))
        self.t.write("test/raid/g/mutants.json", json.dumps([{
            "file": "desktop/probe.go", "old": "return 1", "new": "return 2",
            "killed_by": "TestValue"}]))
        rc, out = self.t.run("--mutants", "g")
        self.assertEqual(rc, 0, out)
        self.assertIn("RESULT: 1/1 mutants killed", out)

    def test_finds_modules(self):
        three_modules(self.t)
        self.t.write("test/raid/notamodule/readme.txt", "no module.json here\n")
        rc, out = self.t.run("--all", "--list")
        self.assertEqual(rc, 0, out)
        self.assertEqual(out.split(), ["a", "b", "c"])
        rc, out = self.t.run("--all")
        self.assertEqual(rc, 0, out)
        self.assertRegex(out, r"ok\s+a\s+\d+\.\ds")
        self.assertIn("RESULT: 3/3 modules passed", out)

    def test_changed_selection(self):
        three_modules(self.t)
        self.t.git("init", "-q")
        self.t.git("add", "-A")
        self.t.git("commit", "-qm", "base")
        self.t.write("lib/a/x.py", "VALUE = 1\n# changed\n")
        rc, out = self.t.run("--changed", "HEAD", "--list")
        self.assertEqual(rc, 0, out)
        self.assertEqual(out.split(), ["a", "b"])

    def test_data_inputs_select_owner_and_dependents(self):
        three_modules(self.t)
        path = os.path.join(self.t.root, "test/raid/a/module.json")
        with open(path, encoding="utf-8") as f:
            spec = json.load(f)
        spec["inputs"] = ["rules/raid/"]
        self.t.write("test/raid/a/module.json", json.dumps(spec))
        self.t.write("rules/raid/scoring.json", '{"value":1}')
        self.t.git("init", "-q")
        self.t.git("add", "-A")
        self.t.git("commit", "-qm", "base")
        self.t.write("rules/raid/scoring.json", '{"value":2}')
        rc, out = self.t.run("--changed", "HEAD", "--list")
        self.assertEqual(rc, 0, out)
        self.assertEqual(out.split(), ["a", "b"])

    def test_unclaimed_raid_data_selects_all(self):
        three_modules(self.t)
        self.t.git("init", "-q")
        self.t.git("add", "-A")
        self.t.git("commit", "-qm", "base")
        self.t.write("rules/raid/new-file.json", "{}")
        rc, out = self.t.run("--changed", "HEAD", "--list")
        self.assertEqual(rc, 0, out)
        self.assertEqual(out.split(), ["a", "b", "c"])

    def test_unplaced_file_selects_all(self):
        three_modules(self.t)
        self.t.git("init", "-q")
        self.t.git("add", "-A")
        self.t.git("commit", "-qm", "base")
        self.t.write("desktop/internal/raid/unclaimed.go", "package raid\n")
        rc, out = self.t.run("--changed", "HEAD", "--list")
        self.assertEqual(rc, 0, out)
        self.assertEqual(out.split(), ["a", "b", "c"])

    def test_mutant_outside_module_refused(self):
        three_modules(self.t)
        other = os.path.join(self.t.root, "lib/c/x.py")
        before = sha(other)
        self.t.module("a", "lib/a/", mutants=[{"file": "lib/c/x.py", "old": "VALUE = 1", "new": "VALUE = 2", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("REFUSED", out)
        self.assertIn("outside module source", out)
        self.assertEqual(sha(other), before)

    def test_mutant_old_twice_refused(self):
        three_modules(self.t)
        self.t.write("lib/a/x.py", "VALUE = 1\nVALUE = 1\n")
        self.t.module("a", "lib/a/", mutants=[{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = 2", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("occurs 2 times", out)

    def test_mutant_survives(self):
        three_modules(self.t)
        self.t.module("a", "lib/a/", mutants=[{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = 1  # same", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("SURVIVED a #1 (test_value)", out)

    def test_product_untouched(self):
        three_modules(self.t)
        target = os.path.join(self.t.root, "lib/a/x.py")
        before = sha(target)
        self.t.module("a", "lib/a/", mutants=[{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = 2", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 0, out)
        self.assertIn("KILLED a #1", out)
        self.assertEqual(sha(target), before)

    def test_slow_module_fails(self):
        self.t.write("lib/s/x.py", "VALUE = 1\n")
        self.t.write("test/raid/s/t.py", "import time\ntime.sleep(2)\n")
        self.t.module("s", "lib/s/")
        rc, out = self.t.run("s", "--limit", "1")
        self.assertEqual(rc, 1, out)
        self.assertIn("slower than the 1s limit", out)

    def test_module_limit_override_and_default(self):
        runner = runner_module()
        spec = {"kind": "py", "test": ["synthetic-command"]}
        proc = subprocess.CompletedProcess(spec["test"], 0, "Ran 1 test\nOK\n", "")
        for override, elapsed, expected, limit in ((None, 40, False, 30),
                                                   (60, 40, True, 60),
                                                   (60, 60, True, 60),
                                                   (60, 61, False, 60)):
            with self.subTest(override=override, elapsed=elapsed):
                current = dict(spec)
                if override is not None:
                    current["limit"] = override
                with mock.patch.object(runner.subprocess, "run", return_value=proc) as run, \
                     mock.patch.object(runner.time, "monotonic", side_effect=[0, elapsed]):
                    ok, seconds, why, output = runner.run_module(self.t.root, current, 30)
                self.assertEqual(ok, expected, why)
                self.assertEqual(seconds, elapsed)
                self.assertEqual(run.call_args.kwargs["timeout"], limit + 5)

    def test_module_limit_validation_and_cfg_only_override(self):
        runner = runner_module()
        self.t.module("a", "lib/a/")
        path = "test/raid/a/module.json"
        with open(os.path.join(self.t.root, path), encoding="utf-8") as f:
            spec = json.load(f)
        for value in (0, -1, True, None, "60", [], float("inf"), float("nan")):
            with self.subTest(value=value):
                self.t.write(path, json.dumps(dict(spec, limit=value)))
                with self.assertRaisesRegex(SystemExit, "limit"):
                    runner.load_modules(self.t.root)
        self.t.write(path, json.dumps(dict(spec, limit=60)))
        self.assertEqual(runner.load_modules(self.t.root)["a"]["limit"], 60)
        root = os.path.dirname(os.path.dirname(RUNNER))
        modules = runner.load_modules(root)
        self.assertEqual({name: s["limit"] for name, s in modules.items() if "limit" in s}, {"cfg": 60})

    def test_failing_module_fails(self):
        self.t.write("lib/f/x.py", "VALUE = 2\n")
        self.t.write("test/raid/f/t.py", PASSING.format(src="lib/f/x.py"))
        self.t.module("f", "lib/f/")
        rc, out = self.t.run("f")
        self.assertEqual(rc, 1, out)
        self.assertRegex(out, r"FAIL\s+f\s")

    # Review round 1: a mutant is KILLED only when its named test ran and failed; everything else is a refusal.
    def one_py_module(self, mutants, test=PASSING):
        self.t.write("lib/a/x.py", "VALUE = 1\n")
        self.t.write("test/raid/a/t.py", test.format(src="lib/a/x.py"))
        self.t.module("a", "lib/a/", mutants=mutants)

    def test_mutant_with_missing_test_refused(self):
        self.one_py_module([{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = 2", "killed_by": "test_does_not_exist"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("REFUSED", out)
        self.assertIn("did not run", out)
        self.assertNotIn("KILLED", out)

    def test_equivalent_mutant_survives(self):
        self.one_py_module([{"file": "lib/a/x.py", "old": "VALUE = 1\n", "new": "VALUE = 1  # same\n", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("SURVIVED a #1", out)

    def test_no_op_mutant_refused(self):
        self.one_py_module([{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = 1", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("changes nothing", out)

    def test_pre_red_test_refused(self):
        red = PASSING.replace('self.assertIn("VALUE = 1", src)', 'self.assertIn("VALUE = 9", src)')
        self.one_py_module([{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = 2", "killed_by": "test_value"}], test=red)
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("baseline", out)
        self.assertNotIn("KILLED", out)

    def test_real_kill_and_deletion(self):
        self.one_py_module([{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = 2", "killed_by": "test_value"},
                            {"file": "lib/a/x.py", "old": "VALUE = 1\n", "new": "", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 0, out)
        self.assertIn("RESULT: 2/2 mutants killed", out)

    def test_declared_dependency_copied(self):
        both = PASSING.replace('src = pathlib.Path("{src}").read_text()',
                               'src = pathlib.Path("{src}").read_text() + pathlib.Path("lib/a/x.py").read_text()')
        self.t.write("lib/a/x.py", "VALUE = 1\n")
        self.t.write("lib/b/y.py", "OTHER = 0\nVALUE = 1\n")
        self.t.write("test/raid/a/t.py", PASSING.format(src="lib/a/x.py"))
        self.t.write("test/raid/b/t.py", both.format(src="lib/b/y.py"))
        self.t.module("a", "lib/a/")
        # b's test needs a's file; the mutation leaves the test green: a clean SURVIVED, not a FileNotFoundError kill
        self.t.module("b", "lib/b/", depends=["a"],
                      mutants=[{"file": "lib/b/y.py", "old": "OTHER = 0", "new": "OTHER = 5", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "b")
        self.assertIn("SURVIVED b #1", out)
        self.assertNotIn("KILLED", out)

    def test_dotdot_escape_refused(self):
        self.t.write("lib/a/x.py", "VALUE = 1\n")
        self.t.write("lib/b/x.py", "VALUE = 1\n")
        self.t.write("test/raid/a/t.py", PASSING.format(src="lib/b/x.py"))
        self.t.module("a", "lib/a/", mutants=[{"file": "lib/a/../b/x.py", "old": "VALUE = 1", "new": "VALUE = 2", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("outside module source", out)

    def test_symlink_escape_refused(self):
        self.t.write("lib/b/x.py", "VALUE = 1\n")
        os.makedirs(os.path.join(self.t.root, "lib", "a"))
        os.symlink(os.path.join(self.t.root, "lib", "b", "x.py"), os.path.join(self.t.root, "lib", "a", "x.py"))
        self.t.write("test/raid/a/t.py", PASSING.format(src="lib/a/x.py"))
        self.t.module("a", "lib/a/", mutants=[{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = 2", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("outside module source", out)

    def test_module_running_no_tests_fails(self):
        self.t.write("lib/a/x.py", "VALUE = 1\n")
        self.t.write("test/raid/a/t.py", "import unittest\nunittest.main()\n")
        self.t.module("a", "lib/a/")
        rc, out = self.t.run("a")
        self.assertEqual(rc, 1, out)
        self.assertRegex(out, r"FAIL\s+a\s")

    def test_empty_mutants_is_no_mutants(self):
        self.one_py_module([])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 0, out)
        self.assertIn("RESULT: no mutants", out)
        self.assertNotIn("0/0", out)

    def test_go_kill_and_refusals(self):
        go_module(self.t, [
            {"file": "desktop/internal/raid/probe/probe.go", "old": "return 1", "new": "return 2", "killed_by": "TestValue"},
            {"file": "desktop/internal/raid/probe/probe.go", "old": "return 1", "new": "return \"x\"", "killed_by": "TestValue"},
            {"file": "desktop/internal/raid/probe/probe.go", "old": "return 1", "new": "return 2", "killed_by": "TestMissing"},
            {"file": "desktop/internal/raid/probe/probe.go", "old": "return 1", "new": "return 2", "killed_by": "TestRed"},
            {"file": "desktop/internal/raid/probe/probe.go", "old": "return 1", "new": "return 2", "killed_by": "["},
            {"file": "desktop/internal/raid/probe/probe.go", "old": "return 1", "new": "return 2", "killed_by": "TestSlow"},
            {"file": "desktop/internal/raid/probe/../probe/probe.go", "old": "return 1", "new": "return 2", "killed_by": "TestValue"},
        ])
        rc, out = self.t.run("--mutants", "g", "--limit", "0.1")
        self.assertEqual(rc, 1, out)
        lines = [l for l in out.splitlines() if l.startswith(("KILLED", "SURVIVED", "REFUSED"))]
        self.assertEqual([l.split()[0] for l in lines], ["KILLED"] + ["REFUSED"] * 5 + ["KILLED"], out)
        self.assertIn("does not build", lines[1])
        self.assertIn("did not run", lines[2])
        self.assertIn("baseline", lines[3])
        self.assertIn("timed out", lines[5])
        self.assertIn("RESULT: 2/7 mutants killed", out)
        self.assertEqual(sha(os.path.join(self.t.root, "desktop/internal/raid/probe/probe.go")),
                         hashlib.sha256(GO_SRC.encode()).hexdigest())

    def test_go_module_without_tests_fails(self):
        go_module(self.t, [], test_src=None)
        rc, out = self.t.run("g")
        self.assertEqual(rc, 1, out)
        self.assertIn("ran no tests", out)
        go_module(self.t, [], test_src="package probe\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) {}\n")
        rc, out = self.t.run("g")
        self.assertEqual(rc, 0, out)


    def test_go_result_line_and_exit_code_must_agree(self):
        # a TestMain that runs the tests but always exits 0: the FAIL line is there, the exit says pass -> refused;
        # a TestMain that never calls m.Run: no result line -> refused
        exit0 = GO_TEST.replace('import (\n\t"testing"\n\t"time"\n)', 'import (\n\t"os"\n\t"testing"\n\t"time"\n)') + \
            "\nfunc TestMain(m *testing.M) { m.Run(); os.Exit(0) }\n"
        go_module(self.t, [{"file": "desktop/internal/raid/probe/probe.go", "old": "return 1", "new": "return 2", "killed_by": "TestValue"}],
                  test_src=exit0)
        rc, out = self.t.run("--mutants", "g")
        self.assertEqual(rc, 1, out)
        self.assertIn("REFUSED", out)
        self.assertIn("exited 0", out)
        self.assertNotIn("KILLED", out)
        norun = GO_TEST + "\nfunc TestMain(m *testing.M) {}\n"
        go_module(self.t, [{"file": "desktop/internal/raid/probe/probe.go", "old": "return 1", "new": "return 2", "killed_by": "TestValue"}],
                  test_src=norun)
        rc, out = self.t.run("--mutants", "g")
        self.assertEqual(rc, 1, out)
        self.assertIn("did not run", out)
        self.assertNotIn("KILLED", out)

    def test_py_result_line_and_exit_code_must_agree(self):
        # the test fails but the script exits 0 anyway
        swallow = PASSING.replace("unittest.main()", "unittest.main(exit=False)")
        self.one_py_module([{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = 2", "killed_by": "test_value"}], test=swallow)
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("exited 0", out)
        self.assertNotIn("KILLED", out)

    def test_py_mutant_that_does_not_compile_refused(self):
        self.one_py_module([{"file": "lib/a/x.py", "old": "VALUE = 1", "new": "VALUE = (", "killed_by": "test_value"}])
        rc, out = self.t.run("--mutants", "a")
        self.assertEqual(rc, 1, out)
        self.assertIn("does not compile", out)
        self.assertNotIn("KILLED", out)


RULES = os.path.join(os.path.dirname(RUNNER), "..", "desktop", "internal", "raid")


class RulesTests(unittest.TestCase):
    """The structure rules (rules_test.go) red on planted files, in a throwaway copy of the raid tree."""

    def setUp(self):
        if not shutil.which("go"):
            raise AssertionError("go is not on PATH: the rules cases cannot run (they do not skip)")
        self.tmp = tempfile.TemporaryDirectory(prefix="raidrules-")
        self.addCleanup(self.tmp.cleanup)
        self.t = Tree(self.tmp.name)
        self.t.write("desktop/go.mod", GO_MOD)
        for rel in ("rules_test.go", "imports.json"):
            with open(os.path.join(RULES, rel), encoding="utf-8") as f:
                self.t.write("desktop/internal/raid/" + rel, f.read())
        self.t.write("desktop/internal/raid/imports.json", json.dumps({"logfmt": [], "fixture": []}))
        self.t.write("desktop/internal/raid/logfmt/a.go", "package logfmt\n")
        self.t.write("desktop/internal/raid/fixture/a.go", "package fixture\n")

    def rules(self):
        p = subprocess.run(["go", "-C", os.path.join(self.t.root, "desktop"), "test", "-count=1", "./internal/raid/"],
                           capture_output=True, text=True, timeout=120)
        return p.returncode, p.stdout + p.stderr

    def test_clean_tree_passes(self):
        rc, out = self.rules()
        self.assertEqual(rc, 0, out)

    def test_windows_only_import_checked_on_any_host(self):
        self.t.write("desktop/internal/raid/logfmt/a_windows.go",
                     "//go:build windows\n\npackage logfmt\n\nimport _ \"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture\"\n")
        rc, out = self.rules()
        self.assertEqual(rc, 1, out)
        self.assertIn("logfmt imports fixture, which imports.json does not allow", out)

    def test_number_literals_above_999(self):
        red = (("1000", "1000"), ("1e3", "1000"), ("1000.0", "1000"), ("999 + 1", "1000"), ("-(2 * 600)", "-1200"),
               ("0x3E8", "1000"), ("1_000", "1000"), ("1 << 10", "1024"), ("1000 * n", "1000"),
               ("12345 / 100", "12345"), ("25000 - 24999", "25000"))
        for k, (src, _) in enumerate(red):  # one file per case, so each finding names its own case
            self.t.write("desktop/internal/raid/logfmt/r%d.go" % k, "package logfmt\n\nvar x%d = %s\n" % (k, src))
        self.t.write("desktop/internal/raid/logfmt/n.go", "package logfmt\n\nvar n = 1\n")
        rc, out = self.rules()
        self.assertEqual(rc, 1, out)
        for k, (src, shown) in enumerate(red):
            with self.subTest(src=src):
                self.assertIn("logfmt/r%d.go:3 %s\n" % (k, shown), out)
        for k in range(len(red)):
            os.remove(os.path.join(self.t.root, "desktop/internal/raid/logfmt/r%d.go" % k))
        green = ("999", "0.5", "999.0", "500 + 499", "999 * n", "-999")
        self.t.write("desktop/internal/raid/logfmt/g.go",
                     "package logfmt\n\n" + "".join("var g%d = %s\n" % (k, src) for k, src in enumerate(green)))
        rc, out = self.rules()
        self.assertEqual(rc, 0, out)


if __name__ == "__main__":
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "_client"))
    from partition import main
    main(sys.modules[__name__], {
        "unit": None,
        "go_refusals": ("test_go_kill_and_refusals",),
        "go_exit": ("test_go_result_line_and_exit_code_must_agree",),
    })
