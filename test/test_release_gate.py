"""Exercise the real release entrypoint without building or publishing assets."""
import os
from pathlib import Path
import re
import runpy
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
import zipfile

ROOT = Path(__file__).resolve().parents[1]
RAID_RELEASE = runpy.run_path(str(ROOT / "scripts/build_guild_bundle.py"))["raid_release"]


class ReleaseGateTests(unittest.TestCase):
    def run_release(self, version, *, old_app=None, gate_rc=0, stale_asset=False, stale_sum=False):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "scripts").mkdir()
            (root / "desktop").mkdir()
            (root / "bin").mkdir()
            shutil.copyfile(ROOT / "scripts/release.sh", root / "scripts/release.sh")
            shutil.copyfile(ROOT / "scripts/build_guild_bundle.py", root / "scripts/build_guild_bundle.py")
            shutil.copyfile(ROOT / "desktop/addon_update.go", root / "desktop/addon_update.go")
            (root / "dist").mkdir()
            legacy = f"RelaxCoach-Bundle-{version}.zip"
            if stale_asset:
                (root / "dist" / legacy).write_bytes(b"stale legacy asset")
            (root / "dist/SHA256SUMS.txt").write_text(
                "0" * 64 + "  " + (legacy if stale_sum else f"RelaxCoach-RaidBundle-{version}.zip") + "\n")
            # Only expensive build commands and the two probe endpoints are
            # replaced. Routing, version comparison and failure propagation
            # execute unchanged from release.sh, including inherited variables.
            stubs = {
                "bin/go": "exit 0\n",
                "bin/python3": 'if [[ "$1" == "-B" ]]; then exec "$REAL_PYTHON" "$@"; fi\nexit 0\n',
                "bin/git": "printf 'v1.7.1\\n'\n",
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
                       GATE_LOG=str(root / "gate.log"), GATE_RC=str(gate_rc), REAL_PYTHON=sys.executable)
            env.pop("OLD_APP", None)
            # No inherited mode/skip setting may turn the pinned refusal into
            # an acceptance check (or suppress either check).
            env.update(APP_FIRST_VERSIONS="1.8.0", SKIP_OLD_APP="1", OLD_APP_MODE="accept")
            if old_app is not None:
                env["OLD_APP"] = old_app
            result = subprocess.run(["bash", "scripts/release.sh", version], cwd=root,
                                    env=env, text=True, capture_output=True)
            log = (root / "gate.log").read_text().splitlines() if (root / "gate.log").exists() else []
            return result, log

    def test_refuse_only_exact_1_8_0(self):
        for version in ("1.8.0", "1.7.0", "1.7.1", "1.7.10", "1.8.1", "2.0.0", "1.8.00"):
            with self.subTest(version=version):
                result, log = self.run_release(version)
                self.assertEqual(result.returncode, 0, result.stderr)
                mode = "refuse" if version == "1.8.0" else "accept"
                kind = "RaidBundle" if tuple(map(int,version.split('.'))) >= (1,8,0) else "Bundle"
                self.assertEqual(log, [f"{mode}|v1.7.1|dist/RelaxCoach-{kind}-{version}.zip|{version}"])

    def test_invalid_release_version_never_reaches_a_gate(self):
        for version in ("1.8", "v1.8.0", "1.8.0-test", "1.8.0 "):
            with self.subTest(version=version):
                result, log = self.run_release(version)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(log, [])

    def test_raid_release_refuses_legacy_asset_or_checksum(self):
        for kwargs in ({"stale_asset": True}, {"stale_sum": True}):
            with self.subTest(kwargs=kwargs):
                result, log = self.run_release("1.8.0", **kwargs)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(log, [])
                self.assertIn("legacy bundle", result.stderr)

    def test_pinned_release_uses_last_tag_not_override(self):
        result, log = self.run_release("1.8.0", old_app="HEAD")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(log, ["refuse|v1.7.1|dist/RelaxCoach-RaidBundle-1.8.0.zip|1.8.0"])

    def test_other_versions_keep_old_app_override(self):
        result, log = self.run_release("1.7.1", old_app="v1.5.6")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(log, ["accept|v1.5.6|dist/RelaxCoach-Bundle-1.7.1.zip|1.7.1"])

    def test_refusal_probe_rejects_unpinned_versions(self):
        for version in ("1.7.0", "1.7.1", "1.8.1", "1.8", "v1.8.0", "1.8.0-test"):
            with self.subTest(version=version):
                result = subprocess.run(
                    ["bash", str(ROOT / "scripts/check_old_app_refuses_bundle.sh"),
                     "v1.7.1", "unused.zip", version],
                    cwd=ROOT, text=True, capture_output=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stderr, "Refusal gate is pinned to 1.8.0 only.\n")

    def test_refusal_probe_pins_complete_real_raid_inventory(self):
        source=(ROOT/'scripts/old_app_refusal_test.go').read_text()
        self.assertIn('const oldAppVersion = "1.8.0"',source)
        match=re.search(r'var oldAppRaidFiles = \[\]string\{([^}]+)\}',source)
        assert match is not None,'reviewed raid path inventory missing'
        paths=re.findall(r'"([^"]+)"',match.group(1))
        actual=sorted('Interface/AddOns/RelaxRaid/'+p.relative_to(ROOT/'addon/RelaxRaid').as_posix()
                      for p in (ROOT/'addon/RelaxRaid').rglob('*') if p.is_file())
        self.assertEqual(sorted(paths),actual)
        self.assertIn('No compatible new addon update is available. This is an app-only release for this updater;',source)
        self.assertIn('delete(m, "raidVersion")',source)

    def test_refusal_probe_first_file_matches_bundle_order(self):
        source=(ROOT/'scripts/old_app_refusal_test.go').read_text()
        first=min('Interface/AddOns/RelaxRaid/'+p.relative_to(ROOT/'addon/RelaxRaid').as_posix()
                  for p in (ROOT/'addon/RelaxRaid').rglob('*') if p.is_file())
        self.assertIn('const oldAppUnknownFile = "archive path is outside the fixed guild allowlist: '+first+'"',source)
        self.assertIn('strings.ReplaceAll(oldAppFallbackError, "'+first.removeprefix('Interface/AddOns/')+'",',source)

    def test_refusal_script_requires_every_named_pass(self):
        # Exercise the real shell wrapper with MOCKED Go output, not a 1.8.0
        # compatibility run. Archive the local published tag only for staging.
        names=['TestOldAppRefusalAssertions','TestOldAppRefusalReplacementExecutable',
               'TestOldAppRefusalStrippedBundleInstalls','TestOldAppRefusesRealBundle']
        names += ['TestOldAppRefusesRealBundle/prior-backup='+prior+'/'+route
                  for prior in ('true','false') for route in ('app-first','installer-fallback')]
        gitdir=subprocess.check_output(['git','rev-parse','--absolute-git-dir'],cwd=ROOT,text=True).strip()
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp)
            for folder in ('scripts','dist','bin'):(root/folder).mkdir()
            for name in ('check_old_app_refuses_bundle.sh','old_app_refusal_test.go'):
                shutil.copy2(ROOT/'scripts'/name,root/'scripts'/name)
            (root/'dist/RelaxCoach.exe').write_bytes(b'MZmock; never validated by Go')
            (root/'dist/bundle.zip').write_bytes(b'mock; never opened by Go')
            stub=root/'bin/go'
            stub.write_text('#!/usr/bin/env bash\n[[ "$RC_VERSION" == "1.8.0" ]] || exit 41\n'
                            'while IFS= read -r name; do\n'
                            '  [[ "$name" == "$MOCK_SKIP" ]] || printf -- "--- PASS: %s (0.00s)\\n" "$name"\n'
                            'done < "$MOCK_NAMES"\nexit 0\n')
            stub.chmod(0o755)
            (root/'names').write_text('\n'.join(names)+'\n')
            for skip in ('',*names):
                with self.subTest(skipped=skip):
                    env=dict(os.environ,GIT_DIR=gitdir,PATH=str(root/'bin')+os.pathsep+os.environ['PATH'],
                             MOCK_SKIP=skip,MOCK_NAMES=str(root/'names'))
                    result=subprocess.run(['bash','scripts/check_old_app_refuses_bundle.sh','v1.7.1',
                                           'dist/bundle.zip','1.8.0'],cwd=root,env=env,capture_output=True,text=True)
                    if skip:
                        self.assertNotEqual(result.returncode,0)
                        self.assertIn('missing PASS for '+skip,result.stderr)
                    else:self.assertEqual(result.returncode,0,result.stdout+result.stderr)

    def test_bundle_ci_fetches_tags_for_real_old_app_probe(self):
        workflow = (ROOT / '.github/workflows/ci.yml').read_text()
        bundle_job = workflow.split('  bundle-contract:', 1)[1].split('  required-ci:', 1)[0]
        self.assertIn('fetch-depth: 0', bundle_job, 'the real old-app gate needs published tag history in CI')

    def test_both_gates_fail_closed(self):
        for version in ("1.8.0", "1.7.1"):
            with self.subTest(version=version):
                result, log = self.run_release(version, gate_rc=23)
                self.assertEqual(result.returncode, 23)
                self.assertEqual(len(log), 1)
                self.assertNotIn("Local release assets built.", result.stdout)


