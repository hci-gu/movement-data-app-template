# Research Steps Template

Reusable Flutter + PocketBase template for research studies that need to:

- enroll a participant with a study-specific identifier
- request read access to Apple Health step data
- preview the extracted dataset on-device
- upload the data to an API in resumable compressed chunks

## What This Repository Contains

- `lib/`: Flutter client for consent, HealthKit authorization, dataset preview, and upload
- `api/`: PocketBase-backed API for participant creation and chunked upload storage
- `health/`: local Health plugin package used by the Flutter app

## Template Defaults

- app name: `Research Steps Template`
- identifier model: generic `participantId`
- requested health data: Apple Health `STEPS`
- API base URL: configured in [`lib/app_config.dart`](/Users/sebastianandreasson/Documents/code/work/gu/swede_heart/lib/app_config.dart) or via `--dart-define=API_BASE_URL=...`

## Before Using With Real Participants

1. Replace the placeholder study description, consent copy, and contact details in [`lib/app_config.dart`](/Users/sebastianandreasson/Documents/code/work/gu/swede_heart/lib/app_config.dart).
2. Update iOS display names and HealthKit permission strings in [`ios/Runner/Info.plist`](/Users/sebastianandreasson/Documents/code/work/gu/swede_heart/ios/Runner/Info.plist).
3. Replace deployment image names and hostnames in [`api/deploy/api.yaml`](/Users/sebastianandreasson/Documents/code/work/gu/swede_heart/api/deploy/api.yaml).
4. Review PocketBase migrations and collections for your own study governance requirements.
5. Configure the real backend base URL before building the app.

## Local Development

Flutter app:

```bash
flutter pub get
flutter run --dart-define=API_BASE_URL=https://your-api.example.org
```

Backend:

```bash
cd api
go run .
```
