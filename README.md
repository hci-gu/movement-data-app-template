# Research Steps Template

Flutter iOS app and PocketBase API for study enrollment, BankID-signed consent, Apple Health step-data preview, and authenticated resumable uploads.

- `lib/`: invitation enrollment, BankID signing/login, consent receipts, HealthKit and uploads.
- `api/`: Go/PocketBase backend, BankID RP client, durable collection worker, encrypted evidence and study operator commands.
- `health/`: local Health plugin.
- [BankID setup and testing](docs/bankid-testing.md): certificates, keys, device setup, deployment and test instructions.
- [BankID research and design](docs/bankid-signing-plan.md): source-backed implementation rationale.

Start with the setup guide. BankID requires backend RP certificates and a published consent document; it defaults to disabled until configured. There is no generated-password or unsigned-consent login fallback.

```bash
flutter pub get
flutter run --dart-define=API_BASE_URL=https://your-api.example.org \
  --dart-define=BANKID_RETURN_URL=researchsteps://bankid/return
```

```bash
cd api
go build -o app .
# Configure certificates and secrets, publish consent and issue invitations first.
./app serve --http=0.0.0.0:8080 --dir=./pb_data
```

Before use with participants, replace the study description and contacts in [app_config.dart](lib/app_config.dart), publish approved consent through the API command, set the intended bundle ID and HealthKit permission copy in the iOS project, and fill the [deployment template](api/deploy/api.yaml). Production uses an HTTPS universal return link. Back up and rehearse the PocketBase upgrade before migrating existing data.
