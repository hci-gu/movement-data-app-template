# Skolios och rörelse

Flutter iOS research app and Go/PocketBase backend for the scoliosis study in
`context/`. Read [study sources, local setup and release conditions](docs/scoliosis-study.md)
first. The app presents the Swedish study information and questionnaire, previews
six Apple Health movement measures, and uploads them for the authenticated participant.

Participants identify through GU KontoAPI with BankID, then explicitly accept
consent in the app. BankID does not sign the consent text. Participants under
15 need approval from all selected guardians, who identify and accept separately
on a browser page. The backend enforces consent, guardian approval, and the
study's age and movement-window rules. The questionnaire's treatment type and
date are needed before Health data can be read.

For local testing, configure the credentials described in [KontoAPI testing](docs/bankid-testing.md),
connect to the GU VPN, and run:

```sh
bash scripts/run-scoliosis-local.sh --prepare-only
bash scripts/run-scoliosis-local.sh
flutter pub get
flutter run --dart-define-from-file=deploy/flutter-local.json
```

The isolated test database listens on port 8091 and contains the study's combined
consent form for local testing. This form is not a substitute for the separate
forms required by the ethics decision. Review the release conditions before
participant use. The old direct-BankID database cannot be migrated automatically.

- [Authentication, consent and evidence architecture](docs/bankid-signing-plan.md)
- [Study workflow](docs/workflow.md)
- [Questionnaire definitions](docs/questionnaire-data.md)
- [Customization and module ownership](docs/customization.md)
- [Manual review](docs/manual-review.md)
- [Demo gallery and app report](docs/app-report.md)
- [Study app icons](docs/app-icons.md)

The study migration seeds thirteen questions and two option sets from the
supplied DOCX. Synthetic participants and measurements exist only in tests and
the separate demo entry point. Physical iPhone testing is required for BankID
handoff and HealthKit; Android builds are verification artifacts.
