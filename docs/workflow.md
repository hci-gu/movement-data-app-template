# Building a study app from the template

1. Read the supplied files in `context/` for the study purpose, participant groups,
   consent, questionnaire, data collection and branding. Record source locations,
   agreed decisions and unresolved questions in the study's documentation.
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
6. **Create and install a study-specific app icon** when image generation tools
   are available, following [Study app icon](#study-app-icon) below.
7. Install dependencies with `flutter pub get`, then run `flutter analyze` and
   `flutter test`. In `api/`, run `go test ./...` and `go build -o app .`.
   Build with `flutter build apk --release` and `flutter build ios --simulator`.
   The app's main target is iOS; complete [manual review](manual-review.md),
   including BankID handoff and HealthKit permission checks on a physical iPhone
   before participant use. Report any unavailable checks or setup steps.

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
ask the study owner to resolve it; clearly label newly drafted text for review.
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
`context/`. Use a simple, recognizable motif, strong contrast and generous margins
so it remains legible at small sizes. Follow any supplied brand requirements.
Save the square master, for example as `assets/icon/app-icon.png`, at least
1024 × 1024 pixels. Use an opaque background for the iOS assets and leave corners
square so the operating system can apply its mask.

Generate the artwork once, then resize/export consistent PNG variants into the
platform projects. Inspect their actual manifests and asset catalogs: saving a
master or replacing just one PNG does not install the app icon. For the current
template, populate these destinations:

| Platform | Destination | Pixel sizes / mapping |
| --- | --- | --- |
| iOS | `ios/Runner/Assets.xcassets/AppIcon.appiconset/` | Each `Contents.json` slot needs a matching PNG at `size × scale`. Current iPhone slots: 40, 60, 29, 58, 87, 80, 120, 120 and 180 px. iPad slots: 20, 40, 29, 58, 40, 80, 76, 152 and 167 px. Marketing: 1024 px (`appstore.png`). |
| Android | `android/app/src/main/res/mipmap-*/ic_launcher.png` | `mdpi`: 48 px, `hdpi`: 72 px, `xhdpi`: 96 px, `xxhdpi`: 144 px, `xxxhdpi`: 192 px. |
| Web | `web/icons/` | Replace `Icon-192.png`, `Icon-512.png`, `Icon-maskable-192.png` and `Icon-maskable-512.png` at their named sizes; keep the motif inside the maskable safe area. |
| Web favicon | `web/favicon.png` | 32 × 32 px; keep the reference in `web/index.html` consistent. |

Update every applicable iOS `Contents.json` filename, including the currently
empty iPad slots. Reuse identical-size assets where suitable. Ensure Xcode's
`ASSETCATALOG_COMPILER_APPICON_NAME` still selects `AppIcon`, Android's manifest
still selects the generated launcher resource, and web manifest paths match.
If adaptive/round Android icons or additional platforms are enabled, export and
wire their required assets too; do not leave another launcher variant showing
the template artwork.

Inspect the master and small exports, verify every referenced file's dimensions
and the iOS PNGs' lack of an alpha channel, then build and inspect the installed
launcher icon. If image generation is unavailable, use supplied study artwork
when available and report any remaining placeholder explicitly.
