#!/usr/bin/env python
"""
Binary Wrapper Creator - Privacy Protection for Executables

Creates a self-contained wrapper that:
- Encrypts the binary with XOR + Base64 (no OpenSSL dependency)
- Executes entirely in memory via memfd_create (no file on disk)
- Falls back to /dev/shm + immediate delete (no residual files)
- Shows real-time progress during decryption
- Compatible with Python 2.7+ and any Linux kernel 3.17+

Usage:
    python create_wrapper.py <input_binary> [output_wrapper]

Examples:
    python create_wrapper.py ./cfd-linux-amd64 ./wrapped_cfd
    python create_wrapper.py /path/to/binary
"""

from __future__ import print_function
import os
import sys
import random
import hashlib

try:
    import base64
    HAS_BASE64 = True
except ImportError:
    HAS_BASE64 = False


def xor_encrypt(data, key):
    """XOR encrypt data with key (works in both Python 2 and 3)."""
    if isinstance(data, bytes) and sys.version_info[0] >= 3:
        return bytes([data[i] ^ key[i % len(key)] for i in range(len(data))])
    else:
        return ''.join([chr(ord(data[i]) ^ ord(key[i % len(key)])) for i in range(len(data))])


def b64encode_compat(data):
    """Base64 encode compatible with Python 2 and 3."""
    if HAS_BASE64:
        if isinstance(data, str) and sys.version_info[0] < 3:
            return base64.b64encode(data)
        return base64.b64encode(data).decode('ascii')

    # Manual base64 for environments without the module
    chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
    result = []
    if isinstance(data, bytes) and sys.version_info[0] >= 3:
        get_byte = lambda i: data[i]
    else:
        get_byte = lambda i: ord(data[i])

    for i in range(0, len(data), 3):
        n = get_byte(i) << 16
        if i + 1 < len(data):
            n += get_byte(i + 1) << 8
        if i + 2 < len(data):
            n += get_byte(i + 2)
        result.append(chars[(n >> 18) & 63])
        result.append(chars[(n >> 12) & 63])
        result.append(chars[(n >> 6) & 63] if i + 1 < len(data) else '=')
        result.append(chars[n & 63] if i + 2 < len(data) else '=')

    return ''.join(result)


def generate_key(length=32):
    """Generate random key bytes."""
    if sys.version_info[0] >= 3:
        return bytes([random.randint(0, 255) for _ in range(length)])
    else:
        return ''.join([chr(random.randint(0, 255)) for _ in range(length)])


def key_to_hex(key):
    """Convert key bytes to hex string."""
    if isinstance(key, bytes) and sys.version_info[0] >= 3:
        return key.hex()
    else:
        return ''.join(['%02x' % ord(c) for c in key])