class RealGateSetupTests(unittest.TestCase):
    def test_real_gate_selects_raid_asset_at_and_above_floor(self):
        for version in ('1.8.0','1.8.1'):
            with self.subTest(version=version), tempfile.TemporaryDirectory() as temp:
                root=Path(temp)
                (root/'addon/ForeverCoach').mkdir(parents=True)
                (root/'addon/ForeverCoach/ForeverCoach.toc').write_text(f'## Version: {version}\n')
                (root/'dist').mkdir()
                (root/'dist/RelaxCoach.exe').touch()
                bundle=root/f'dist/RelaxCoach-RaidBundle-{version}.zip'
                bundle.touch()
                case=RealRefusalGateTests if version=='1.8.0' else RealAcceptanceGateTests
                with mock.patch.object(sys.modules[__name__],'ROOT',root), \
                        mock.patch.dict(os.environ,{'CI':'true'}), \
                        mock.patch.object(subprocess,'check_output',return_value='mock-git-result\n'):
                    case.setUpClass()
                    self.assertEqual(case.bundle,bundle)

    def test_discovery_runs_the_current_release_gate_not_a_stale_bundle(self):
        def test_ids(suite):
            for test in suite:
                if isinstance(test, unittest.TestSuite):
                    yield from test_ids(test)
                else:
                    yield test.id()

        for version in ('1.8.0', '1.8.1', '1.9.0'):
            with self.subTest(version=version), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                (root / 'addon/ForeverCoach').mkdir(parents=True)
                (root / 'addon/ForeverCoach/ForeverCoach.toc').write_text(f'## Version: {version}\n')
                with mock.patch.object(sys.modules[__name__], 'ROOT', root):
                    suite = unittest.defaultTestLoader.loadTestsFromModule(sys.modules[__name__])
                real_ids = [name for name in test_ids(suite)
                            if '.RealRefusalGateTests.' in name or '.RealAcceptanceGateTests.' in name]
                selected = 'RealRefusalGateTests' if version == '1.8.0' else 'RealAcceptanceGateTests'
                self.assertTrue(real_ids, 'CI must collect a real gate')
                self.assertTrue(all(f'.{selected}.' in name for name in real_ids), real_ids)

    def test_missing_release_assets_fail_in_ci(self):
        for marker, version in ((m, v) for m in ('CI', 'GITHUB_ACTIONS')
                                for v in ('1.8.0', '1.7.1')):
            for missing in ('exe', 'bundle', 'both'):
                with self.subTest(marker=marker, version=version, missing=missing), tempfile.TemporaryDirectory() as temp:
                    root = Path(temp)
                    (root / 'dist').mkdir()
                    (root / 'addon/ForeverCoach').mkdir(parents=True)
                    (root / 'addon/ForeverCoach/ForeverCoach.toc').write_text(f'## Version: {version}\n')
                    if missing == 'exe':
                        kind = 'RaidBundle' if RAID_RELEASE(version) else 'Bundle'
                        (root / f'dist/RelaxCoach-{kind}-{version}.zip').touch()
                    if missing == 'bundle':
                        (root / 'dist/RelaxCoach.exe').touch()
                    with mock.patch.dict(os.environ, {marker: 'true'}, clear=True), \
                            mock.patch.object(sys.modules[__name__], 'ROOT', root):
                        try:
                            with self.assertRaisesRegex(AssertionError, 'release assets missing'):
                                case = RealRefusalGateTests if version == '1.8.0' else RealAcceptanceGateTests
                                case.setUpClass()
                        except unittest.SkipTest as exc:
                            self.fail(f'CI must fail, not skip: {exc}')

    def test_missing_release_assets_may_skip_outside_ci(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'addon/ForeverCoach').mkdir(parents=True)
            for version in ('1.8.0', '1.7.1'):
                (root / 'addon/ForeverCoach/ForeverCoach.toc').write_text(f'## Version: {version}\n')
                with (self.subTest(version=version),
                      mock.patch.dict(os.environ, {}, clear=True),
                      mock.patch.object(sys.modules[__name__], 'ROOT', root)):
                    with self.assertRaisesRegex(unittest.SkipTest, 'release assets missing'):
                        case = RealRefusalGateTests if version == '1.8.0' else RealAcceptanceGateTests
                        case.setUpClass()


