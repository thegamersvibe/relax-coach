"""Real published readers and installers, no rewritten compatibility rules.
Run after the bundle-contract build; needs published tags, Go and no Lua runtime.
"""
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


class OldAppRankTests(unittest.TestCase):
    def save_accepted(self, tag):
        # Both published apps have the generic literal reader, not a rank page
        # or prep reader. Run that exact parser, never a copy of its rules.
        with tempfile.TemporaryDirectory(prefix="rank-reader-") as work:
            work = Path(work)
            source = subprocess.check_output(["git", "show", tag + ":desktop/luasv.go"], cwd=ROOT)
            (work / "luasv.go").write_bytes(source)
            shutil.copyfile(HERE / "saved_reader_probe_test.go", work / "rank_probe_test.go")
            result = subprocess.run(["go", "test", "-count=1", "-v", "luasv.go", "rank_probe_test.go"],
                                    cwd=work, env=dict(os.environ, RANK_SAVE=str(HERE / "save.lua")),
                                    text=True, capture_output=True, timeout=300)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("--- PASS: TestRankSaveAccepted ", result.stdout)
            print(tag + " save reader:\n" + result.stdout, end="")

    def bundle_accepted(self, tag):
        toc = (ROOT / "addon/ForeverCoach/ForeverCoach.toc").read_text()
        match = re.search(r"^## Version: (\d+\.\d+\.\d+)$", toc, re.M)
        self.assertIsNotNone(match, "Coach release version missing")
        assert match is not None
        version = match.group(1)
        bundle = ROOT / f"dist/RelaxCoach-RaidBundle-{version}.zip"
        self.assertTrue(bundle.is_file(), "build the current bundle before this gate")
        result = subprocess.run(["bash", "scripts/check_old_app_accepts_bundle.sh", tag, str(bundle), version],
                                cwd=ROOT, text=True, capture_output=True, timeout=600)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("--- PASS: TestOldAppAcceptsRealBundle ", result.stdout)
        print(result.stdout, end="")

    def test_app_1_8_0_accepts_save(self):
        self.save_accepted("v1.8.0")

    def test_app_1_8_1_accepts_save(self):
        self.save_accepted("v1.8.1")

    def test_app_1_8_0_accepts_bundle(self):
        self.bundle_accepted("v1.8.0")

    def test_app_1_8_1_accepts_bundle(self):
        self.bundle_accepted("v1.8.1")


if __name__ == "__main__":
    unittest.main(verbosity=2)
