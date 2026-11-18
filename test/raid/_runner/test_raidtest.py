"""Tests of tools/raidtest.py, each on a throwaway tree in a temporary directory.

  python -I -B test/raid/_runner/test_raidtest.py
"""
import hashlib
import json
import os
import subprocess
import sys
import tempfile
import unittest

RUNNER = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..", "tools", "raidtest.py"))

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


class RunnerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="raidtest-tree-")
        self.addCleanup(self.tmp.cleanup)
        self.t = Tree(self.tmp.name)

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

    def test_failing_module_fails(self):
        self.t.write("lib/f/x.py", "VALUE = 2\n")
        self.t.write("test/raid/f/t.py", PASSING.format(src="lib/f/x.py"))
        self.t.module("f", "lib/f/")
        rc, out = self.t.run("f")
        self.assertEqual(rc, 1, out)
        self.assertRegex(out, r"FAIL\s+f\s")


if __name__ == "__main__":
    unittest.main()