def release_version():
    # The bundle contract also asserts that both build/release defaults match.
    toc = (ROOT / 'addon/ForeverCoach/ForeverCoach.toc').read_text()
    match = re.search(r'^## Version: (\d+\.\d+\.\d+)$', toc, re.M)
    if match is None:
        raise AssertionError('Coach release version missing or malformed')
    return match.group(1)


class RealGateAssets:
    @classmethod
    def setUpClass(cls):
        cls.version = release_version()
        cls.exe = ROOT / 'dist/RelaxCoach.exe'
        kind = 'RaidBundle' if RAID_RELEASE(cls.version) else 'Bundle'
        cls.bundle = ROOT / f'dist/RelaxCoach-{kind}-{cls.version}.zip'
        missing = [p.name for p in (cls.exe, cls.bundle) if not p.is_file()]
        if missing:
            message = 'release assets missing: ' + ', '.join(missing) + '; build before running the real old-app gate'
            if os.environ.get('CI') or os.environ.get('GITHUB_ACTIONS'):
                raise AssertionError(message)
            raise unittest.SkipTest(message)
        # Match release.sh: only 1.8.0 excludes its own published tag.
        describe = ['git', 'describe', '--tags', '--abbrev=0', '--match', 'v*']
        if cls.version == '1.8.0':
            describe += ['--exclude', 'v1.8.0']
        cls.old = subprocess.check_output(describe + ['HEAD'], cwd=ROOT, text=True).strip()
        cls.gitdir = subprocess.check_output(['git', 'rev-parse', '--absolute-git-dir'], cwd=ROOT, text=True).strip()


