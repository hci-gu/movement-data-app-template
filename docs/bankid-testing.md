# BankID: configuration, testing and operations

Implemented 2026-09-07. Start here to configure the Flutter app and PocketBase API. The [research and design](bankid-signing-plan.md) explains the provider decisions; this document describes the actual implementation.

## What you need to provide

Direct Swedish BankID integration uses **mutual TLS certificates, not an API key, client ID or OAuth secret**. No intermediary signing provider account is required by this implementation. The public test environment is free; production requires an agreement with a connected bank or reseller. [BankID environments](https://developers.bankid.com/getting-started/environments), [connect a company](https://www.bankid.com/foretag/anslut-foeretag).

| Item | How to obtain it | Configuration |
| --- | --- | --- |
| BankID RP client certificate and corresponding private key for **test** | Download the certificate bundle from the [official test information page](https://developers.bankid.com/test-portal/test-information). It currently offers `FPTestcert5_20240610.p12`, PEM and a legacy PFX; its published test passphrase is `qwerty123`. | Backend `BANKID_CERT_FILE` and `BANKID_KEY_FILE`, both PEM files. The private key must be unencrypted in its protected runtime mount. |
| BankID server trust CA for the chosen environment | Use the correct PEM certificate from [BankID environments](https://developers.bankid.com/getting-started/environments). The test and production trust roots differ. | Backend `BANKID_CA_FILE`. Normal website trust alone is not used by this client. |
| Test BankID on an iPhone | Follow the [test client setup guide](https://developers.bankid.com/test-portal/bankid-for-test), then [issue a test BankID](https://developers.bankid.com/test-portal/testing). Use a dedicated test device and synthetic identity. | Installed in the BankID app; never supply the participant's security code to this backend. |
| Evidence encryption keyring and identity HMAC key | Generate locally using the included script below; these are **our secrets**, not credentials issued by BankID. | Backend `STUDY_SECRETS_FILE`; keep a secure backup separate from the database. |
| Actual iOS bundle identifier | Runner target in Xcode. The repository currently has `com.appademin.swedHeart`; change it for the intended study if necessary. | Backend `BANKID_APP_IDENTIFIER` must match the installed app exactly. [BankID app request fields](https://developers.bankid.com/api-references/auth--sign/sign). |
| API hostname and return URL | Your test/production hosting. The phone must reach the API over HTTPS for a realistic test. | Flutter `API_BASE_URL`; the same `BANKID_RETURN_URL` on Flutter and backend. |
| Apple app ID prefix and associated domain, for HTTPS returns | Your [Apple developer account](https://developer.apple.com/account/), app identifier and provisioning profile. The prefix is usually the team ID; confirm the installed app's identifier. | `BANKID_IOS_APP_ID=PREFIX.bundle.identifier`, the domain entitlement, and the AASA endpoint described below. [Flutter universal link setup](https://docs.flutter.dev/cookbook/navigation/set-up-universal-links). |
| Production RP certificate/key and agreement, later | Order through the bank/reseller selected by the service owner. They determine certificate issuance, organization display name and commercial terms. | Separate production deployment, database, trust root and keyring; `BANKID_ENVIRONMENT=production`. |
| Optional researcher export credential | Generate a separate random secret if using the existing `GET /data/{participantId}` research export. | Backend `API_KEY`; send as `X-API-Key`. This is unrelated to BankID and must never be compiled into Flutter. |

No real RP credentials or test identities were installed during implementation. Fake BankID responses exist only in automated test files, with no runtime bypass flag.

## Configure a test backend

Use Go 1.27.1 (the module pins the toolchain), Flutter 3.35.2 or later compatible with the lockfile, and a working iOS Xcode installation. Keep test and production in different data directories and deployments.

From the repository root:

```bash
python3 scripts/create-study-secrets.py --out api/secrets/study-keys.json
cp api/.env.example api/.env
cd api
go build -o app .
```

The script creates two independent random 32-byte secrets, writes a new file with mode `0600`, and refuses to overwrite it. It does not print keys. Store the downloaded certificates under `api/secrets/`, which is excluded from Git and Docker contexts.

If you downloaded the P12 bundle, extract PEM files using OpenSSL. These commands prompt for its import passphrase. Use the modern P12 with OpenSSL 3; if your local OpenSSL cannot read it, use a compatible OpenSSL installation or the official PEM alternative.

```bash
umask 077
openssl pkcs12 -in secrets/FPTestcert5_20240610.p12 -clcerts -nokeys -out secrets/client.pem
openssl pkcs12 -in secrets/FPTestcert5_20240610.p12 -nocerts -nodes -out secrets/client-key.pem
```

Save the **test server CA** as `secrets/bankid-server-ca.pem`. Set the actual bundle identifier in `.env`, retain `BANKID_ENVIRONMENT=test`, and choose a unique test `STUDY_ID`. Paths in `.env` are relative to the API process working directory. The program deliberately does not load `.env` automatically:

```bash
set -a
source .env
set +a
```

An unset environment defaults to `disabled`; signing and new logins are unavailable until configured. Enabled environments fail startup when required keys, certificates or return configuration are invalid. The provider endpoint is fixed by the environment, with no arbitrary URL override:

- Test: `https://appapi2.test.bankid.com/rp/v6.0`
- Production: `https://appapi2.bankid.com/rp/v6.0`

If an ingress sits in front of PocketBase, set `TRUSTED_PROXY_CIDRS` to its actual network ranges. The backend walks `X-Forwarded-For` from the trusted proxy back toward the caller, ignoring spoofed forwarding headers from other peers. The proxy must preserve the genuine client address and restrict direct access to the backend. Wrong addresses can affect BankID's risk assessment. [Request IP requirements](https://developers.bankid.com/api-references/auth--sign/auth).

## Publish consent and issue an invitation

Run local study commands against the same `--dir` as the service. Stop the service during operator commands: this release uses one process per database, including its BankID worker. Commands run migrations before operating. No approved study consent is invented or automatically published by this integration.

Create a UTF-8 plain-text file containing the approved consent. In a test database, use clearly labelled test consent. The exact bytes are hashed, displayed in the app and sent as `userVisibleData` to BankID. Maximum size is 30,000 UTF-8 bytes (40,000 after Base64); unsupported control characters and emoji are rejected.

```bash
./app study publish-consent --dir=./pb_data \
  --file=/absolute/path/to/approved-consent.txt \
  --version=2026-01 --title='Study participation consent'

./app study invite --dir=./pb_data --participant=TEST-001 --hours=168

./app serve --http=0.0.0.0:8080 --dir=./pb_data
```

The invitation is a random, one-use bearer code printed once by the local command. Deliver it through your study's existing enrollment process. A participant cannot choose or claim a record by typing an identifier. To bind an invitation to a known signer, add `--expected-identity-file=/private/path/test-identity.txt`; the file must contain the expected 12-digit personal number. Its content is encrypted, not printed or supplied as a command-line value. In the test environment use synthetic identities only. [BankID test identity guidance](https://developers.bankid.com/test-portal/test-information).

Without that optional binding, possession of the invitation authorizes the first eligible signer to enroll that participant. For existing participant records, verify the intended recipient before issuing or delivering their invitation.

Publishing a new version preserves old documents/signatures and immediately requires the new version for further uploads. Existing boolean consent does not count as a BankID signature.

## Connect the iPhone app

Run the Flutter commands from the repository root in another terminal.

For an initial **test-only** same-device return, `.env.example` uses the registered custom scheme `researchsteps://bankid/return`. Build with the same value:

```bash
flutter pub get
flutter run \
  --dart-define=API_BASE_URL=https://YOUR_TEST_API_HOST \
  --dart-define=BANKID_RETURN_URL=researchsteps://bankid/return
```

Use a physical device with BankID configured for test. A production BankID cannot sign against the test RP endpoint. The official setup procedure changes the BankID app's environment and can require reinstalling it, which is why a dedicated test device is preferable. Follow the current [BankID iOS test instructions](https://developers.bankid.com/test-portal/bankid-for-test) rather than changing a daily-use installation casually.

BankID launches through `https://app.bankid.com/` using iOS universal-links-only mode. App returns are received through `app_links`; Flutter's default deep link handler is disabled to avoid competition. Pending flow secrets and app session tokens live in iOS Keychain; the shared Runner entitlements include the secure-storage capability for all build configurations. The old generated password is removed from preferences.

For **production**, or a realistic universal-link test:

```bash
python3 scripts/configure-bankid-ios.py --domain YOUR_API_HOST
```

1. Enable Associated Domains for the app identifier and use an appropriate provisioning profile in Xcode.
2. Set backend `BANKID_RETURN_URL=https://YOUR_API_HOST/bankid/return` and `BANKID_IOS_APP_ID=PREFIX.YOUR_BUNDLE_IDENTIFIER`.
3. Build Flutter with that exact return URL and the correct `API_BASE_URL`.
4. Serve `https://YOUR_API_HOST/.well-known/apple-app-site-association` publicly, as JSON, without authentication or redirects. The API generates it from `BANKID_IOS_APP_ID`, restricted to `/bankid/return`.
5. Install the build and test the link from another app. Apple caches domain associations, so provisioning/CDN issues may delay recognition. If the return opens in a browser, a static fallback asks the participant to return to the app; it never declares success or exposes a session token.

The helper preserves existing HealthKit entitlements and adds the domain you provide. It does not change your bundle ID, developer team or provisioning account. See the [official Flutter/Apple setup instructions](https://docs.flutter.dev/cookbook/navigation/set-up-universal-links).

## Real BankID acceptance test

The following steps require your certificates, published consent, invitation and test device:

1. Open the app, enter the invitation, read the full consent, acknowledge review and choose **Sign with BankID**. Check that BankID shows the intended consent text. Return to the app, see the accepted receipt, then continue to Apple Health.
2. Permit HealthKit read access, preview step data and upload. Check the backend's `info` and `dataUploads` records and the files under `<data-dir>/raw/` belong to the authenticated participant.
3. Check **View signed consent** from the summary, or the document button on the Apple Health screen before uploading. Export the corresponding evidence with the command below and verify it includes the exact request, consent, raw completion, signature and OCSP response.
4. Repeat enrollment using a fresh invitation and **Use BankID on another device**. Scan the changing QR code with the second device. Let a code reach its 30-second scan limit and use **Show a new QR code**. Once BankID has picked up an order, continue collecting its result rather than resetting it on the scan deadline. [Animated QR guidance](https://developers.bankid.com/getting-started/qr-code).
5. Cancel in BankID and in the app. Test an expired/blocked test BankID, a signer that does not match a bound invitation, and an already-used invitation. None should enable uploads.
6. Background or terminate the study app during signing, then reopen it. Saved request IDs recover the existing order. Drop network connectivity around start/completion, then retry. An ambiguous provider result can require a new signature; a callback by itself cannot authenticate the participant.
7. Sign out and use **Already enrolled? Sign in with BankID**. This uses `/auth`. Publish a new consent version, sign in again, and confirm `/sign` is required before uploads resume.
8. Withdraw consent from the receipt. Future uploads must fail, including requests made with an earlier unexpired session. Sign in and review/sign again only if participation is to resume.
9. Confirm expiry after two hours, rejected PocketBase auth-refresh, direct collection mutation denial, and that a different `participantId` in `/info` or `/data` is rejected. Test same-device Wi-Fi/mobile changes: this implementation requires the original caller IP until the grant is completed, so switching networks prompts a restart.

The application accepts only `risk=low`. Moderate, high, unknown and absent risk indications never activate consent or grant access. A valid BankID signature can therefore be retained with a rejected application outcome. This conservative first-release policy needs review with the service owner if real test traffic regularly produces other results. [Collect and risk indications](https://developers.bankid.com/api-references/auth--sign/collect).

For App Store/TestFlight review, prepare a separate test backend and synthetic reviewer invitation with instructions for obtaining a test BankID. Coordinate this with the review process; the production build has no fake-login or fake-consent fallback. [BankID app review guidance](https://developers.bankid.com/support).

## Evidence, maintenance and rotation

```bash
# Stop the API and use its environment and data directory.
./app study export-evidence --dir=./pb_data \
  --signature=SIGNATURE_RECORD_ID \
  --out=/private/evidence-exports/consent-evidence.json

./app study purge-sessions --dir=./pb_data
```

Use encrypted persistent volumes and encrypted backups for the database and raw health uploads. The application-level encryption specifically protects identity and BankID evidence fields.

Evidence export creates a new `0600` file and refuses to overwrite it. It includes identifying information. Completion/signature records are encrypted with AES-256-GCM and bound to their record; personal number lookup uses a study/environment-specific keyed HMAC. Generic record APIs cannot modify immutable consent/identity/evidence records, including through the admin UI. Use the local study commands. PocketBase superusers still have read access to private collections; restrict admin access to operators.

`purge-sessions` clears expired flow credentials, flow identities, invitation credentials and terminal-order operational payloads after a 24-hour grace period, and removes expired app sessions. It retains immutable consent evidence, identity mappings, consent events and minimal order history. It does not erase pending or durably collected work. Schedule it during maintenance; determine the study's retention period for the retained evidence, identities, health data, backups and audit logs separately.

For encryption rotation, add a new random key under a new `encryptionKeys` ID and make it `activeKey`. Keep earlier keys for old records and backups. This changes the key for new writes; it does not rewrite old evidence. Do not replace `identityHmacKey` casually: identity lookup would stop resolving existing participants. Back up all required keys separately from the database and rehearse restoration. Never reuse test keys in production.

For RP certificate rotation, configure the new cert/key as current and retain the old pair using `BANKID_PREVIOUS_CERT_FILE` and `BANKID_PREVIOUS_KEY_FILE` until previous pending orders finish. Each order selects its original certificate by fingerprint. Startup validates certificate validity and logs its expiry. Monitor expiry, `unresolved`/`rejected` counts, stuck `collected` records, database errors and BankID availability. `BANKID_SIGNING_ENABLED=false` stops new flows/orders while the worker can finish existing ones.

The provider verifies BankID signatures; this implementation retains the XML signature, OCSP and complete provider response as evidence, rather than implementing a second XML signature validator. Evidence authentication is checked again before upload authorization. [BankID signature verification guidance](https://developers.bankid.com/how-to-guides/verifying-signatures).

## Deployment and upgrading existing data

[Dockerfile](../api/Dockerfile) builds the Go API, including its internal packages and compiled migrations, then runs as a non-root user. [Deployment template](../api/deploy/api.yaml) uses one replica and `Recreate`, a persistent data volume, and a read-only secret volume. Fill the hostname, image tag, app identifiers and actual proxy CIDRs before deploying; the placeholders deliberately do not enable a working production connection.

Create the cluster secret from protected local files using your existing cluster workflow. Its required filenames are `client.pem`, `client-key.pem`, `bankid-server-ca.pem` and `study-keys.json`. No secret values are included in the manifest. Ensure the runtime UID/group can read the mount and write the persistent volume. Terminate HTTPS at the ingress and keep the PocketBase admin UI off public participant access through your ingress policy. Do not run overlapping replicas against the same database.

PocketBase has been upgraded from 0.22.5 to 0.40.3, and the Dart SDK to 0.25.1. Before production migration, stop the old service and take a restorable backup of the complete data directory, raw uploads, configuration and encryption keys. Rehearse on a copy. The historical Go migrations were adapted for the current API, and the final migration locks participant collection APIs and disables the old password login. Existing participant IDs and legacy records remain; existing users need a correctly delivered invitation and an actual signature. Review the [PocketBase Go upgrade guide](https://pocketbase.io/v023upgrade/go/).

This database change is not a supported automatic downgrade. Roll back by restoring the pre-upgrade backup with the previous binary, not by running the old binary against an upgraded database. Keep any older upload files in their original relative paths when moving data directories.

## API contract and automated checks

Flow authorization is `Bearer <flowID>.<clientSecret>`; the app generates and securely saves the 32-byte random secret before creating a flow. The server stores a keyed digest. Creation and order starts are retry-safe for the same secret/request key. App authorization uses the granted PocketBase token **plus** a private server session record, expires at most two hours after the BankID event, and cannot be refreshed. Enrollment flows last 20 minutes.

| Endpoint | Purpose |
| --- | --- |
| `GET /api/study/consent/current` | Published document and availability. |
| `POST /api/study/enrollments` | `{kind: "enroll"/"login", clientSecret, invitationCode}`. |
| `POST /api/study/bankid-orders` | Flow auth; `{consentVersionId, documentHash, mode: "sameDevice"/"qr", requestKey}`. Omit consent fields for initial returning `/auth`. |
| `GET /api/study/bankid-orders/{id}` | Owner-only cached order status and current QR frame. |
| `POST /api/study/bankid-orders/{id}/return` | Validate nonce and flow possession, then return cached status. |
| `POST /api/study/bankid-orders/{id}/cancel` | Serialize cancellation with collection and acceptance. |
| `POST /api/study/enrollments/{id}/complete` | Obtain a short session or `consentRequired`; a retry returns the same grant. |
| `POST /api/study/enrollments/{id}/abandon` | End the flow and release an unused invitation. |
| `GET /api/study/me`, `POST /api/study/logout` | Validate or revoke the app session. |
| `GET /api/study/consent/receipt`, `POST /api/study/consent/withdraw` | Own receipt and withdrawal. |
| `POST /info`, `POST /data` | Require current signed consent and derive identity from the app session. |

The backend collects independently of app polling, at least two seconds apart per order. It saves the provider result before finalizing consent in a transaction. Lost terminal responses remain unresolved and cannot create access. There is no claim of exactly-once provider delivery. [BankID collect contract](https://developers.bankid.com/api-references/auth--sign/collect), [error handling](https://developers.bankid.com/api-references/errors).

```bash
flutter analyze
flutter test
cd api
go test -race ./...
go vet ./...
go build .
```

Automated tests cover official QR vectors, mutual TLS and server trust, response preservation, signing evidence, wrong signer/risk failures, owner/nonce checks, idempotent completion, returning authentication and re-consent, session expiry/withdrawal/revocation, recovery after a persisted result, ambiguous results, key rotation, proxy handling, upload identity/gzip/chunk behavior, app cold starts, cancellation races and review/QR UI on a small screen. A synthetic 0.22.5 database upgrade was also rehearsed with legacy participant and info records preserved.

Real BankID/device acceptance is still required after configuration. On the implementation machine, Dart analysis/tests and Go checks run; the native iOS build is currently blocked by Xcode reporting the iOS 26.2 platform component is missing. Install that component in **Xcode → Settings → Components**, then build and run on the configured test phone. Automated provider tests do not replace that final device test. Docker image execution was not verified on this machine because its Docker daemon is unavailable.
