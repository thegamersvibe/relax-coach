"""Exercise the real release entrypoint without building or publishing assets."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class ReleaseGateTests(unittest.TestCase):
    def run_release(self, version, *, old_app=None, gate_rc=0):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "scripts").mkdir()
            (root / "desktop").mkdir()
            (root / "bin").mkdir()
            shutil.copyfile(ROOT / "scripts/release.sh", root / "scripts/release.sh")
            # Only expensive build commands and the two probe endpoints are
            # replaced. Routing, version comparison and failure propagation
            # execute unchanged from release.sh, including inherited variables.
            stubs = {
                "bin/go": "exit 0\n",
                "bin/python3": "exit 0\n",
                "bin/git": "printf 'v1.6.0\\n'\n",
                "scripts/check_old_app_accepts_bundle.sh":
                    'printf "accept|%s|%s|%s\\n" "$1" "$2" "$3" >> "$GATE_LOG"\nexit "$GATE_RC"\n',
                "scripts/check_old_app_refuses_bundle.sh":
                    'printf "refuse|%s|%s|%s\\n" "$1" "$2" "$3" >> "$GATE_LOG"\nexit "$GATE_RC"\n',
            }
            for name, body in stubs.items():
                path = root / name
                path.write_text("#!/usr/bin/env bash\n" + body)
                path.chmod(0o755)
            env = dict(os.environ, PATH=str(root / "bin") + os.pathsep + os.environ["PATH"],
                       GATE_LOG=str(root / "gate.log"), GATE_RC=str(gate_rc))
            env.pop("OLD_APP", None)
            # No inherited mode/skip setting may turn the pinned refusal into
            # an acceptance check (or suppress either check).
            env.update(APP_FIRST_VERSIONS="1.8.0", SKIP_OLD_APP="1", OLD_APP_MODE="accept")
            if old_app is not None:
                env["OLD_APP"] = old_app
            result = subprocess.run(["bash", "scripts/release.sh", version], cwd=root,
                                    env=env, text=True, capture_output=True)
            log = (root / "gate.log").read_text().splitlines()
            return result, log

    def test_refuse_only_exact_1_7_0(self):
        for version in ("1.7.0", "1.6.1", "1.7.1", "1.8.0", "1.7",
                        "v1.7.0", "1.7.0-test", "1.7.00", "1.70.0", "1.7.0 "):
            with self.subTest(version=version):
                result, log = self.run_release(version)
                self.assertEqual(result.returncode, 0, result.stderr)
                mode = "refuse" if version == "1.7.0" else "accept"
                self.assertEqual(log, [f"{mode}|v1.6.0|dist/RelaxCoach-Bundle-{version}.zip|{version}"])

    def test_pinned_release_uses_last_tag_not_override(self):
        result, log = self.run_release("1.7.0", old_app="HEAD")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(log, ["refuse|v1.6.0|dist/RelaxCoach-Bundle-1.7.0.zip|1.7.0"])

    def test_other_versions_keep_old_app_override(self):
        result, log = self.run_release("1.7.1", old_app="v1.5.6")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(log, ["accept|v1.5.6|dist/RelaxCoach-Bundle-1.7.1.zip|1.7.1"])

    def test_refusal_probe_rejects_unpinned_versions(self):
        for version in ("1.6.1", "1.7.1", "1.8.0", "1.7", "v1.7.0", "1.7.0-test"):
            with self.subTest(version=version):
                result = subprocess.run(
                    ["bash", str(ROOT / "scripts/check_old_app_refuses_bundle.sh"),
                     "v1.6.0", "unused.zip", version],
                    cwd=ROOT, text=True, capture_output=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stderr, "Refusal gate is pinned to 1.7.0 only.\n")

    def test_both_gates_fail_closed(self):
        for version in ("1.7.0", "1.7.1"):
            with self.subTest(version=version):
                result, log = self.run_release(version, gate_rc=23)
                self.assertEqual(result.returncode, 23)
                self.assertEqual(len(log), 1)
                self.assertNotIn("Local release assets built.", result.stdout)


if __name__ == "__main__":
    unittest.main(verbosity=2)
