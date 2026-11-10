# Skolios och rörelse

> KontoAPI authenticates identity; consent is then explicitly accepted in the app or guardian browser. BankID does not sign the study text. Historical verification below concerns the former direct-BankID implementation.

This checkout implements **Rörelsemönster före och efter behandling av adolescent
idiopatisk skolios**, case 2025-08213-01, for local development and device testing.
The research principal is Västra Götalandsregionen; the study is based at
Ortopedkliniken, Sahlgrenska universitetssjukhuset. Olof Westin is the responsible
researcher and Sara Knutsson is the doctoral researcher.

## Sources and decisions

The supplied PDF in `context/` is the combined ethics application and attachments.
Page numbers below refer to the PDF, starting at 1, rather than attachment footers.
Original PDF and DOCX files have not been modified.

| Source | Implementation |
| --- | --- |
| Application §§3.1, 4.1, 6.1, 8.1–8.6, 9.1.1 | Ages 10–30; iPhone; treatment in 2020–2027; one retrospective extraction; one year before to five years after treatment; guardian consent below 15. Clinical inclusion/exclusion and previous iPhone ownership are assessed by the research team. |
| PDF p.19, attachment 6 | `deploy/consent/scoliosis-consent.txt` preserves the title and consent paragraphs, including the source's grammatical wording. Blank paper signature/name/address fields are replaced by verified identity, explicit acceptance, timestamp and encrypted evidence. |
| PDF pp.20–31, attachments 2–5 | Source for Swedish study information, voluntariness, data rights and researcher contacts. App copy describes the actual electronic flow; the source's paper/email distribution process is not represented as an app action. |
| PDF p.52, decision dated 2026-01-07 | Requires separate participant (15+) and guardian forms, plus the child's right to decline even when guardians agree. The latter is visible in study information and guardian screens. See release conditions below. |
| `context/scolios_formulär.docx` | Thirteen question records, two option sets, one once-only questionnaire. Wording and option order preserved, including the source's omitted “ha” in the surgery activity question. Month-only menarche date; surgery and brace branches; full-time and night brace adherence branches; menarche shown only for the verified female registered category. |

Owner decisions in this session, 2026-10-26:

- Use the original combined consent form for now. This is an explicit local-build
  decision, not confirmation that the ethics decision's separate-form condition
  has been satisfied.
- Use a new local backend, not the shared deployed BankID test database.
- Use the six measures from the protocol and participant information. Attachment
  8's conflicting `walking_steadiness` entry is excluded; walking/running distance
  is included.
- Require treatment type and the matching treatment date before submitting the
  questionnaire. Other questions remain optional. Matching client/server study
  rules implement this outside widget settings.

Recruitment counts differ between the application (200, with a 20-person pilot)
and DOCX (30 per treatment group). They are not app eligibility quotas or
participant-facing promises. The research team must settle recruitment planning.
No numeric bounds were supplied for exercise frequency or brace adherence, so
the seed adds units without invented ranges. These inputs use the template's
integer control; accepting fractional answers would require an agreed revision.

## Implemented collection and authorization

BankID supplies verified identity. Participants aged 10–14 need all selected
guardians (one or two); the guardian form defaults to two and explains that all
guardians must approve. Ages 15–30 consent for themselves. Guardians must be adults,
must match the requested identity, and cannot be the participant. The template
verifies identity, not the legal guardianship relationship. No age selector or
birth-date question establishes access.

`GET /api/movement/window` requires a participant session, current consent and
any required guardian approvals. It derives the window from the saved study
response, not from client-supplied upload dates. Treatment must be in 2020–2027
and no later than today. Dates use Swedish calendar boundaries; leap-day
anniversaries clamp to February 28. The final anniversary day is included, capped
at the time of extraction. There is no background or future collection.

The client requests read access only for steps, walking step length, walking and
running distance, walking speed, walking asymmetry percentage and walking double
support percentage. It filters intervals to the window. `/data` independently
checks the participant, consent, guardians, saved treatment, type, unit, iOS
source platform, finite nonnegative value and interval before touching storage.
Native values and source metadata are preserved; sample timestamps are
canonicalized to UTC for unambiguous server checks and researcher export.

