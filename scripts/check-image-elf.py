#!/usr/bin/env python3
"""Check ELF bytes without executing them (translation cannot hide a mismatch)."""
import pathlib
import struct
import sys

path, arch = sys.argv[1:]
with pathlib.Path(path).open('rb') as binary:
    header = binary.read(20)
assert header[:4] == b'\x7fELF', 'not ELF'
assert header[4:6] == b'\x02\x01', 'expected 64-bit little-endian ELF'
machine = struct.unpack('<H', header[18:20])[0]
expected = {'amd64': 62, 'arm64': 183}[arch]
print(f'{path}: linux/{arch}, e_machine={machine}, expected={expected}')
assert machine == expected, 'image platform and binary architecture differ'
