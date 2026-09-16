# BankID setup, testing and operations

## Backend configuration

Use the Go toolchain pinned in `api/go.mod` and Flutter compatible with `pubspec.yaml`. Keep test and production databases and credentials separate.

The BankID client uses mutual TLS. Configure PEM client certificate/private key and the BankID server CA for the selected environment. Obtain test material and test BankID setup instructions from the official BankID developer portal. The private key belongs on the backend only.

```bash
python3 scripts/create-study-secrets.py --out api/secrets/study-keys.json
cp api/.env.example api/.env
cd api
# Edit .env with certificate paths, study identity, bundle ID and return URL.
set -a
source .env
set +a
go build -o app .
```

The key-generation script refuses to overwrite an existing file and does not print secrets. Preserve the study keyring separately from database backups. The backend does not automatically load `.env`.

| Setting | Meaning |
| --- | --- |
| `BANKID_ENVIRONMENT` | `test`, `production`, or `disabled` (default). |
| `STUDY_ID` | Study namespace. |
| `BANKID_APP_IDENTIFIER` | Installed iOS bundle identifier. |
| `BANKID_CERT_FILE`, `BANKID_KEY_FILE`, `BANKID_CA_FILE` | PEM credential and trust files. |
| `STUDY_SECRETS_FILE` | JSON keyring and identity HMAC key. |
| `BANKID_RETURN_URL` | Test custom return URL or HTTPS universal link. |
| `BANKID_IOS_APP_ID` | Apple app ID prefix plus bundle ID, required for HTTPS returns. |
| `TRUSTED_PROXY_CIDRS` | Actual ingress proxy ranges; empty for direct access. |
| `BANKID_SIGNING_ENABLED` | Set `false` to stop new attempts while existing attempts finish. |
| `API_KEY` | Optional researcher export credential; never put it in Flutter. |

Enabled environments require valid certificates, keys, and return configuration. Disabled mode permits consent administration but no BankID requests. Configure the intended environment before issuing invitations. The backend verifies client IP through its configured trusted proxy chain.

## Existing installation: one-time cutover

Stop the old backend and keep its data directory and encryption keys. From `api/`, with the same study configuration loaded:

```bash
go run ./cmd/cutover --source ./pb_data --out ./pb_data-v2
./app serve --http=0.0.0.0:8080 --dir=./pb_data-v2
```

The output directory must not exist and must be outside the source directory. The converter copies SQLite databases using SQLite snapshots and copies uploaded files with private permissions. It never modifies the source. Keep the server stopped to maintain consistency between database and file snapshots. If conversion fails, treat its output as incomplete and use a new output directory for the next attempt.

The conversion preserves participant IDs, answers and questionnaire relations, consent documents/evidence, metadata history, and upload files. Conflicting live invitations or identity mappings stop conversion for reconciliation. Old pending attempts become unknown outcomes; missing provider responses are not invented. Existing login tokens are invalidated. The matching Flutter release must be installed; old clients are unsupported.

After verifying the converted copy, point the deployment's persistent data directory at that copy. Retain the source as an offline backup according to the study's storage policy. Do not run the previous backend against the converted database.

A fresh installation needs no conversion:

```bash
./app serve --http=0.0.0.0:8080 --dir=./pb_data
```

## Publish consent and issue invitations

Open the protected PocketBase administration UI at `/_/` as a superuser while this backend is running.

In **Collections → consent_texts → New record**, enter `version`, `title`, and `text`. Saving publishes the immutable text immediately and makes it current. The backend assigns study and hash. Publish another version to change the text; participants must sign that version before further writes.

In **Collections → users → New record**, enter `username` (the participant ID), optionally `validityHours` (default 168), and optionally `expectedPersonalNumber` (12 digits). PocketBase’s auth-record form also requires its password fields: use **Generate and set random password**. The backend ignores those inputs and keeps password login disabled. Leave server-managed fields at their defaults. Saving creates an inactive participant with a six-digit invitation code formatted `XXX-XXX`. Reopen the record to copy `invitationCode`. The expected personal number clears after saving; the stored value is encrypted.

To reissue an invitation, create another invitation using the same `username`, or use the command below. The operation retains the existing inactive user and replaces the invitation. Already enrolled users use BankID login and cannot receive an enrollment invitation.

After signing, the same user becomes active, the invitation is cleared, and a `signatures` record contains the encrypted evidence and outcome. `signatures` also includes authentication, rejected signing, and observed failed/cancelled/unknown attempts. No pending-attempt collection exists.

Equivalent offline operator commands (stop the server first):

```bash
./app study publish-consent --file consent.txt --version 2026-01 --title 'Study consent' --dir=./pb_data
./app study invite --participant TEST-001 --hours 168 --dir=./pb_data
./app study export-evidence --signature SIGNATURE_ID --out evidence.json --dir=./pb_data
```

