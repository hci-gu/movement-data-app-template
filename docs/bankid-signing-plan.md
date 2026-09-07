**BankID signing: research and implementation plan**

Implementation is now in the repository. See [BankID setup, credentials and testing](bankid-testing.md) for the actual endpoint names, chosen session/risk policies, operator commands and remaining device acceptance steps. The proposal below is retained as research/design history; `/api/study/bankid-orders` handles both signing and returning authentication, and the client persists its own random flow secret before enrollment creation.


Researched 7 September 2026 against official BankID, PocketBase, and Flutter documentation and the repository source. This is a proposal; no integration code or deployment changes have been made.

Working assumption: participants sign study consent during enrollment, then authorize Apple Health and upload their step data. The first release targets the existing iPhone app. Returning-user BankID login and signing on another device are separate scope decisions.

**Recommendation**

Add a small BankID service inside the existing Go/PocketBase backend. The server owns consent documents, signing orders, identity matching, completion evidence, and upload authorization. Flutter presents the document, launches BankID, and observes the server's result. Start with direct integration in the test environment; use an existing institutional integration provider if the organisation already has one.

Production access requires an agreement and relying-party certificate. BankID offers purchasing through banks or resellers; service and pricing vary. Confirm the organisation's existing procurement route and that its agreement covers signing before committing to a new supplier. [BankID onboarding](https://www.bankid.com/foretag/anslut-foeretag)

**What the repository currently does**

| Location | Finding and consequence |
| --- | --- |
| `lib/screens/login.dart`, `lib/widgets/consent_modal.dart` | Enrollment accepts a checkbox. Replace this with document review and a signing flow. |
| `lib/app_config.dart` | Consent text and `consentVersion = 'template-v1'` live in the app. The signing backend must own immutable versions; the version is not sent by the current registration request. |
| `lib/state/auth.dart`, `lib/storage.dart` | The app creates a random password and saves it in SharedPreferences for later login. This needs an explicit session redesign for BankID. |
| `lib/api.dart` | Dio calls to `/users`, `/info`, and `/data` do not attach the PocketBase auth token. |
| `api/main.go` | Those three POST routes do not require participant authentication. `/users` finds an existing participant and can overwrite its consent boolean without proving account ownership. `/data` uses the supplied participant ID for storage, before looking up the user. |
| `api/migrations/1711536709_collections_snapshot.go` | The checked-in users rules allow public creation and owner updates. There is also an older, unused `consent` collection. Audit actual deployed rules before rollout. |
| `api/go.mod`, `pubspec.lock` | Server is PocketBase `0.22.5`; Dart SDK is `0.23.0`. This is outside the documented compatibility pairing. |
| iOS project | No return-link configuration is present. The bundle ID still reads `com.appademin.swedHeart`; select the final study identity before configuring links. |
| `api/Dockerfile`, `api/deploy/api.yaml` | One backend replica and persistent storage. Docker copies only `main.go` and migrations, so new internal packages must be included in the build. No BankID certificate mount is configured. |

These are source findings, not a penetration test or an inspection of the deployed database. PocketBase's upgrade guide says a `0.22.x` server needs Dart SDK `<0.19.0`; newer servers also change Go routing, hooks, models, and migrations. Plan a separate upgrade to a selected current release and compatible SDK before production integration. Do not copy modern `OnServe` examples into this Echo-based server unchanged. [PocketBase upgrade guide](https://pocketbase.io/v023upgrade/go/), [Dart SDK changelog](https://github.com/pocketbase/dart-sdk/blob/master/CHANGELOG.md)

**Verified BankID protocol**

Use Swedish BankID's Auth & Sign RP API. The documented production base is `https://appapi2.bankid.com/rp/v6.0`; the test counterpart is `https://appapi2.test.bankid.com/rp/v6.0`. Requests use mutually authenticated TLS with separate environment certificates and trusted BankID server CAs. TLS 1.2/1.3 are documented. The current website embeds a schema named `openapi-auth-sign-6.1.11.yaml`; that document revision does not change the documented `/rp/v6.0` URL. [API overview](https://developers.bankid.com/api-references/auth--sign/overview), [Environments](https://developers.bankid.com/getting-started/environments)

