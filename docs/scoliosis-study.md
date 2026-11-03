# Skolios och rörelse

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
| PDF p.19, attachment 6 | `deploy/consent/scoliosis-consent.txt` preserves the title and consent paragraphs, including the source's grammatical wording. Blank paper signature/name/address fields are replaced by BankID identity, timestamp and encrypted signature evidence. |
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
guardians must sign. Ages 15–30 consent for themselves. Guardians must be adults,
must match the requested identity, and cannot be the participant. The template
verifies identity, not the legal guardianship relationship. No age selector or
birth-date question establishes access.

`GET /api/movement/window` requires a participant session, current consent and
any required guardian signatures. It derives the window from the saved study
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

The isolated database is `api/.local/scoliosis/pb_data`; its **new** evidence
keyring is `api/.local/scoliosis/secrets/study-keys.json`. Both are ignored by Git.
Back up this keyring with this database. The template's original keyring/database
are untouched. The local launch script consumes the documented test certificate
paths without reading or sourcing `api/.env`.

```sh
cd api
go build -o app .
cd .local/scoliosis
../../app bankid publish-consent \
  --file ../../../deploy/consent/scoliosis-consent.txt \
  --version scoliosis-2026-10-26-v1 --title 'Rörelsemönster vid skolios' \
  --dir=./pb_data
```

The above version has already been published in this local database. Do not run
the publication command again with the same version. Publish a new immutable
version for wording changes. Normal startup applies the questionnaire migration.

From the repository root:

```sh
bash scripts/run-scoliosis-local.sh
flutter run --dart-define-from-file=deploy/flutter-local.json
```

The simulator API is `http://127.0.0.1:8091`. For physical-device or guardian tests,
start `ngrok http http://127.0.0.1:8091`, set `BANKID_PUBLIC_URL` to its HTTPS origin,
then restart the local backend. Run Flutter with that origin as `API_BASE_URL`.
The tunnel is temporary; it is not a production deployment. The test backend
accepts direct loopback connections without forwarded headers while still
requiring valid proxy headers for trusted nonlocal/production proxies.

The development bundle identifier is `se.gu.skoliosrorelse`. The test-only return
scheme remains `researchsteps://bankid/return`, matching the native project and
BankID configuration. Production requires a registered app ID, signed build and
HTTPS associated-domain return links. Android builds are verification artifacts;
the study's Health collection is iPhone-only.

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
BankID signing/return, all six Health permissions and actual sample availability
remain necessary. A real signature cannot be completed by automated fixtures.
The local database and temporary tunnel do not establish the protocol's planned
GU production hosting or researcher access controls.

See `docs/manual-review.md` for device checks and the freshly generated
`app_report.html` for automated results and visual coverage.

## Verification

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
