# Setup and testing

Generate the evidence keyring from the repository root:

```bash
python3 scripts/create-study-secrets.py --out api/secrets/study-keys.json
```

Set BankID certificate, return URL, and keyring paths using [api/.env.example](../api/.env.example). The backend does not load `.env` automatically. Use one backend replica because pending BankID attempts live in process memory.

Publish approved consent in the PocketBase `consent_texts` collection or with:

```bash
cd api
./app bankid publish-consent --file consent.txt --version 2026-01 --title 'Study consent' --dir=./pb_data
```

The app shows consent before starting BankID. A successful signature creates or finds the user and grants the session in one BankID request. No invitations or user records need to be issued by an administrator. Uploads use the authenticated `userId`.

## API

| Action | Endpoint |
| --- | --- |
| Current consent | `GET /api/consent/current` |
| Sign consent and enter app | `POST /api/bankid/attempts` with `clientSecret`, `mode`, `consentTextId`, and `documentHash` |
| Attempt status | `GET /api/bankid/attempts/{id}` |
| Signed-in user | `GET /api/me` |
| Consent receipt | `GET /api/consent/receipt` |
| Withdraw | `POST /api/consent/withdraw` |
| Upload | `POST /data` with `userId`, `chunkIndex`, and `data` |
| Researcher export | `GET /data/{userId}` with `X-API-Key` |

Back up the database, raw upload directory, and keyring before upgrading an older installation. The startup migration converts encrypted evidence and linked users, moves upload directories to user IDs, then removes the old namespace and invitation fields. It stops if required keys or evidence are missing. Unused invitation users are removed. Existing sessions are invalidated; users sign in with BankID again.

Run `go test ./...` in `api/`, plus `flutter analyze` and `flutter test test/bankid` from the repository root. Verify BankID handoff and HealthKit permissions on a physical iPhone before recruiting participants.