def create_wrapper(input_file, output_file):
    """Create the encrypted wrapper."""
    print('[*] Creating fileless memory-execution wrapper...')
    print('[*] Input: %s' % input_file)

    # Read original binary
    with open(input_file, 'rb') as f:
        data = f.read()

    original_size = len(data)
    print('[+] Original binary size: %d bytes (%.1f MB)' % (original_size, original_size / 1048576.0))

    # Generate encryption key
    key = generate_key(32)
    key_hex = key_to_hex(key)
    print('[+] Generated encryption key')

    # XOR encrypt
    print('[*] Encrypting binary...')
    encrypted = xor_encrypt(data, key)

    # Base64 encode
    print('[*] Encoding payload...')
    encoded = b64encode_compat(encrypted)

    # Add newlines for readability
    encoded_lines = '\n'.join([encoded[i:i+76] for i in range(0, len(encoded), 76)])

    print('[+] Encoded payload size: %d bytes (%.1f MB)' % (len(encoded_lines), len(encoded_lines) / 1048576.0))

    # Build wrapper script
    print('[*] Building wrapper script...')

    wrapper_header = r'''#!/bin/bash
# Fileless Memory-Execution Wrapper
# - Encrypts binary content (XOR + Base64) to evade file scanners
# - Executes in memory via memfd_create (no file created on any filesystem)
# - Falls back to /dev/shm + immediate delete (no residual even with kill -9)
# - Real-time progress indicator during decryption
# - Compatible with Python 2.7+ (no external dependencies)

set -e

PAYLOAD_LINE=$(grep -n "^__PAYLOAD__$" "$0" | head -1 | cut -d: -f1)
[ -z "$PAYLOAD_LINE" ] && { echo "[-] Corrupted wrapper" >&2; exit 1; }

# Decrypt payload to stdout
decrypt_payload() {
    tail -n +$((PAYLOAD_LINE + 1)) "$0" | python -c 'import sys, os

sys.stderr.write("[*] Loading encrypted payload...\n")
sys.stderr.flush()

key_hex = "'''

    wrapper_mid = r'''"
key = "".join([chr(int(key_hex[i:i+2],16)) for i in range(0,64,2)])

# Read stdin in chunks with progress
chunks = []
total_read = 0
while True:
    chunk = sys.stdin.read(1048576)
    if not chunk:
        break
    chunks.append(chunk)
    total_read += len(chunk)
    sys.stderr.write("\r[*] Reading payload... %d MB" % (total_read // 1048576))
    sys.stderr.flush()

b64 = "".join(chunks).replace("\n","").replace("\r","")
total = len(b64)

sys.stderr.write("\r[*] Decoding payload...   0%%")
sys.stderr.flush()

chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
d = {}
for i in range(64): d[chars[i]] = i
data = []
for i in range(0, len(b64), 4):
    if i % 400000 == 0:
        pct = (i * 100) // total
        sys.stderr.write("\r[*] Decoding payload... %3d%%" % pct)
        sys.stderr.flush()
    n = d.get(b64[i],0) << 18
    n += d.get(b64[i+1],0) << 12
    c2 = b64[i+2] if i+2 < len(b64) else "="
    c3 = b64[i+3] if i+3 < len(b64) else "="
    if c2 != "=": n += d.get(c2,0) << 6
    if c3 != "=": n += d.get(c3,0)
    data.append(chr((n>>16)&255))
    if c2 != "=": data.append(chr((n>>8)&255))
    if c3 != "=": data.append(chr(n&255))
data = "".join(data)
total_b = len(data)
dec = []
for i in range(len(data)):
    if i % 400000 == 0:
        pct = (i * 100) // total_b
        sys.stderr.write("\r[*] Decrypting payload... %3d%%" % pct)
        sys.stderr.flush()
    dec.append(chr(ord(data[i])^ord(key[i%32])))
sys.stderr.write("\r[+] Ready! Launching...        \n")
sys.stderr.flush()
sys.stdout.write("".join(dec))
'
}

# Method 1: memfd_create via Perl (completely fileless - no file on any path)
try_memfd_exec() {
    decrypt_payload | perl -e '
use strict;
use POSIX;

binmode(STDIN);
my $binary = do { local $/; <STDIN> };

# memfd_create syscall (319 on x86_64, 385 on arm64)
my $fd = syscall(319, "exec", 1);
if ($fd < 0) {
    exit 1;
}

open(my $fh, ">&=", $fd) or exit 1;
binmode($fh);
print $fh $binary;
close($fh);

my $fd_path = "/proc/self/fd/$fd";
exec {$fd_path} $fd_path, @ARGV;
' -- "$@"
}

# Method 2: /dev/shm + open fd + immediate delete (no residual even with kill -9)
try_shm_exec() {
    TEMP_FILE=/dev/shm/.x$$
    decrypt_payload > "$TEMP_FILE"
    chmod +x "$TEMP_FILE"
    exec 3<"$TEMP_FILE"
    rm -f "$TEMP_FILE"
    exec /proc/self/fd/3 "$@"
}

# Try memfd first (truly fileless), fallback to shm+delete
try_memfd_exec "$@" 2>/dev/null || try_shm_exec "$@"

__PAYLOAD__
'''

    # Write wrapper file
    with open(output_file, 'w') as f:
        f.write(wrapper_header)
        f.write(key_hex)
        f.write(wrapper_mid)
        f.write(encoded_lines)

    # Make executable
    os.chmod(output_file, 0o755)

    final_size = os.path.getsize(output_file)
    print('')
    print('=' * 60)
    print('[+] Wrapper created successfully!')
    print('=' * 60)
    print('[+] Output: %s' % output_file)
    print('[+] Size: %d bytes (%.1f MB)' % (final_size, final_size / 1048576.0))
    print('[+] SHA256: %s' % hashlib.sha256(open(output_file, 'rb').read()).hexdigest())
    print('')
    print('Execution methods (in priority order):')
    print('  1. memfd_create  - Completely fileless (Linux 3.17+)')
    print('  2. /dev/shm + fd - No residual files (any Linux)')
    print('')
    print('Usage:')
    print('  ./%s [args...]' % os.path.basename(output_file))


def main():
    if len(sys.argv) < 2 or sys.argv[1] in ('-h', '--help'):
        print(__doc__)
        sys.exit(0)

    input_file = sys.argv[1]
    if not os.path.isfile(input_file):
        print('[-] Error: Input file not found: %s' % input_file)
        sys.exit(1)

    if len(sys.argv) >= 3:
        output_file = sys.argv[2]
    else:
        base = os.path.basename(input_file)
        output_file = 'wrapped_' + base

    create_wrapper(input_file, output_file)


if __name__ == '__main__':
    main()
