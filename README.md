# Research Steps Template

Flutter iOS app and PocketBase backend for invitation enrollment, BankID-signed consent, Apple Health step-data preview, and authenticated chunked uploads.

The application has eight collections: `users`, `answers`, `dataUploads`, `signatures`, `consent_texts`, `questionnaires`, `questions`, and `questionOptions`. A user can begin as an invitation. BankID attempts live in one backend process's memory; terminal outcomes and encrypted evidence are stored in `signatures`.

- `lib/`: consent review, BankID attempts/login, receipts, HealthKit, and uploads.
- `api/`: Go/PocketBase backend, BankID client, study administration, and uploads.
- `api/cmd/cutover/`: one-time offline conversion into a new data directory.
- `health/`: local Health plugin.
- [Setup and testing](docs/bankid-testing.md)
- [Architecture](docs/bankid-signing-plan.md)
- [Implementation checklist](implementation-plan.md)

```bash
flutter pub get
flutter run --dart-define=API_BASE_URL=https://your-api.example.org \
  --dart-define=BANKID_RETURN_URL=researchsteps://bankid/return
```

```bash
cd api
go build -o app .
# Load certificates and study keys as described in the setup guide.
./app serve --http=0.0.0.0:8080 --dir=./pb_data
```

This release requires the matching Flutter app. Existing installations must stop the backend and run the offline cutover before starting this release. There are no compatibility endpoints or runtime readers for the previous storage layout.

Before use with participants, replace study information and contacts in `lib/app_config.dart`, publish approved consent, configure the final bundle identifier and HealthKit permission copy, and fill the deployment template. Run one backend replica. A restart interrupts pending BankID requests; saved signatures and enrolled users survive.