The `/sign` request requires `endUserIp` and `userVisibleData`. Encode visible text as UTF-8, then Base64. Optional `userNonVisibleData` is also Base64. The embedded schema limits the encoded strings to 40,000 and 200,000 characters respectively; `returnUrl` is limited to 512. Use `requirement.personalNumber` only when the expected signer is already known; it restricts who can complete the order. It is not the old personal-number-based launch flow. [Signing reference](https://developers.bankid.com/api-references/auth--sign/sign)

Proposed server request, with placeholders below expanded before transmission:

```json
{
  "endUserIp": "<IP observed through trusted ingress>",
  "userVisibleData": "<Base64 of approved consent text>",
  "userNonVisibleData": "<Base64 of immutable signing manifest>",
  "returnUrl": "https://app.example.org/bankid/return#nonce=<random nonce>",
  "returnRisk": true,
  "app": {
    "appIdentifier": "<actual iOS bundle identifier>"
  }
}
```

The response contains `orderRef`, `autoStartToken`, `qrStartToken`, and `qrStartSecret`. Keep order details on the server and return only what the selected app flow needs. [Signing reference](https://developers.bankid.com/api-references/auth--sign/sign)

For iOS, launch `https://app.bankid.com/?autostarttoken=<token>` as a universal link, with universal-links-only behavior. Handle BankID not being installed. Configure our own universal link or custom scheme for the return. Supply the return URL in the server request; the older launch-URL `redirect` mechanism is deprecated. The return link wakes the app; it does not establish success. [Autostart guide](https://developers.bankid.com/how-to-guides/autostart), [Return URL guide](https://developers.bankid.com/how-to-guides/return-url)

The server calls `POST /collect` with `{"orderRef":"..."}` every two seconds while pending. Completion returns the signer, a Base64 XML signature, and an OCSP response. Stop on a terminal state, map `hintCode` to the official user messages, and tolerate new hint codes. Cancellation uses `POST /cancel` with the order reference. [Collect reference](https://developers.bankid.com/api-references/auth--sign/collect), [Cancel reference](https://developers.bankid.com/api-references/auth--sign/cancel), [User messages](https://developers.bankid.com/resources/user-messages)

BankID already verifies returned signatures and certificates; custom XML signature verification is not required for normal integration. We remain responsible for saving the evidence. Independent later verification needs the signature specification and signing trust roots, available by request. The result is XML signature evidence; a PDF receipt would be a separate artifact produced by us. [Signature verification guide](https://developers.bankid.com/how-to-guides/verifying-signatures)

Research limitation: the standalone YAML download returned a CAPTCHA. The signing field limits above were verified from the structured schema embedded in the public signing page's JavaScript asset. No live signing transaction was performed.

**Proposed enrollment flow**

```mermaid
sequenceDiagram
    participant U as Participant
    participant A as Flutter app
    participant P as Go / PocketBase
    participant B as BankID API
    participant M as BankID app
    U->>A: Open invitation and review consent
    A->>P: Start signing for reviewed version
    P->>P: Validate invitation; freeze document and manifest
    P->>B: POST /sign over mTLS
    B-->>P: Order reference and launch tokens
    P-->>A: Local signing ID and launch URL
    A->>M: Open BankID universal link
    U->>M: Review and sign
    loop Pending, every 2 seconds
        P->>B: POST /collect
    end
    B-->>P: Complete with identity and evidence
    P->>P: Persist evidence; validate signer; activate consent
    M-->>A: Return link
    A->>P: Read signing status with bound session
    P-->>A: Confirm accepted consent
    A->>P: Exchange completed enrollment for app session
    U->>A: Authorize Apple Health and upload
```

Order the return and collection independently: either can happen first. Persist the signing ID and session secret before launching BankID so the app can resume after termination. Backend collection continues while Flutter is backgrounded.

An arbitrary typed participant ID must not establish ownership. Proposed default: a random, expiring, single-use invitation selects a server-side participant reservation. Compare the completed BankID identity with an expected identity when the study has one. Otherwise explicitly treat the result as the first identity binding for that invitation; it does not prove a pre-existing clinical-record match. Never take over an existing account based on its participant ID alone.

**Consent and identity storage**

Use separate collections; the names below are proposals.

| Collection | Contents and access |
| --- | --- |
| `consent_versions` | Study ID, version, language, exact displayed text, exact document bytes if applicable, SHA-256 hashes, publication status. Published versions cannot be edited. Serve approved content through a read-only endpoint. |
| `enrollment_sessions` | Invitation reservation, hashed high-entropy session secret, expiry, device/session binding, claimed state. Server access only. |
| `bankid_orders` | Local signing ID, enrollment/user owner, environment, encrypted order/launch secrets, request bytes, nonce hash, initiation time, status/hint, collection scheduling and terminal result. Server access only. |
| `consent_signatures` | Unique `(environment, orderRef)`, consent version, participant reference, exact visible/non-visible signed payloads, complete provider response, XML signature, OCSP response, server receipt timestamp, acceptance outcome. Server writes only; restricted evidence access. |
| `participant_identities` | Protected mapping between BankID identity and study participant. Keep identity data out of health upload paths and ordinary client-visible user records. |
| `consent_events` | Accepted, superseded, or withdrawn events, actor and timestamp. Preserve the signed evidence when consent changes; apply the approved retention policy. |
| `app_sessions` | Session identifier, user, authentication event, expiry and revocation state if needed to enforce session limits beyond PocketBase JWT validation. |

The manifest should contain a schema version, signing purpose, study ID, consent-version ID, document hash, participant reservation, signing-session ID, and random nonce. Serialize once and retain those exact bytes. Include the actual consent commitments visibly; hidden metadata binds records and hashes, not undisclosed consent terms. Any separately reviewed document must be frozen and hash-bound before the order starts.

Encrypt identity/evidence storage and backups with keys managed separately from the data; an ordinary SQLite file or Base64 field is not encryption. If lookup needs a pseudonymous identifier, use a keyed HMAC of the normalized identity with study separation, not a plain hash of an enumerable personal number. The signature itself still contains identifying information. Document evidence access, key rotation, export and retention requirements with the study owner.

**App-facing API proposal**

These are our endpoints, distinct from BankID's endpoints. Responses are minimal DTOs rather than exposed order records.

| Endpoint | Purpose and authorization |
| --- | --- |
| `GET /api/study/consent/current` | Published consent ID, version, text and document hashes for review. |
| `POST /api/study/enrollments` | Redeem invitation into a limited, expiring enrollment session. Rate limited; return a random secret once and store only its hash. |
| `POST /api/study/consent-signings` | Enrollment authorization plus `consentVersionId`, `documentHash`, launch mode and idempotency key. Resolve participant and signing text on the server. Reject stale review with a re-review response. |
| `GET /api/study/consent-signings/{id}` | Owner-only cached status, message code and minimal consent receipt. Never call BankID directly from each client poll. |
| `POST /api/study/consent-signings/{id}/cancel` | Owner-only cancellation; serialize with collection/finalization. |
| `POST /api/study/enrollments/{id}/complete` | Exchange a successfully accepted signing for a short-lived app session; retry-safe for the same bound caller without creating another account. |
| `POST /info`, `POST /data` | Require participant authentication and active required consent. Derive the participant from the authenticated record before processing data or opening files. |

Return `Cache-Control: no-store` for session/order responses. Use a configured, allowlisted return URL plus nonce; never accept an arbitrary redirect destination. Session secrets belong in authorization headers, not callback URLs or logs. Validate callback nonce and possession of the original session; use the same-device channel-binding checks described by BankID. Treat network/device changes explicitly rather than accepting a new caller on possession of a signing ID.

After signing, check identity, expected consent version, session ownership, evidence presence and the configured risk policy before authorizing uploads. With `returnRisk: true`, define handling for all values and missing results; high-risk completions must not activate enrollment. Separate provider completion from application acceptance. [Collect risk response](https://developers.bankid.com/api-references/auth--sign/collect)

Retire public `/users` consent mutation. Lock generic collection creation and protect consent, identity, status, and participant-link fields from direct owner updates. Apply equivalent enforcement to any built-in collection API that remains exposed. Client routing alone cannot enforce consent. Existing boolean consent remains legacy evidence and must not be migrated into a fabricated BankID signature.

**Session design is a required decision**

BankID's public terms describe restrictions on identity switching, including issuing other credentials from a BankID verification and preserving identification after more than two hours of inactivity. The existing permanent generated-password flow cannot simply be retained as a BankID login substitute. Confirm the applicable agreement with the institutional service owner. [BankID identity-switching guidance](https://www.bankid.com/foretag/anslut-foeretag)

Proposed first-release behavior: a short session for consent and upload, backed by PocketBase authentication after accepted completion. Use at most two hours absolute lifetime initially, with no silent renewal beyond that authentication event. Enforce expiry/revocation on the server, including standard PocketBase auth-refresh and any accessible record APIs. A later login uses `/auth`; a new consent version uses `/sign`. If returning access is required at launch, include `/auth` in the first release. Alternatively keep enrollment as a one-session workflow until return access is designed. Store resumable session secrets in iOS Keychain-backed storage and clear both Riverpod state and `pb.authStore` on logout.

**Reliability and implementation details**

Build `api/internal/bankid` around an injectable interface with `Sign`, `Collect`, and `Cancel`; use Go `net/http`, `crypto/tls`, Base64, and HMAC primitives. Configure a reusable HTTP client, bounded response sizes, timeouts, strict server certificate verification, and environment-specific certificate material. Resolve `endUserIp` only through trusted proxy configuration; disregard untrusted forwarded headers and client-supplied IP fields.

Implement a durable order worker with one collector per order, startup recovery, controlled shutdown and a lease if multiple workers are ever introduced. Keep network requests outside database transactions. Use unique keys and atomic finalization to prevent duplicate signatures/accounts. Persist the complete response before reporting success; retain it for repeated client status reads.

BankID does not let us repeatedly retrieve an already-collected terminal result, and its result availability is short. A network/process failure between provider completion and durable local storage can therefore require a fresh signature. Represent this as an unresolved attempt, do not infer success, and do not claim exactly-once delivery from BankID. Likewise, do not automatically retry an ambiguous `/sign` timeout as though it were idempotent. [BankID error semantics](https://developers.bankid.com/api-references/errors)

Use separate application states such as `creating`, `pending`, `accepted`, `rejected`, `failed`, `cancelled`, and `unresolved`; retain the original provider status separately. Track deadlines for launch/pickup separately from the time allowed to finish signing. Collect/cancel races must never turn a completed signature into an assertion that nothing was signed. Stopping an already accepted consent is a withdrawal action, not cancellation of the order.

In Flutter, add a signing controller/state provider and a dedicated review/progress screen. Use `url_launcher` with the appropriate external-app launch mode or a small native bridge that ensures the documented iOS behavior. Handle cold-start return links and lifecycle resume without requiring the callback to arrive. Associate the production domain through AASA and iOS Associated Domains, and add a callback route that can be reached before login. [Flutter universal links](https://docs.flutter.dev/cookbook/navigation/set-up-universal-links), [url_launcher launch modes](https://pub.dev/documentation/url_launcher/latest/url_launcher/LaunchMode.html)

**Optional signing on another device**

Use an animated QR code, refreshed every second. Compute it on the server:

```text
t = decimal whole seconds since receiving the order response
mac = lowercase_hex(HMAC_SHA256(UTF8(qrStartSecret), UTF8(t)))
qr = "bankid." + qrStartToken + "." + t + "." + mac
```

Keep `qrStartSecret` exclusively on the backend and supply only the current QR payload through an authenticated endpoint. Do not precompute future frames. Current guidance gives a 30-second scan window; extending the UI requires new orders and orderly cancellation/replacement. Once picked up, follow collection state rather than replacing an order being signed. Static QR support was removed in 2025. [QR guide](https://developers.bankid.com/getting-started/qr-code), [Static QR removal](https://developers.bankid.com/news/static-qr-code-to-be-removed-april-30th-2025)

**Implementation sequence and acceptance checks**

1. **Confirm product and service inputs.** Study owner supplies approved consent text, participant-to-person matching policy, evidence retention/export requirements, production agreement route, app identity and return domain. Decide whether returning access and another-device signing are in the initial release. Test integration can proceed while procurement is pending.
2. **Align the platform and close enrollment bypasses.** Upgrade PocketBase/Go and align the Dart SDK in a separate change. Test both a new database and upgrade of a backed-up representative database. Add authenticated uploads, immutable identity/consent fields and migration behavior for legacy users. Verify unsigned, unauthenticated and cross-participant requests fail before filesystem writes. A temporary test spike on `0.22.x` must use that version's APIs and compatible SDK.
3. **Implement the signing service and evidence model.** Add migrations, server-owned document versions, invitation sessions, mTLS client, durable collection, cancellation and atomic acceptance. Use a fake BankID transport to exercise terminal states and storage failures deterministically. Acceptance: an accepted signature always points to preserved evidence and the exact reviewed document.
4. **Integrate the iPhone flow.** Add consent review, launch, callback/resume handling, progress/error messages and receipt; replace checkbox submission and permanent password restoration. Attach app authentication to Dio. Gate HealthKit/upload navigation on server-confirmed consent. Acceptance: review → BankID → return → consent receipt → HealthKit → upload works on a physical iPhone, including relaunch after termination.
5. **Exercise the real BankID test environment.** Use the official test certificate, test-configured BankID app, and synthetic identities. Test cancel, missing app, launch timeout, blocked identity, wrong signer, stale consent, duplicate taps, expired sessions, callback replay, network changes, and concurrent collect/cancel. If QR is included, test with two physical devices. [BankID test setup](https://developers.bankid.com/test-portal/test-information)
6. **Prepare production operations and pilot.** Mount certificates/keys through managed secrets, restrict evidence access, rehearse backup restore and certificate rotation, redact request bodies/tokens/identities from logs, and monitor order failures and certificate expiry. Use a feature flag and a controlled pilot. Provide a review/test path for App Store review, isolated from production enrollment. Disable new signing during rollback without losing previously saved evidence or bypassing consent checks.

Regression coverage should specifically prove: the client cannot manufacture a signed state; one participant cannot inspect or cancel another's order; consent edits force review of a new version; failed/missing evidence prevents acceptance; authenticated upload retries still preserve chunk ordering; and short-session limits cannot be bypassed using built-in PocketBase refresh or password endpoints.

No application tests were run for this document-only research task. End-to-end feasibility still needs a test certificate, a test BankID on a physical device, and the implemented flow.
