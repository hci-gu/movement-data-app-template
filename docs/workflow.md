# Building a study app from the template

1. Read the supplied files in `context/` for the study purpose, participant groups,
   consent, questionnaire, data collection and branding. Record source locations,
   agreed decisions and unresolved questions in the study's documentation. Read
   [design and copy guidelines](design-guidelines.md) before adapting any screen.
2. Set up the Go/PocketBase server in `api/` using
   [BankID setup and testing](bankid-testing.md). Configure the intended database,
   BankID environment, certificates, evidence keyring and return links.
3. **Populate and verify the study's BankID consent in PocketBase**, following
   [Publish study consent](#publish-study-consent) below. Updating app copy or
   adding a text file alone does not populate the consent record.
4. Populate questionnaire definitions from `context/` with a reproducible Go
   migration. Follow [questionnaire data](questionnaire-data.md) for wording,
   choices, settings, branching and verification; apply the seed to the intended
   database and check the records.
5. Adapt the source using [the customization guide](customization.md). Shared
   widgets live in `lib/ui/`; questionnaire inputs in `lib/questionnaires/widgets/`;
   study copy in `lib/study/`; flow composition in `lib/app/`; collection behavior
   in `lib/steps/`. Review native names, identifiers and permission descriptions.
   Apply [delivery readiness](#delivery-readiness): final study content and working
   enrollment are requirements for the generated app.
6. **Create and install a study-specific app icon** when image generation tools
   are available. Normalize the artwork to 1024 × 1024 and run the checked-in
   resizing/installation helper for every Xcode icon slot, following
   [Study app icon](#study-app-icon) below. Validate the generated assets before
   the final build.
7. Install dependencies with `flutter pub get`, then run `flutter analyze` and
   `flutter test`. In `api/`, run `go test ./...` and `go build -o app .`.
   Build with `flutter build apk --release` and `flutter build ios --simulator`.
   The app's main target is iOS; complete [manual review](manual-review.md),
   including BankID handoff and HealthKit permission checks on a physical iPhone
   before participant use. Report any unavailable checks or setup steps.
8. **End by showcasing the finished app in `app_report.html`.** Follow
   [Demo gallery and app report](app-report.md): update preview cases, generate
   the report with `--study` and the study consent source, inspect every screen
   and scroll continuation, fix rendering problems, and regenerate after the last
   change. Link the generated report in the final response. The report must use
   the adapted app's content and record actual verification results. Review the
   [visual acceptance criteria](design-guidelines.md#visual-acceptance), including
   small screens and enlarged text.

## Delivery readiness

Treat a study adaptation as an app intended for deployment and testing against
the configured backend. Complete consent publication, BankID configuration and
the supported participant flow. Final participant screens must contain zero
unfinished placeholders: replace template names, contacts, icons and permission
copy; supply approved consent and source-backed questionnaire content.

Do not add “study not open yet”, “coming soon”, draft banners, fake success,
inactive registration or similar workarounds because inputs or setup are missing.
Keep unresolved requirements and newly drafted text in developer/owner review
documents. Resolve them with the supplied sources or study owner; if they remain
unresolved, report the precise delivery blocker and do not claim production
readiness. Do not invent approval or bypass consent/security enforcement.

Actual network/BankID failures still need clear participant-facing error and retry
states. Availability restrictions must come from an explicit study requirement
and real state, never an agent's assumption about readiness.

The report's `--study` mode rejects template source content, known default study
copy, example contacts and explicit draft markers. It does not establish ethical
approval, recognize all unfinished wording or verify a live backend. Review all
copy and assets, verify `/api/consent/current` and a complete signing flow against
the intended environment, and record physical-device results separately.

## Publish study consent

Find the relevant consent text in `context/`, including attachments to ethics
applications, participant information and any guardian material. Read the actual
documents; filenames and the template's example text are insufficient. Preserve
the source wording and record which file, section/pages and version it came from.
Check that the text covers the intended signing audience and data collection.
The template currently publishes one consent document used by its signing flow;
if the study requires distinct participant/guardian texts or a different consent
process, adapt that flow before use.

Follow [Age and consent routing](#age-and-consent-routing) when adapting these
audiences; age comes from BankID, without a participant age-group selector.

Extract the agreed consent into a UTF-8 text file, for example
`deploy/consent/study-consent.txt`. Check extraction for missing paragraphs,
broken characters and lost headings. If approved wording is missing or conflicting,
ask the study owner to resolve it; clearly label newly drafted text in owner-review
documents and keep it out of participant-facing screens until resolved.
`deploy/test-consent.txt` is for technical testing only and must not become the
study's research consent.

Publish the agreed text to the **database actually used by the app's backend**.
From `api/`, after configuring that environment, use:

```sh
go build -o app .
./app bankid publish-consent \
  --file ../deploy/consent/study-consent.txt \
  --version study-consent-v1 \
  --title 'Study consent' \
  --dir=./pb_data
```

Replace the file, version, title and data directory with the study's actual values.
For a deployed backend, copy the file and run the command in that deployment with
its persistent data directory; see [BankID setup](bankid-testing.md) for the
deployment command pattern. Publishing into a temporary or unrelated local
database does not configure the app's backend.

The command creates the immutable `consent_texts` record, computes its
`documentHash`, sets `current: true`, and retires the previous current version.
Never hand-edit a published text or its hash. Use a new version when wording
changes; existing participants must sign the new version before further uploads.

Verify all of the following before calling consent setup complete:

- PocketBase contains the intended `version`, `title`, full `text` and
  `documentHash`, with exactly one current consent record.
- `GET /api/consent/current` on the app's configured `API_BASE_URL` returns that
  same document, and `bankidAvailable` is true when BankID is configured.
- The app displays the full study consent before signing and a test signing
  produces a receipt for that version.

Record the published version and environment. If publication or signing could
not be performed, state exactly what remains; a prepared file or command is not
evidence that PocketBase was populated.

## Age and consent routing

Determine participant age on the backend from the personal number in the
successful, verified BankID response (`completionData.user.personalNumber`).
Use that identity to apply the study's age thresholds and guardian requirements.
Do not ask participants whether they are under 15, under 18, or in another age
group, or ask for their birth date to establish those facts. The app follows the
backend's verified consent/guardian state; a client-supplied age or toggle must
never determine access.

The template already does this for its under-18 guardian requirement. Adapt the
threshold to the study requirements in `context/` when needed, keeping the
decision on the backend. Verify identities on both sides of the threshold and
on the relevant birthday.

For a new participant, verified age becomes available when BankID completes.
The current template presents a shared consent document before that signature,
then determines guardian requirements from the returned identity. If a study
requires different age-specific documents **before signing**, adapt the sequence
to establish the verified identity before selecting and presenting the document
for consent signing. Do not solve that sequencing problem with an age toggle.
Keep the recorded consent tied to the exact text shown and signed; discovering
the participant's age does not turn a signature into consent to a different text.

## Study app icon

When an agent has access to image generation tools, use them to create a custom
icon that fits the study's subject, participants and supplied branding in
`context/`. Save the selected original as `assets/icon/source.png`, at least
1024 pixels on each side. Use a clear motif, an opaque background and square
corners; keep important content away from the system's mask. If generation is
unavailable, use final supplied study artwork. Missing artwork is an unresolved
delivery requirement; do not call a study app with a placeholder icon complete.

Follow [App icon generation and installation](app-icons.md) for Apple's current
requirements, prompt guidance, dependencies, supported inputs and verification.
With Python 3.11+ and the icon dependency environment prepared, run:

```sh
build/icon-tools/bin/python scripts/prepare-app-icons.py assets/icon/source.png
build/icon-tools/bin/python scripts/prepare-app-icons.py --check
```

The helper saves the exact 1024 × 1024 RGB/sRGB master at
`assets/icon/app-icon-1024.png`, generates 13 PNGs for all 19 iPhone/iPad/App Store
entries, installs them into `ios/Runner/Assets.xcassets/AppIcon.appiconset/`, and
updates `Contents.json`. It verifies dimensions, alpha removal, matching artwork
and the project's `AppIcon` build setting. Cropping and flattening transparent
artwork require explicit options described in the guide.

Inspect the master and small exports, build iOS, and inspect the installed icon.
The App Store icon travels with the signed app build; validate the archive before
upload. This helper prepares assets and does not publish the app. Also replace
Android/web icon assets if those platforms are included in the study app.