`invite` accepts `--expected-identity-file` containing the expected 12-digit signer identity. Evidence export creates a new private file and includes personal information; it never overwrites an existing file.

## Configure and run Flutter

Set the final bundle identifier and HealthKit permission text in Xcode. For test custom-scheme returns:

```bash
flutter pub get
flutter run --dart-define=API_BASE_URL=https://YOUR_TEST_API_HOST \
  --dart-define=BANKID_RETURN_URL=researchsteps://bankid/return
```

Production uses an HTTPS return URL at `/bankid/return`. Configure the same URL in Flutter/backend and use `scripts/configure-bankid-ios.py` to configure the associated domain as described by its `--help`. The backend serves `/.well-known/apple-app-site-association` using `BANKID_IOS_APP_ID`. Keep Flutter's default deep link handler disabled so `app_links` owns return handling.

The shared Runner entitlements explicitly configure the app’s Keychain access group, used by secure storage in all build modes. See the [secure-storage plugin’s Keychain setup](https://github.com/juliansteenbakker/flutter_secure_storage#macos--ios) when changing signing or app-group configuration. The app saves attempt credentials and login tokens in secure storage. A relaunch can resume an attempt while the backend process still holds it. A backend restart requires a new attempt. Session expiry and logout clear the app's authentication and health state. Logout signs out every device for that participant.

## HTTP interface

All participant responses use explicit public fields. Generic collection endpoints cannot bypass consent or expose invitation/identity/evidence fields. Authenticated participants with current consent can read questionnaire definitions and read/write their own `answers` through the standard PocketBase collection endpoints.

| Operation | Request |
| --- | --- |
| Current consent | `GET /api/study/consent/current` |
| Start enrollment signing | `POST /api/study/bankid/attempts` with `clientSecret`, `invitationCode`, `consentTextId`, `documentHash`, and `mode`. |
| Start returning login | Same endpoint, with `clientSecret` and `mode`. |
| Sign after returning login | Same endpoint, with a new `clientSecret`, `authAttempt` credential and reviewed consent fields. |
| Status and session delivery | `GET /api/study/bankid/attempts/{id}` |
| Cancel | `POST /api/study/bankid/attempts/{id}/cancel` |
| BankID return | `POST /api/study/bankid/attempts/{id}/return` with `nonce`. |
| Participant | `GET /api/study/me` |
| Receipt | `GET /api/study/consent/receipt` |
| Withdraw | `POST /api/study/consent/withdraw` |
| Logout all devices | `POST /api/study/logout` |
| Participant metadata | `POST /api/study/metadata` with matching `participantId` and `data`. |
| Chunked upload | `POST /data` with matching `participantId`, `chunkIndex`, and `data`; gzip supported. |
| Researcher export | `GET /data/{participantId}` with `X-API-Key`. |

The app generates a 32-byte random Base64URL `clientSecret` without padding. The attempt ID is the first 32 hexadecimal characters of its SHA-256 digest. Save ID/secret/expiry before starting. Attempt authorization is `Bearer <id>.<clientSecret>`; participant authorization is the returned PocketBase token. Retrying an identical start is safe while its attempt remains in memory. After a lost start response, query status; do not automatically start another provider request.

Accepted status includes either a `grant` or `consentRequired: true`. A signing receipt can be displayed before accepting the grant. The server rechecks current consent and revocation when returning status. An expired/missing attempt returns HTTP 410, requiring a fresh start. A return callback never substitutes for accepted status.

## Verification

```bash
cd api
go test -race ./...
go vet ./...
cd ..
flutter analyze
flutter test
flutter build ios --simulator
```

Automated tests use fake BankID responses with real temporary PocketBase databases. They verify stored evidence, identity checks, consent renewal/withdrawal, invitation replacement, duplicate requests, cancellation races, persistence retry, restart handling, revoked sessions, conversion, and upload ownership/chunk ordering. Flutter tests verify consent review, launch/return handling, resume, expiry, and QR layout. No runtime fake-provider switch exists.

On a test iPhone, verify invitation → review → BankID signing → return → receipt → HealthKit → upload. Then verify returning login, a newly published consent, wrong signer, cancellation, QR signing with another device, app background/termination, backend restart, and logout. Record these results separately from automated tests; a simulator build does not prove real BankID handoff or HealthKit behavior.

## Deployment and maintenance

Use `api/deploy/api.yaml` with the actual image, hostnames, app identifiers and secret mount. Keep `replicas: 1` and the `Recreate` strategy. Pending attempts are process-local and do not survive rolling replacement or certificate rotation; participants retry after restart.

For certificate rotation, finish or cancel active attempts, then restart with the new certificate. For encryption key rotation, retain old decryption keys and change the active key ID; deleting old keys makes existing evidence unreadable. No database session-purge command is needed. Monitor certificate expiry, failed/rejected outcomes, persistence errors, and process memory. Configure backup and retention for participant data and evidence with the study owner.
