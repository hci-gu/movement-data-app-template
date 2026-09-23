# Research Steps Template

Flutter iOS app and PocketBase backend for one-step BankID consent signing and account creation, Apple Health step-data preview, and authenticated chunked uploads.

The backend serves one study. The application has eight collections: `users`, `answers`, `dataUploads`, `signatures`, `consent_texts`, `questionnaires`, `questions`, and `questionOptions`. The app shows consent first; one successful BankID signature creates or finds a user by `personalNumber` and grants a session. Each terminal BankID attempt is saved in `signatures`; its `user` field is a relation to `users`. Consent state is derived from accepted signature records.

- `lib/`: consent review, BankID attempts/login, receipts, HealthKit, and uploads.
- `api/`: Go/PocketBase backend, BankID client, study administration, and uploads.
- `health/`: local Health plugin.
- [Setup and testing](docs/bankid-testing.md)
- [Architecture](docs/bankid-signing-plan.md)

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

Existing databases migrate on startup with the same encryption keys used to save identity and signing evidence. Back up the database, raw upload directory, and keyring before upgrading. The migration moves upload directories from old participant IDs to user IDs and removes unused invitation records. Use the matching Flutter app; older invitation-based clients are unsupported.

Before use with participants, replace study information and contacts in `lib/app_config.dart`, publish approved consent, configure the final bundle identifier and HealthKit permission copy, and fill the deployment template. Run one backend replica. A restart interrupts pending BankID requests; saved signatures and enrolled users survive.
