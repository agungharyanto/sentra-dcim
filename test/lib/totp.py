#!/usr/bin/env python3
"""Prints the current TOTP code (RFC 6238: SHA1, 6 digits, 30 s) for a base32 secret read from stdin.

Test tooling only (CI throwaway accounts, recovery drill). The secret is read from stdin so it never appears in
the process list. Optional argument: step offset (e.g. 1 = next 30 s window, to avoid the anti-replay rule).
    printf '%s' "$secret" | test/lib/totp.py [offset]
"""
import base64
import hashlib
import hmac
import struct
import sys
import time


def code(secret: bytes, step: int) -> str:
    digest = hmac.new(secret, struct.pack(">Q", step), hashlib.sha1).digest()
    off = digest[-1] & 0x0F
    value = struct.unpack(">I", digest[off:off + 4])[0] & 0x7FFFFFFF
    return f"{value % 1_000_000:06d}"


def main() -> None:
    raw = sys.stdin.read().replace(" ", "").strip().upper()
    secret = base64.b32decode(raw + "=" * (-len(raw) % 8))
    offset = int(sys.argv[1]) if len(sys.argv) > 1 else 0
    # Self-check against RFC 6238 Appendix B before trusting the implementation.
    assert code(b"12345678901234567890", 59 // 30) == "287082"
    print(code(secret, int(time.time()) // 30 + offset))


if __name__ == "__main__":
    main()
