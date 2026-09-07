#!/usr/bin/env python3
"""Create a private keyring file without printing keys or overwriting a file."""
import argparse
import base64
import json
import os
import secrets
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', type=Path, required=True)
args = parser.parse_args()
args.out.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
key = lambda: base64.b64encode(secrets.token_bytes(32)).decode('ascii')
value = {'activeKey': 'v1', 'encryptionKeys': {'v1': key()}, 'identityHmacKey': key()}
fd = os.open(args.out, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as output:
    json.dump(value, output, indent=2)
    output.write('\n')
    output.flush()
    os.fsync(output.fileno())
print(f'Created {args.out}. Back it up securely; it is required to read signing evidence.')
