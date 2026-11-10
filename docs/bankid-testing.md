# KontoAPI setup and testing

Start with [the workflow](workflow.md). The integration authenticates through
GU KontoAPI, then records explicit consent in the app or guardian browser page.
It does not sign arbitrary text in BankID. See [architecture](bankid-signing-plan.md).

## Local test setup

Connect this computer to the GU VPN before live testing. ADFS authentication was
verified during implementation; the KontoAPI test host on port 9444 timed out
without VPN. The [remote test deployment](test-deployment.md#deployed-verification--2026-11-03)
has since verified live order creation, QR retrieval and pending status from the
cluster; completing a BankID identity and accepting consent remain manual checks.

The ignored root `test-env.txt` uses these keys, one per line:

```text
ak-test:API_KEY
cid-test:CLIENT_ID
cs-test:CLIENT_SECRET
```

Do not commit credentials or put them in Flutter build arguments. The runner
reads them without printing values, builds the API, creates an isolated evidence
keyring and database under ignored `api/.local/kontoapi/`, and publishes
`deploy/test-consent.txt` as `kontoapi-technical-v1`. This consent is only for
technical tests and does not enrol anyone in a research study.

From the repository root:

```sh
# Offline preparation (already possible without VPN):
python3 scripts/run-kontoapi-test.py --prepare-only

# After connecting to VPN: create an order, verify QR/status response formats.
python3 scripts/run-kontoapi-test.py --check

# Run the API on port 8090:
python3 scripts/run-kontoapi-test.py
```

The diagnostic order is not used for enrollment and creates no study session
or consent. It expires upstream; the wrapper documents no cancellation endpoint.
The service does not read an old `api/.env` when launched through this runner.

Run Flutter against that API, using a reachable LAN address on a physical phone:

```sh
flutter pub get
PATH="$(xcode-select -p)/usr/bin:$PATH" \
  flutter run --dart-define=API_BASE_URL=http://YOUR_LAN_IP:8090
```

Replace `YOUR_LAN_IP` with the Mac's actual LAN address (`ipconfig getifaddr en0`
for Wi-Fi on this Mac). If Flutter lists multiple devices, add `-d DEVICE_ID`
using the iPhone ID from `flutter devices`. The PATH prefix selects Apple's LLDB;
Homebrew LLVM's LLDB lacks the `device` command used to launch on an iPhone.

An iOS simulator on this Mac can use `http://127.0.0.1:8090`; Android Emulator
uses `http://10.0.2.2:8090`. Physical BankID and HealthKit tests need an iPhone.
The backend needs VPN access; the phone needs connectivity to the backend and
the appropriate test BankID setup. Return manually from BankID after identifying.

For guardian browser testing, expose port 8090 through a public HTTPS origin and
restart with that origin. If using a local tunnel forwarding over loopback:

```sh
python3 scripts/run-kontoapi-test.py \
  --public-url https://YOUR_HTTPS_HOST \
  --trusted-proxies 127.0.0.1/32,::1/128
```

Use only actual proxy peers. Without a public HTTPS origin, participant testing
works but invitation creation reports missing configuration. No tunnel or
deployment is created automatically.

The old database is unsupported and is left untouched. To reset this disposable
setup, stop the API and remove `api/.local/kontoapi/pb_data/`. Retain the keyring
while retaining any evidence. Local client sessions from an old backend are
rejected on verification; reinstall/clear app storage if needed.

### iPhone stays blank during debug launch

On the development Mac, Homebrew LLDB was selected first in PATH and failed with
`'device' is not a valid command`. Flutter then fell back to Xcode automation.
A failed launch also left generated build settings pointing at a stale app bundle
in `build/ios/iphoneos/`; that bundle was missing `Runner.debug.dylib` and crashed
before Dart started. The fresh Xcode bundle was in `build/ios/Debug-iphoneos/`.

If Xcode reports `Library not loaded: @rpath/Runner.debug.dylib`, stop the failed
Flutter/Xcode run, regenerate the build configuration, then launch with Apple LLDB:

```sh
flutter build ios --config-only --debug --no-pub \
  --dart-define=API_BASE_URL=http://YOUR_LAN_IP:8090
PATH="$(xcode-select -p)/usr/bin:$PATH" \
  flutter run --dart-define=API_BASE_URL=http://YOUR_LAN_IP:8090
```

Successful attachment prints `Syncing files to device` and the Flutter run key
commands. Press `r` to verify hot reload. No signing or app-code changes were
needed for this failure.

## Backend configuration

Copy `api/.env.example` to ignored `api/.env` for a manual setup, or inject
environment variables. Direct BankID certificates are no longer needed.

| Variable | Meaning |
| --- | --- |
| `KONTOAPI_ENVIRONMENT` | `disabled` (default), `test`, or `production`. Enabled modes require credentials and evidence keys. |
| `KONTOAPI_URL` | API HTTPS origin; test: `https://sptest.it.gu.se:9444`. |
| `KONTOAPI_TOKEN_URL` | ADFS HTTPS token URL; test: `https://idptest.it.gu.se/adfs/oauth2/token/`. |
| `KONTOAPI_CLIENT_ID`, `KONTOAPI_CLIENT_SECRET`, `KONTOAPI_API_KEY` | Backend credentials only. |
| `STUDY_SECRETS_FILE` | JSON evidence keyring; relative to the API working directory. |
| `BANKID_PUBLIC_URL` | Public HTTPS origin hosting guardian invitations. |
| `TRUSTED_PROXY_CIDRS` | Comma-separated exact trusted ingress networks. Empty for a direct connection. |
| `API_KEY` | Optional, separate researcher upload export credential. |

