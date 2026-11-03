# Skolios och rörelse

Flutter iOS research app and PocketBase backend for the scoliosis study supplied in `context/`. Read [study sources, local setup and release conditions](docs/scoliosis-study.md) first. The app provides Swedish BankID consent, a treatment questionnaire, preview of six Apple Health movement measures and authenticated uploads.

The backend serves one study. The application has nine collections: `users`, `answers`, `dataUploads`, `signatures`, `signingRequests`, `consent_texts`, `questionnaires`, `questions`, and `questionOptions`. The app shows consent first; one successful BankID signature creates or finds a user by `personalNumber` and grants a session. Participants under 15 need all guardians to sign before submitting answers or uploading. Each terminal BankID attempt is saved in `signatures`; its `user` field relates to the participant. Consent state is derived from accepted signature records.

- `lib/`: consent review, BankID attempts/login, receipts, questionnaires, HealthKit, and uploads.
- `api/`: Go/PocketBase backend, BankID client, study administration, and uploads.
- `health/`: local Health plugin.
- [Setup and testing](docs/bankid-testing.md)
- [Architecture](docs/bankid-signing-plan.md)
- [Questionnaire definitions and responses](docs/questionnaire-data.md)
- [Adapting the source and module ownership](docs/customization.md)
- [Manual review checklist](docs/manual-review.md)
- [Demo gallery and final app report](docs/app-report.md)
- [Study app icon generation and Xcode installation](docs/app-icons.md)

The study migration seeds thirteen questions and two option sets from the supplied DOCX. Synthetic participants and measurements exist only in tests and the separate demo entry point.

The journey is Introduction → BankID/consent → Home checklist → Done. Submit treatment type and date before reading Health data; they determine the permitted collection period. Questions appear one per vertical page; saved responses can be reviewed from home.

```bash
flutter pub get
flutter run --dart-define=API_BASE_URL=https://your-api.example.org \
  --dart-define=BANKID_RETURN_URL=researchsteps://bankid/return
```

Explore screens without a backend or real participant data, then generate the
standalone screenshot report:

```bash
flutter run -t lib/main_demo.dart
python3 scripts/build-app-report.py
```

Every study adaptation finishes with a reviewed `app_report.html`. See the
[report guide](docs/app-report.md) for exporting actual study content, adding
screen coverage and the browser/native build prerequisites.

```bash
cd api
go build -o app .
# Load certificates and study keys as described in the setup guide.
./app serve --http=0.0.0.0:8080 --dir=./pb_data
```

Existing databases migrate on startup with the same encryption keys used to save identity and signing evidence. Back up the database, raw upload directory, and keyring before upgrading. The migration moves upload directories from old participant IDs to user IDs and removes unused invitation records. Use the matching Flutter app; older invitation-based clients are unsupported.

Before use with participants, replace study information and contacts in `lib/study/`, publish approved consent, configure the final bundle identifier and HealthKit permission copy, and fill the deployment template. Connection overrides live in `lib/app/environment.dart`. Run one backend replica. A restart interrupts pending BankID requests; saved signatures and enrolled users survive.
