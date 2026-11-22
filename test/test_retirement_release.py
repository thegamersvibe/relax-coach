"""App-only retirement packaging: stdlib fixtures, no publication."""
from pathlib import Path
import hashlib
import runpy
import shutil
import struct
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


def synthetic_pe():
    # Minimal header fixture, not a runnable binary. The real release gate also
    # runs the unchanged Go updater validator on the actual cross-built EXE.
    data = bytearray(512)
    data[:2] = b'MZ'
    struct.pack_into('<I', data, 0x3c, 0x80)
    data[0x80:0x84] = b'PE\0\0'
    struct.pack_into('<H', data, 0x84, 0x8664)
    struct.pack_into('<H', data, 0x94, 0xf0)
    struct.pack_into('<H', data, 0x98, 0x20b)
    struct.pack_into('<H', data, 0x98 + 68, 2)
    return data


class RetirementPackagingTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        (self.root / 'scripts').mkdir()
        (self.root / 'dist').mkdir()
        for source in (ROOT / 'scripts').glob('build_*.py'):
            shutil.copy2(source, self.root / 'scripts' / source.name)
        self.exe = self.root / 'dist/RelaxCoach.exe'
        self.exe.write_bytes(synthetic_pe())

    def build(self):
        return subprocess.run([sys.executable, str(self.root / 'scripts/build_guild_bundle.py'),
                               '--exe', str(self.exe), '--version', '1.9.0'],
                              capture_output=True, text=True)

    def test_retirement_build_has_only_exe_and_one_checksum(self):
        result = self.build()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual({p.name for p in self.exe.parent.iterdir()}, {'RelaxCoach.exe', 'SHA256SUMS.txt'})
        self.assertEqual((self.exe.parent / 'SHA256SUMS.txt').read_text(),
                         hashlib.sha256(self.exe.read_bytes()).hexdigest() + '  RelaxCoach.exe\n')
        again = self.build()
        self.assertEqual(again.returncode, 0, again.stdout + again.stderr)

    def test_stale_asset_or_link_is_refused_not_wiped(self):
        for name in ('RelaxCoach-RaidBundle-1.9.0.zip', 'RelaxCoach-Bundle-1.8.2.zip', 'guild-package'):
            with self.subTest(name=name):
                extra = self.exe.parent / name
                extra.write_bytes(b'keep')
                result = self.build()
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('Unexpected retirement asset', result.stderr)
                self.assertEqual(extra.read_bytes(), b'keep')
                extra.unlink()
        link = self.exe.parent / 'SHA256SUMS.txt'
        link.symlink_to(self.root / 'missing')
        result = self.build()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('linked', result.stderr)
        self.assertTrue(link.is_symlink())

    def test_pe_requires_amd64_pe32plus_windows_gui(self):
        for offset, value in ((0, b'ZZ'), (0x84, b'\x4c\x01'), (0x98, b'\x0b\x01'), (0x98 + 68, b'\x03\x00')):
            with self.subTest(offset=offset):
                data = synthetic_pe()
                data[offset:offset+len(value)] = value
                self.exe.write_bytes(data)
                result = self.build()
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('Windows GUI amd64 PE32+', result.stderr)
                self.assertFalse((self.exe.parent / 'SHA256SUMS.txt').exists())

    def test_resource_version_is_190(self):
        import json
        resource = json.loads((ROOT / 'desktop/winres/winres.json').read_text())
        version = resource['RT_VERSION']['#1']['0000']
        self.assertEqual(version['fixed']['file_version'], '1.9.0.0')
        self.assertEqual(version['fixed']['product_version'], '1.9.0.0')
        self.assertEqual(version['info']['0409']['FileVersion'], '1.9.0')
        self.assertEqual(version['info']['0409']['ProductVersion'], '1.9.0')


if __name__ == '__main__':
    unittest.main(verbosity=2)
