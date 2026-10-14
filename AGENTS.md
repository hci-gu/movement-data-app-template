# Repository guide for agents

## Purpose and starting point

This repository is a reusable research-app template. Its current implementation is an iOS-focused Flutter app for reviewing study consent, signing with BankID, reading Apple Health step data, previewing it, and uploading it to a Go/PocketBase backend. A specific application is created by adding application context and adapting the template to those requirements.

**Read [docs/workflow.md](docs/workflow.md) first for setup and deployment work.** It is the primary workflow guide (VIP). It currently outlines PocketBase setup, Flutter dependencies/build, deployment, and BankID/file verification, with some steps still unfinished. Use the detailed resources below to fill in operational details; do not assume the unfinished steps are complete instructions.

Read the relevant creator-supplied application context in `context/` before implementing changes. Developer-maintained guides in `docs/` explain how to work with the template. Template copy, test consent, existing deployment values, and example configuration are starting points that must be reviewed for the intended application.

## Markdown resource index

| Resource | Use it for |
| --- | --- |
| [docs/workflow.md](docs/workflow.md) | Primary setup and deployment workflow; start here. |
| [README.md](README.md) | Project overview, quick-start commands, collections, and customization prerequisites. |
| [docs/bankid-testing.md](docs/bankid-testing.md) | BankID configuration, evidence keyring setup, consent publication, local and OpenShift test deployment, API endpoints, and verification commands. Environment-specific values describe the documented test setup. |
| [docs/bankid-signing-plan.md](docs/bankid-signing-plan.md) | Current BankID architecture, collection relationships, participant and guardian signing, consent state, and signing-attempt persistence. |
| [docs/questionnaire-data.md](docs/questionnaire-data.md) | Turning source questionnaire material into PocketBase definitions, option sets, answer settings, reproducible seed migrations, and verification. |
| [health/README.md](health/README.md) | Bundled Health plugin setup, supported data types, permissions, and usage. Android guidance here describes plugin capabilities; the app's current focus is iOS/Apple Health. |
| [health/CHANGELOG.md](health/CHANGELOG.md) | Bundled Health plugin version history. |
| [ios/Runner/Assets.xcassets/LaunchImage.imageset/README.md](ios/Runner/Assets.xcassets/LaunchImage.imageset/README.md) | Replacing iOS launch-screen assets. |

Keep this index updated when adding or moving template documentation. Application-specific source material belongs in `context/` and may include formats other than Markdown.

## Codebase map

| Location | Responsibility |
| --- | --- |
| `context/` | Creator-supplied application source material, such as ethical approvals and questionnaire specifications. |
| `docs/` | Technical guides maintained by the template developers, including setup, architecture, and questionnaire-data instructions. |
| `lib/main.dart`, `lib/router.dart` | Flutter startup, BankID return links, lifecycle handling, navigation, and session/guardian route gates. |
| `lib/app_config.dart` | App/study names, contacts, participant-facing copy, Health data selection, import period, and `API_BASE_URL` / `BANKID_RETURN_URL` build configuration. |
| `lib/screens/`, `lib/widgets/`, `lib/theme.dart` | Participant screens, step preview chart, and Cupertino styling. |
| `lib/bankid/` | BankID models, gateway, and signing controller. |
| `lib/state/auth.dart`, `lib/storage.dart` | Authentication state, session restoration, secure session/signing storage, and local upload preferences. |
| `lib/state/health.dart`, `lib/api.dart`, `lib/pocketbase.dart` | Health permissions/import/summary, authenticated API calls, compressed chunk uploads, and PocketBase client. |
| `api/main.go` | PocketBase entry point, migrations, study service, and route registration. |
| `api/internal/study/` | Consent, participant sessions, guardians, encrypted signing evidence, administration, and configuration. |
| `api/internal/bankid/` | BankID provider client. |
| `api/internal/schema/`, `api/migrations/` | Collection schema, schema/data migrations, and questionnaire seeds. |
| `api/upload_routes.go`, `api/upload_storage.go` | Upload authorization, metadata, raw chunk storage, and researcher export. |
| `health/` | Local Flutter Health plugin used through a path dependency in `pubspec.yaml`. |
| `ios/`, `android/`, `web/`, `assets/` | Platform projects and assets; validate platform support for the intended app. |
| `deploy/`, `api/Dockerfile` | Deployment manifest, Flutter test configuration, test consent, and backend image build. |
| `scripts/` | Study keyring generation and iOS BankID associated-domain configuration helpers. |
| `test/bankid/`, Go `*_test.go` files under `api/` | Flutter BankID tests and backend tests. |

The Flutter app uses Riverpod/hooks for state, GoRouter for navigation, Dio for API calls, and PocketBase for the client/auth store. Dependency and toolchain declarations live in `pubspec.yaml` / `pubspec.lock` and `api/go.mod` / `api/go.sum`.

## Current behavior to preserve when adapting the template

- One backend serves one study. Collections cover users, signatures, guardian signing requests, consent texts, answers, uploads, and questionnaire definitions; see the architecture guide for relationships.
- The app presents consent before BankID signing. Successful signing creates or finds the participant and grants a session. Consent and withdrawal state are derived from signature records.
- Participants under 18 need the selected one or two guardian signatures before uploading. The backend enforces consent as well as session authorization.
- Step data is read from Apple Health, previewed locally, and uploaded in compressed chunks linked to the authenticated user.
- Pending BankID attempts live in process memory, so the backend runs as one replica and restarts interrupt pending attempts. Persisted signatures and users survive.
- Existing encrypted evidence depends on the original study keyring. Follow the setup guide's backup and migration instructions when upgrading an existing installation.

## Adding application-specific context

Application creators place source material in `context/`, such as an ethical approval, questionnaire specification, or study requirements. These files do not need to be Markdown. Read the relevant files when adapting the template, preserve the supplied source material, and distinguish approved requirements from examples, open questions, and test material.

The template developers maintain `docs/` for reusable technical documentation. Add or update those guides when the template's setup or implementation changes.

Useful context includes the application's purpose and scope, participant groups, approved study/consent/privacy text, contacts, branding and app identifiers, required screens and flows, Health data types and collection periods, questionnaire sources, deployment environments, and acceptance criteria. Record decisions and unresolved requirements so future sessions can use them without reconstructing the conversation.

Apply that context to the relevant code and configuration, especially `lib/app_config.dart`, platform metadata/permissions, consent records, questionnaire migrations, and deployment files. Keep the workflow guide current when setup requirements change. Do not invent questionnaire choices, answer limits, or behavior absent from source material; follow [docs/questionnaire-data.md](docs/questionnaire-data.md).

## Working and verification

- On Windows, use PowerShell, `rg` for searches, and `Get-Content` for reads. Use `apply_patch` for manual file edits.
- Inspect nearby code, keep changes scoped, and preserve existing user changes. Do not inspect secrets or local `.env` files; consult the documented configuration interface and `api/.env.example` as needed.
- From the repository root, install Flutter dependencies with `flutter pub get`. For Flutter changes, run `flutter analyze` and the relevant tests; the documented BankID suite is `flutter test test/bankid`.
- From `api/`, build with `go build -o app .` and run backend checks with `go test ./...`.
- For Python helper changes, use `python -m py_compile <changed-files>` for a syntax check.
- Verify BankID handoff and HealthKit permissions on a physical iPhone before participant use. Report any checks that could not run and what remains unverified.
