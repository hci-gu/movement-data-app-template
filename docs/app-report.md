# Demo gallery and app delivery report

Every adapted study app ends with a freshly generated, visually reviewed
`app_report.html` at the repository root. It is a standalone file with embedded
screenshots, search, full-size image viewing, coverage counts, content provenance
and build/check results. It works offline and can be shared without its build
directory. Generated reports are ignored by Git; link the actual file in the
agent's final response.

Each preview has a descriptive heading separated from its screenshot by a margin.
Only screenshots have a rounded frame; routes and case IDs are not displayed
around them. Scroll continuations use the same framing. Routes and IDs remain
in the build manifest for coverage/debugging. Follow the
[design guidelines](design-guidelines.md#visual-acceptance) when reviewing layout.

## Run the template

Prerequisites: the repository's Flutter and Go toolchains, Python 3.10+, Node.js
with `npx`, and Google Chrome for browser captures. The report script runs the
pinned `@playwright/cli@0.1.22` through npx; the first run may download that package.
Native iOS builds require macOS and Xcode. Android builds require the normal
Android toolchain. No running backend, BankID credentials, Health permissions or
participant records are needed.

```sh
flutter pub get
flutter run -t lib/main_demo.dart
python3 scripts/build-app-report.py
```

On Windows, use `python` instead of `python3` if necessary. The gallery works on
the app's supported native targets. Select a case to open it directly; use the
separate Gallery toolbar to return and choose another. Each selection gets fresh
in-memory data. The toolbar is excluded from report captures.

The checked-in demo snapshot lets a fresh clone launch immediately. With no
study questionnaire or consent seeded, the exporter uses explicitly labelled
synthetic examples. They are loaded into a disposable database only, never on
normal backend startup. A small additional text-input example is demo-only.

## Finish a study adaptation

1. Finish source changes and questionnaire migrations. Publish the study consent
   to the intended backend following [the workflow](workflow.md). The report's
   disposable copy does not replace that publication.
2. Refresh preview content from the migrations and agreed consent source:

   ```sh
   python3 scripts/build-app-report.py --prepare-only --study \
     --consent deploy/consent/study-consent.txt --consent-title 'Study consent'
   flutter run -t lib/main_demo.dart
   ```

   Omit `--consent` only if a migration already publishes the intended consent.
   `--study` rejects the template fallback and explicit draft/test markers in
   exported consent and definitions. A study with no questionnaire is
   supported when study consent is supplied. Preserve the source file outside
   the generated output; report text is a preview copy, not proof of approval.
3. Update demo cases and sample services for added screens, overlays, data types,
   consent audiences and important states. Use the actual app widgets and source
   wording. Keep participant identities, answers and measurements synthetic.
4. Generate the final report after the last change:

   ```sh
   python3 scripts/build-app-report.py --study \
     --consent deploy/consent/study-consent.txt --consent-title 'Study consent'
   ```

5. Open `app_report.html`. Inspect every screen, all long-content continuations,
   images of conditional questions and the browser pages. Check clipping, text,
   blank states, correct data and completed animations. Check artwork alignment,
   visible navigation bars, centered BankID states, guardian-button spacing and
   factual final copy. Inspect small screens and enlarged text as described in
   [manual review](manual-review.md). Fix problems and rerun.
   Verify search, image enlargement and offline loading. Link the report in the
   final response and state any unverified device-dependent behavior.

The command exports content, runs analysis and the complete Flutter/Go suites,
captures screenshots, builds the backend and Android release app, and builds the
iOS simulator app on macOS. Missing captures, unreachable questions, rendering
exceptions, unexpected HTTP calls and failed checks stop generation. The final
HTML is replaced atomically only after success. A failed run leaves any previous
report unchanged and explicitly reports that it is stale.

A full `--study` run also checks the capture manifest's `studyContent` and
`participantText`: known default study names/introduction, example contact emails,
and explicit unfinished markers (TODO, TBD, FIXME, lorem ipsum, demonstration/test
consent, `[insert …]`, `[draft]`, and standalone Draft/Utkast headings) fail delivery.
Study metadata includes the introduction, information sections and contacts;
rendered Flutter text is collected through every scroll continuation. Input hints
and synthetic typed answers are excluded. Loading views may have no text.
`--prepare-only` checks exported source content only; app copy is checked during
the full capture run. Ordinary template reports remain explicitly synthetic.

This is a narrow check for identifiable leftovers, not a language/approval or
deployment validator. Review guardian HTML copy, assets, native metadata, every
participant-facing string and actual enrollment availability. Do not remove real
failure states or bypass safeguards to pass a report. Record unresolved inputs
outside participant UI following [delivery readiness](workflow.md#delivery-readiness).

For quicker iteration, `--skip-builds` omits the two native app builds and marks
them **not run** in the report. This is a development report, not final build
verification. On non-macOS hosts the iOS build is also explicitly marked not run.
The script uses Flutter SDK's Roboto font for portable widget captures. For SDK
wrappers whose installation cannot be located, pass `--font /path/to/font.ttf`;
the selected filename is recorded in the report.

## Where the scaffolding lives

| File | Responsibility |
| --- | --- |
| `lib/main_demo.dart` | Separate native demo entry point; no native session/bootstrap/link startup. |
| `lib/demo/gallery.dart` | Gallery and real app viewport; opens overlays through the actual controls. |
| `lib/demo/cases.dart` | Explicit screen/state cases shared by gallery and captures. |
| `lib/demo/data.dart` | Parse exported definitions and derive valid ancestor answers/audience for each question. |
| `lib/demo/services.dart` | In-memory service implementations and initial controller states. |
| `lib/demo/study_data.g.dart` | Generated content snapshot, imported only by demo code. Refresh with the script; do not hand-edit. |
| `api/cmd/demo-export/` | Apply migrations in a temporary database and export a whitelist of definition/consent fields. Never loads `.env` or accepts an existing database directory. |
| `api/internal/study/demo_pages.go` | Export the real guardian HTML template with inert local sample endpoints. No preview HTTP routes are added to the backend. |
| `test/report/` | Render cases, assert coverage/isolation and capture scrolling content. |
| `scripts/build-app-report.py` | Run checks, capture browser pages, validate artifacts and assemble the single HTML file. |
| `scripts/app-report-template.html` | Report presentation, search and image viewer. |

`build/app-report/` contains command logs, the exported source snapshot, raw PNGs
and a combined manifest. These intermediate files are disposable. The generated
Dart snapshot is tracked so the demo works on a fresh checkout; regenerate it
and include its update when delivering a study adaptation.

## Adding coverage

Add a `PreviewCase` with a stable ID, title, group, real route, sample state and
optional action keys. Configure its fake service responses in `services.dart`.
Overlay actions invoke keyed production buttons; add a stable widget key when
needed. Do not duplicate screen markup in the demo. Add a readiness check in the
capture test for a new kind of overlay.

Question cases are generated from every enabled exported definition, including
conditional branches, restricted audiences and saved answers. Their parent
answers must satisfy the production visibility rules. Conflicting audiences or
unreachable questions fail loudly. A `?question=<id>` link selects an initial
visible question after loading; normal session and guardian guards still apply.
It cannot expose a hidden question.

Route coverage is checked against the real router. Redirect-only legacy aliases
are not screens. Sheets/dialogs and important error/loading states require
explicit cases because route enumeration cannot discover them. When adding or
removing any user-visible surface, update the cases in the same change. This
small preview list is development tooling, not a production workflow registry.

Captures use bounded frame advancement rather than waiting indefinitely on
loading indicators. Transient states such as accepted signing and active uploads
are held for inspection. Long pages are captured in overlapping viewport sections;
date wheels and questionnaire page navigation are excluded from that scrolling.
Every required case needs at least one valid PNG. The builder clears only its own
capture directory before running, so stale images cannot fill missing coverage.

The capture suite also checks the introduction, consent, BankID states, checklist,
guardian setup and Done at 320 × 568 with 2× text. It writes their overlapping
review images to `build/app-report/captures/layout-review/`. Inspect these alongside
the report; they are additional layout checks and are not included in its view
counts. Normal BankID captures assert that the full status/action group is centered.

The gallery uses real controllers with seeded state and fake gateways. Production
routing and authorization are unchanged. HTTP is blocked as a backstop, storage is
in memory, and BankID/settings launches are simulated. There is no participant
demo toggle, age selector, backend authorization bypass or live data upload.

Flutter screenshots use the real widgets with portable font rendering; they are
not screenshots of a signed-in physical iPhone. Guardian pages are captured by
Chrome from the real Go HTML template with sample responses. External BankID UI,
system permission dialogs and real HealthKit/signing behavior remain in the
[physical-device checklist](manual-review.md).
