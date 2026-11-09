# Research Steps Template

Flutter iOS app and Go/PocketBase backend for BankID identity verification through
GU KontoAPI, explicit study consent, Apple Health preview and authenticated
chunked uploads.

One backend serves one study. Participants identify first, then review and accept
the current consent in the app. Authentication alone does not grant study access.
Participants under 18 need one or two guardians, who identify and explicitly
approve through browser invitations. Consent is recorded by the application;
BankID does not sign the consent text.

- [Setup and VPN testing](docs/bankid-testing.md)
- [Remote OpenShift test deployment](docs/test-deployment.md)
- [Authentication, consent and evidence architecture](docs/bankid-signing-plan.md)
- [Study workflow](docs/workflow.md)
- [Questionnaire definitions](docs/questionnaire-data.md)
- [Customization and module ownership](docs/customization.md)
- [Manual review](docs/manual-review.md)
- [Demo gallery and app report](docs/app-report.md)
- [Study app icons](docs/app-icons.md)

The journey is Introduction → BankID identity → Consent → Home checklist → Done.
Questionnaires and step upload can be completed in either order. The baseline
has no study-specific questionnaire seeds; synthetic examples live in tests and
the separate demo entry point.

With ignored test credentials in the repository root, connect to GU VPN and run:

```sh
python3 scripts/run-kontoapi-test.py
```

In another terminal, use the backend address reachable from the device:

```sh
flutter pub get
flutter run --dart-define=API_BASE_URL=http://YOUR_LAN_IP:8090
```

The runner initializes an isolated disposable database and technical test consent.
See the setup guide for credentials, guardian HTTPS links and deployment.

Explore real screens with offline sample services:

```sh
flutter run -t lib/main_demo.dart
python3 scripts/build-app-report.py
```

This schema replacement requires a fresh database. Old signature data is not
migrated. Use one backend replica; restarting interrupts pending authentication
attempts. Persisted identities and consent survive with their original evidence
keyring.

Before participant use, apply the supplied study requirements in `context/`,
publish approved consent, adapt age/guardian rules as needed, replace template
copy/contacts/branding and verify physical BankID and HealthKit flows. See
`lib/study/`, `lib/app/environment.dart`, `api/internal/kontoapi/` and
`api/internal/study/`. Every study adaptation ends with a reviewed app report.
