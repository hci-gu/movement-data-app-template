#!/usr/bin/env python3
"""Add a real BankID return domain to the existing iOS entitlements."""
import argparse
import plistlib
import re
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--domain', required=True, help='HTTPS return hostname, without a scheme or path')
parser.add_argument('--entitlements', type=Path, default=Path(__file__).resolve().parents[1] / 'ios/Runner/Runner.entitlements')
args = parser.parse_args()
if not re.fullmatch(r'[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?', args.domain) or '.' not in args.domain or '..' in args.domain:
    parser.error('Provide a valid public DNS hostname')
with args.entitlements.open('rb') as source:
    data = plistlib.load(source)
domains = data.setdefault('com.apple.developer.associated-domains', [])
entry = 'applinks:' + args.domain.lower()
if entry not in domains:
    domains.append(entry)
with args.entitlements.open('wb') as target:
    plistlib.dump(data, target, sort_keys=False)
print(f'Configured {entry} in {args.entitlements}. Enable Associated Domains for the app in your Apple developer account.')
