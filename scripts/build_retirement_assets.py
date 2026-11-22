#!/usr/bin/env python3
"""Build only the retirement EXE checksum; never bundle, delete or publish."""
from pathlib import Path
import argparse
import hashlib
import struct

ASSETS = {'RelaxCoach.exe', 'SHA256SUMS.txt'}


def check_directory(directory):
    directory = Path(directory).absolute()
    if any(p.is_symlink() for p in (directory, *directory.parents)):
        raise SystemExit('Refusing linked retirement directory')
    if not directory.exists():
        return
    for path in directory.iterdir():
        if path.name not in ASSETS:
            raise SystemExit('Unexpected retirement asset: ' + path.name + '; use a clean output directory')
        if path.is_symlink() or not path.is_file():
            raise SystemExit('Refusing linked or non-regular retirement asset: ' + path.name)


def validate_pe(data):
    message = 'Expected Windows GUI amd64 PE32+ executable'
    try:
        pe = struct.unpack_from('<I', data, 0x3c)[0]
        optional = pe + 24
        if (data[:2] != b'MZ' or data[pe:pe+4] != b'PE\0\0'
                or struct.unpack_from('<H', data, pe+4)[0] != 0x8664
                or struct.unpack_from('<H', data, pe+20)[0] < 70
                or struct.unpack_from('<H', data, optional)[0] != 0x20b
                or struct.unpack_from('<H', data, optional+68)[0] != 2):
            raise SystemExit(message)
    except (struct.error, OverflowError):
        raise SystemExit(message)


def validate_release(directory):
    directory = Path(directory)
    check_directory(directory)
    if {p.name for p in directory.iterdir()} != ASSETS:
        raise SystemExit('retirement release assets missing')
    data = (directory / 'RelaxCoach.exe').read_bytes()
    validate_pe(data)
    expected = hashlib.sha256(data).hexdigest() + '  RelaxCoach.exe\n'
    if (directory / 'SHA256SUMS.txt').read_text() != expected:
        raise SystemExit('retirement checksum mismatch')


def build(exe):
    exe = Path(exe).absolute()
    check_directory(exe.parent)
    if exe.name != 'RelaxCoach.exe' or not exe.is_file():
        raise SystemExit('Provide the verified Windows GUI amd64 PE32+ as RelaxCoach.exe')
    data = exe.read_bytes()
    validate_pe(data)
    (exe.parent / 'SHA256SUMS.txt').write_text(hashlib.sha256(data).hexdigest() + '  RelaxCoach.exe\n')
    validate_release(exe.parent)
    print('Retirement assets: RelaxCoach.exe, SHA256SUMS.txt (no bundle)')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check-directory', type=Path)
    parser.add_argument('--exe', type=Path)
    args = parser.parse_args()
    if args.check_directory is not None:
        check_directory(args.check_directory)
    elif args.exe is not None:
        build(args.exe)
    else:
        parser.error('provide --check-directory or --exe')


if __name__ == '__main__':
    main()