The preview includes all six types, sample ranges and means, sources and a step
chart. Daily step totals retain the template's sample-sum behavior, including
possible overlapping phone/watch samples; this is not a deduplicated Apple Health
daily total. Non-step measures are never added to step totals. Percent values
remain raw fractions in storage and are multiplied by 100 only for display,
following [Apple's percent-unit documentation](https://developer.apple.com/documentation/healthkit/hkunit/percent()).

Empty or denied reads and partial uploads never mark completion. Completion uses
`<API>:scoliosis-movement-v1:<participant>:completedAt`, so historical step-only
completion cannot satisfy this six-measure task. Retrying begins at chunk zero.
Questionnaire access remains independent of Health availability.

## Local backend

The fresh KontoAPI test database is `api/.local/scoliosis-kontoapi/pb_data`,
with keyring `api/.local/scoliosis-kontoapi/study-keys.json`. The setup publishes
`scoliosis-kontoapi-v1` from `deploy/consent/scoliosis-consent.txt` and applies
the study questionnaire migration. This is the original combined form, retained
for local testing only; the separate-form release condition below remains open.
The previous direct-BankID database is incompatible and is left untouched.

```sh
bash scripts/run-scoliosis-local.sh --prepare-only
bash scripts/run-scoliosis-local.sh
flutter run --dart-define-from-file=deploy/flutter-local.json
```

The runner uses the credential interface documented in [KontoAPI testing](bankid-testing.md),
requires the GU VPN for live authentication, and listens on port 8091. To test
guardians, pass `--public-url https://YOUR_HTTPS_HOST --trusted-proxies
127.0.0.1/32,::1/128` for a loopback tunnel. Return manually after BankID.
Back up the keyring with any retained evidence; use a fresh database to reset
this disposable setup. The iPhone must be able to reach the backend.

The development bundle identifier is `se.gu.skoliosrorelse`. Android builds
are verification artifacts; the study's Health collection is iPhone-only.

## Remote test deployment

On 2026-11-10, the test API at `https://movement-api.test.appadem.in` was rebuilt
and deployed from `test-scolios-2` using image
`registry.k8s.gu.se/appademin/movement-api:0.0.6-scolios-simulation`, digest
`sha256:8135d5051786cecce8b008900856d400befb74c3831e86cc00ac0a556031fd51`.
OpenShift reports one ready replica with zero restarts. The pod image digest
matches the published image.

The simulation uses `/pb/pb_data/simulation-v1` on the existing Bound PVC and
Secret `movement-study-keys-simulation-v1`. The previous ordinary test database
and original keyring remain intact. The base `deploy/` manifest retains image
`0.0.4-scolios-test` for returning to that ordinary test environment.

The init container published `scoliosis-kontoapi-v1` as current (record
`94xbayvqplwfdvf`). Public HTTPS health returned 200 with
`X-Study-Simulation: scoliosis-test-v1`; anonymous session and consent requests
returned 401, and mapping-collection access returned 403. The administration
page returned 200. The in-pod KontoAPI diagnostic created an authentication order,
retrieved its QR image and received pending status without creating a participant
session or consent.

The database's existing superuser and mapping were retained. Enter or edit
mappings in `test_identity_mappings`; see
[test identities](test-identities.md). Real BankID completion, explicit consent,
guardian approval and HealthKit remain device checks. This is a test deployment,
not approval for recruitment.

At the owner's request, this rollout removes `simulation_state` and its startup
checks, plus all app and guardian-page simulation banners. The removal migration
was verified in the live database: zero marker collections, with unchanged counts
for the superuser, mapping, participants, authentication and consent records, and
all 13 questions. No keyring or study database was replaced.

Mapping creation previously rejected `YYYYMMDD-XXXX` before PocketBase could show
the field error, producing the generic “Failed to create record” message. Creation
and updates now accept that format and surrounding whitespace, store 12 digits,
and show field-specific validation messages. Regression tests cover the dashboard
payload, normalized identity lookup, invalid dates and malformed input.

## Test identity deployment preparation — 2026-11-10

The owner requested editable real-to-test BankID mappings. `test-scolios-2`
implements the private `test_identity_mappings` collection with participant and
guardian roles. Mapping edits invalidate previous sessions/approvals and start
fresh participant progress. Real KontoAPI authentication evidence remains
unchanged. Study eligibility stays 10–30, with guardian approval below 15.
The equivalent collection workflow is also maintained on `main`, adapted to
that branch's direct signing implementation.

`bash scripts/prepare-scoliosis-simulation.sh` built the local `linux/amd64`
image `registry.k8s.gu.se/appademin/movement-api:0.0.6-scolios-simulation` and
rendered `build/scoliosis-simulation-manifest.yaml`. OpenShift server-side dry
run passed. The overlay uses a separate directory on the existing PVC and a
new evidence keyring, preserving the existing deployment's database. The owner
subsequently authorized the push and rollout; deployed verification is recorded
above. Follow [test identities](test-identities.md) to configure the new
database's superuser and enter mappings through PocketBase.

Verification completed:

- Ordinary and simulation backend suites, race checks, and both Go builds passed.
  Tests include superuser mapping CRUD, closed participant access, revision
  revocation, birthdays, provider failures and explicit one/two guardian approvals.
- Flutter analysis and all 170 tests passed; Android release and iOS simulator
  builds passed. Python report tests and changed helper syntax checks passed.
- Container checks passed under arbitrary UID `1001230000`, group `0`, with a
  read-only root filesystem: idempotent study-consent initialization, health 200
  with the simulation header and anonymous session 401. The former marker-based
  startup check has since been removed at the owner's request.
- `api/.local/scoliosis-simulation/pb_data` contains all 13 study questions,
  zero mappings and current `scoliosis-kontoapi-v1` consent matching the source
  text and SHA-256. Existing local databases were retained.
- The refreshed study report contains 84 views and 88 screenshots, including the
  missing-mapping error and ordinary layouts without banners. All views/continuations and
  small-phone/2× text captures were reviewed. Search, enlargement and offline
  image loading passed in Chrome without console errors.

Real BankID completion, guardian interaction and HealthKit permissions/data on
an iPhone remain manual checks. The existing consent release conditions still
apply to this test deployment.

## Artwork

Generated with the built-in image-generation tool. Original:
`assets/icon/source.png`; normalized master: `assets/icon/app-icon-1024.png`.
All 19 Xcode slots and Android/web icons use the new artwork.

Prompt: “Use case: logo-brand. Asset type: final iOS app icon, square 1024x1024
or larger, for a Swedish clinical research app studying movement before and after
scoliosis treatment. Create a refined abstract walking figure with a gentle
curved spine motif, formed from soft sculptural ivory and mint shapes on a solid
deep petroleum teal background. Simple balanced iconic silhouette, warm and
reassuring but not childish, subtle dimensional shading, very legible at 40
pixels. Centered generous margins, opaque full-bleed square canvas with square
corners, no text, no lettering, no numbers, no institutional logos, no medical
cross, no border, no mockup. This is research branding, no claims of treatment
or recovery.”

## Conditions before participant use

This local build is **not approved for recruitment or production**. The owner
retained the original combined consent temporarily; the separate-form condition
in the ethics decision remains outstanding. Final revised forms and their
age-appropriate information must be supplied and published. If distinct
documents must be selected before signing, implement verified identity before
document selection as described in `docs/workflow.md`; do not add an age toggle.

The study owner also needs to confirm the electronic BankID process in place of
the paper/email process described in the source. Physical iPhone checks of test
BankID authentication and explicit consent, all six Health permissions and actual sample availability
remain necessary. A real authentication cannot be completed by automated fixtures.
The local database and temporary tunnel do not establish the protocol's planned
GU production hosting or researcher access controls.

See `docs/manual-review.md` for device checks and the freshly generated
`app_report.html` for automated results and visual coverage.

## Historical verification (previous direct-BankID implementation)

Completed on 2026-10-26:

- `flutter analyze`: no issues; complete Flutter suite: 161 tests passed.
- `go test ./...` and `go build -o app .`: passed. Coverage includes treatment
  prerequisites, six-type upload restrictions, collection boundaries, guardian
  requirements, consent enforcement and session isolation.
- Android release APK and iOS simulator build: passed. The simulator app was
  installed and launched on iPhone 17 Pro / iOS 26.3; its introduction and artwork
  were inspected. No signed physical-device or App Store archive was produced.
- `prepare-app-icons.py --check`: verified the master and all 19 Xcode slots.
  Python report tests (7), helper syntax and local launcher shell syntax passed.
- Final report generated with `--study`: 76 views, 83 screenshots, all 13
  questionnaire records in editable and saved states. All screenshots and long
  continuations were visually reviewed, along with 31 small-phone/2× text
  captures. The review corrected image-decoding timing in captures and saved
  choice/date contrast.
- Report search, image enlargement and dismissal passed in Chrome. With browser
  networking disabled, the full standalone document was reloaded from its HTML:
  all 76 previews were present, all image sources were embedded, zero external
  resource requests and zero console errors. The CLI blocks `file:` navigation,
  so this used a local HTTP load followed by offline document reload.
- Actual isolated database: one study questionnaire with 13 questions; current
  consent matched the source file, hash
  `1e59826ab1b08b499fbd0616f92c69b3997e875d974e14294d18726253059cd6`.
  Direct local consent API returned 200; unauthenticated movement window returned
  401. A real BankID test QR request returned pending with a QR payload and was
  successfully cancelled. No participant identity was signed or Health data read.

On this workstation, inherited `JAVA_HOME` pointed to a removed JDK. The final
report/build command used the installed JDK 21 without changing shell settings:

```sh
JAVA_HOME=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home \
  python3 scripts/build-app-report.py --study \
  --consent deploy/consent/scoliosis-consent.txt \
  --consent-title 'Rörelsemönster vid skolios'
```

Interactive Simulator automation could not continue because the computer-use
tool did not approve access to Simulator. The physical-device and consent
conditions above remain unresolved; automated results do not replace them.