`BANKID_RETURN_URL`, `BANKID_APP_IDENTIFIER`, `BANKID_IOS_APP_ID` only configure
optional existing app-link hosting. KontoAPI has no documented return-URL
parameter, so leave these unset for the normal manual-return flow.

Use the existing `scripts/create-study-secrets.py` helper for non-runner setups.
Back up the database, raw uploads and original keyring together. Losing keys
makes retained evidence unreadable; key rotation must retain old key IDs.

From `api/` with the desired configuration:

```sh
go build -o app .
./app bankid publish-consent --file ../deploy/test-consent.txt \
  --version kontoapi-technical-v1 --title 'KontoAPI technical test' --dir ./pb_data
./app serve --http=0.0.0.0:8090 --dir ./pb_data
```

For a real study, replace the test file/version with approved text and verify
the authenticated `GET /api/consent/current` result and a full acceptance.

Encrypted evidence can be exported locally to a new private file:

```sh
./app bankid export-evidence --kind consent --record RECORD_ID \
  --out ./evidence-exports/consent.json --dir ./pb_data
```

Kinds are `authentication`, `consent` and `guardian`. Exports contain personal
information; keep them outside version control and shared reports.

## OpenShift

Use [the remote test deployment guide](test-deployment.md) for the complete
OpenShift procedure, private Secret files, image publication, existing storage,
automatic test-consent initialization and in-pod provider check. Render all of
`deploy/` with `kubectl kustomize`; do not apply `deploy/api.yaml` alone.

The target is `https://movement-api.test.appadem.in` in namespace `appademin`.
Once deployed, launch Flutter with `--dart-define-from-file=deploy/flutter-test.json`.
The phone uses the public HTTPS URL; only the backend needs access to KontoAPI.

## API and checks

| Endpoint | Access / behavior |
| --- | --- |
| POST `/api/bankid/attempts` | Client-generated attempt ID/secret and `sameDevice` or `qr`; starts identity only. |
| GET `/api/bankid/attempts/{id}` | Owned attempt status; accepted participant result includes session grant. |
| GET `/api/bankid/attempts/{id}/qr` | Owned pending attempt; PNG data URL. |
| POST `/api/bankid/attempts/{id}/cancel` | Local stop, no upstream cancel. |
| GET `/api/me` | Verified session and current participation state. |
| GET `/api/consent/current` | Authenticated current document. |
| POST `/api/consent/accept` | Session plus document ID/hash and acceptance request ID. |
| GET `/api/consent/receipt` | Authenticated acceptance receipt. |
| POST `/api/consent/withdraw` | Withdraw acceptance and revoke sessions. |
| GET/POST `/api/guardians` | Guardian status/invitations for a consenting minor. |
| GET `/guardian/{id}` | Browser authentication, then explicit approval. |

```sh
flutter analyze
flutter test
cd api
go test ./...
go build -o app .
```

The report builder also checks native builds and captures all preview cases.
On this Mac, the inherited `JAVA_HOME` points to a removed Java installation.
For Android/report builds use the installed Java 21 for that command:

```sh
JAVA_HOME=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home python3 scripts/build-app-report.py
```

Automated tests cover identity-only denial, current-document acceptance,
idempotency, withdrawal/rejoin, guardian ownership/CSRF/identity/version binding,
provider token refresh and malformed responses. Use [manual review](manual-review.md)
for live tests, including unavailable network, local stop, expired requests,
wrong guardian identity, and re-consent after a document change.

## Implementation verification — 2026-11-03

- Flutter analysis and all 164 Flutter tests passed; backend tests and build passed.
- Android release and iOS simulator builds passed using the Java override above.
- Physical iPhone debug build, installation, launch, debugger attachment and hot
  reload passed with Apple's LLDB first in PATH. The owner confirmed that the
  introduction screen displayed and responded to taps.
- The isolated runner initialized technical consent and started successfully:
  health returned 200; unauthenticated consent/session requests returned 401.
- The synthetic report contains 84 views/84 screenshots, plus separate small-screen
  layout captures. Visual review included participant and guardian consent, QR,
  receipts and errors; report search/enlargement worked with browser networking disabled.
- The remote test backend was subsequently deployed and verified for live
  KontoAPI order creation, QR retrieval, pending status and local stop; see
  [deployment verification](test-deployment.md#deployed-verification--2026-11-03).
  Completing an identity, accepting consent, guardian approval and physical-iPhone
  HealthKit remain manual checks. No production/study-content adaptation was performed.
# Real BankID in the test deployment

The study owner confirmed that GU's KontoAPI test host authenticates against
production BankID. A test BankID identity cannot complete that flow. Use real
BankID with [private test identity mappings](test-identities.md) for age and
guardian scenarios in the isolated simulation deployment.
