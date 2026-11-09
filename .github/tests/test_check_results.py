"""Exercise the exact CI gate process, including skipped and absent jobs."""
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest
from typing import Any

GATE = Path(__file__).resolve().parents[1] / "scripts" / "check_results.py"
JOBS = ("coach-lua", "desktop-go", "bundle-contract")


class RequiredCITests(unittest.TestCase):
    def run_gate(self, payload):
        env = dict(os.environ)
        env.pop("NEEDS_JSON", None)
        if payload is not None:
            env["NEEDS_JSON"] = payload
        return subprocess.run([sys.executable, str(GATE)], env=env,
                              text=True, capture_output=True, timeout=10)

    def good(self) -> dict[str, Any]:
        return {name: {"result": "success"} for name in JOBS}

    def test_all_required_jobs_must_succeed(self):
        result = self.run_gate(json.dumps(self.good()))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Required CI passed", result.stdout)

    def test_each_non_success_result_fails(self):
        for job in JOBS:
            for state in ("failure", "skipped", "cancelled", "neutral", "", None, True):
                with self.subTest(job=job, state=state):
                    data = self.good()
                    data[job]["result"] = state
                    result = self.run_gate(json.dumps(data))
                    self.assertEqual(result.returncode, 1, result.stderr)
                    self.assertIn("Required CI failed", result.stderr)

    def test_missing_or_extra_job_fails(self):
        missing = self.good()
        del missing["coach-lua"]
        extra = self.good()
        extra["unexpected"] = {"result": "success"}
        for data in (missing, extra):
            with self.subTest(data=data):
                self.assertEqual(self.run_gate(json.dumps(data)).returncode, 1)

    def test_missing_malformed_or_nonobject_input_fails(self):
        for payload in (None, "not-json", "[]", "null", "true", '"success"', "{}"):
            with self.subTest(payload=payload):
                self.assertEqual(self.run_gate(payload).returncode, 1)

    def test_missing_or_malformed_job_result_fails(self):
        for row in ({}, None, [], "success"):
            with self.subTest(row=row):
                data = self.good()
                data["desktop-go"] = row
                self.assertEqual(self.run_gate(json.dumps(data)).returncode, 1)


if __name__ == "__main__":
    unittest.main(verbosity=2)