class RealRefusalGateTests(RealGateAssets, unittest.TestCase):
    """Only 1.8.0 is app-first; preserve its real old-code negative controls."""

    def run_gate(self, *, corrupt_exe=False, extra_path=False):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'scripts').mkdir()
            (root / 'dist').mkdir()
            for name in ('check_old_app_refuses_bundle.sh', 'old_app_refusal_test.go'):
                shutil.copy2(ROOT / 'scripts' / name, root / 'scripts' / name)
            exe = bytearray(self.exe.read_bytes())
            if corrupt_exe:
                exe[:2] = b'ZZ'  # preserve size, but not a Windows PE
            (root / 'dist/RelaxCoach.exe').write_bytes(exe)
            bundle = self.bundle
            if extra_path:
                bundle = root / 'dist/second-unknown.zip'
                shutil.copy2(self.bundle, bundle)
                with zipfile.ZipFile(bundle, 'a') as z:
                    z.writestr('Interface/AddOns/ForeverCoach/unexpected.lua', '-- not shipped')
            # The script archives the real tag from this repository, not mocked
            # installer code. All script writes and game fixtures stay isolated.
            env = dict(os.environ, GIT_DIR=self.gitdir)
            return subprocess.run(['bash', 'scripts/check_old_app_refuses_bundle.sh',
                                   self.old, str(bundle), '1.8.0'], cwd=root, env=env,
                                  text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)

    def test_real_gate_covers_backup_states_and_accepts_executable(self):
        result = self.run_gate()
        self.assertEqual(result.returncode, 0, result.stdout)
        for prior in ('true', 'false'):
            for route in ('app-first', 'installer-fallback'):
                self.assertIn('--- PASS: TestOldAppRefusesRealBundle/prior-backup=' + prior + '/' + route, result.stdout)
        self.assertIn('old app accepted replacement executable sha256=', result.stdout)

    def test_real_gate_rejects_invalid_replacement_executable(self):
        result = self.run_gate(corrupt_exe=True)
        self.assertNotEqual(result.returncode, 0, 'gate skipped old-app executable validation')
        self.assertIn('old app rejected replacement executable', result.stdout)

    def test_real_gate_rejects_a_second_unknown_archive_path(self):
        result = self.run_gate(extra_path=True)
        self.assertNotEqual(result.returncode, 0, 'first-error match hid a second unknown path')
        self.assertIn('unexpected old-app unknown path set', result.stdout)


class RealAcceptanceGateTests(RealGateAssets, unittest.TestCase):
    def test_real_old_app_accepts_current_bundle(self):
        result = subprocess.run(
            ['bash', str(ROOT / 'scripts/check_old_app_accepts_bundle.sh'),
             self.old, str(self.bundle), self.version],
            cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        self.assertEqual(result.returncode, 0, result.stdout)
        # go test exits zero even if GOFLAGS filtered out every test.
        self.assertIn('--- PASS: TestOldAppAcceptsRealBundle ', result.stdout)
        self.assertIn(f'Old app {self.old} accepts {self.bundle.name}.', result.stdout)
        print(result.stdout, end='')


def load_tests(loader, tests, pattern):
    # Explicit selection, not SkipTest: CI must run the real gate for the bundle
    # it just built, without requiring the historical 1.8.0 assets forever.
    real = RealRefusalGateTests if release_version() == '1.8.0' else RealAcceptanceGateTests
    return unittest.TestSuite(loader.loadTestsFromTestCase(case) for case in
                              (ReleaseGateTests, RealGateSetupTests, real))


if __name__ == "__main__":
    unittest.main(verbosity=2)
