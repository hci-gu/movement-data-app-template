#!/usr/bin/env python3
"""Run the KontoAPI test backend with local credentials; never print secrets."""
import argparse
import base64
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--credentials", type=Path, default=ROOT / "test-env.txt")
    parser.add_argument("--public-url", default="", help="Public HTTPS origin for guardian invitations (e.g. a tunnel to port 8091)")
    parser.add_argument("--kontoapi-url", default="https://sptest.it.gu.se:9444")
    parser.add_argument("--token-url", default="https://idptest.it.gu.se/adfs/oauth2/token/")
    parser.add_argument("--trusted-proxies", default="", help="Exact ingress CIDRs; for a local tunnel use 127.0.0.1/32,::1/128")
    parser.add_argument("--check", action="store_true", help="Check live order/QR/status and exit; requires VPN")
    parser.add_argument("--prepare-only", action="store_true", help="Build and initialize the disposable database without contacting KontoAPI")
    parser.add_argument("--simulation", action="store_true", help="Use a separate test-identity database; manage mappings in PocketBase")
    args = parser.parse_args()
    values = {}
    for line in ([] if args.prepare_only else args.credentials.read_text(encoding="utf-8").splitlines()):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        key, separator, value = line.partition(":")
        if not separator:
            raise ValueError("Credentials must use ak-test:, cid-test: and cs-test: lines")
        values[key.strip()] = value.strip()
    if not args.prepare_only and not all(values.get(key) for key in ("ak-test", "cid-test", "cs-test")):
        raise ValueError("Missing ak-test, cid-test or cs-test in the credentials file")
    local = ROOT / "api" / ".local" / ("scoliosis-simulation" if args.simulation else "scoliosis-kontoapi")
    local.mkdir(parents=True, exist_ok=True)
    keyring = local / "study-keys.json"
    if not keyring.exists():
        with os.fdopen(os.open(keyring, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as output:
            json.dump({"activeKey": "local-test", "encryptionKeys": {
                "local-test": base64.b64encode(secrets.token_bytes(32)).decode()
            }}, output)
    environment = os.environ.copy()
    # This runner is isolated from an old local .env and any previous study setup.
    for key in list(environment):
        if key.startswith(("BANKID_", "KONTOAPI_", "STUDY_")):
            del environment[key]
    environment.update({
        "STUDY_SKIP_DOTENV": "1", "KONTOAPI_ENVIRONMENT": "disabled" if args.prepare_only else "test",
        "KONTOAPI_URL": args.kontoapi_url, "KONTOAPI_TOKEN_URL": args.token_url,
        "KONTOAPI_API_KEY": values.get("ak-test", ""), "KONTOAPI_CLIENT_ID": values.get("cid-test", ""),
        "KONTOAPI_CLIENT_SECRET": values.get("cs-test", ""), "STUDY_SECRETS_FILE": str(keyring),
        "BANKID_PUBLIC_URL": args.public_url, "TRUSTED_PROXY_CIDRS": args.trusted_proxies,
    })
    binary = local / "app"
    if args.simulation:
        environment.update({"STUDY_DEPLOYMENT": "simulation", "STUDY_SIMULATION_ID": "scoliosis-local-v1"})
    subprocess.run(["go", "build", "-tags", "study_simulation" if args.simulation else "", "-o", str(binary), "."], cwd=ROOT / "api", check=True)
    data = local / "pb_data"
    # Seed explicitly labelled study consent only on first initialization.
    first = not (data / "data.db").exists()
    if first:
        subprocess.run([str(binary), "bankid", "publish-consent", "--file", str(ROOT / "deploy" / "consent" / "scoliosis-consent.txt"),
                        "--version", "scoliosis-kontoapi-v1", "--title", "Rörelsemönster vid skolios", "--dir", str(data)],
                       cwd=ROOT / "api", env=environment, check=True)
    if args.prepare_only:
        print("Test backend built; disposable database and study consent initialized.")
        return
    command = [str(binary), "bankid", "check-kontoapi"] if args.check else [str(binary), "serve", "--http=0.0.0.0:8091"]
    command += ["--dir", str(data)]
    if not args.check:
        print("Starting test API on port 8091. KontoAPI requires VPN. Guardian invitations require --public-url.", flush=True)
    try:
        subprocess.run(command, cwd=ROOT / "api", env=environment, check=True)
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        # Do not print credential values or subprocess environments.
        print(f"Test setup failed ({type(error).__name__}). Check the credentials file and command output.", file=sys.stderr)
        sys.exit(1)
