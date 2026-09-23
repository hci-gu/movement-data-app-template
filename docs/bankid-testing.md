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

The app shows consent before starting BankID. A successful signature creates or finds the user and grants the session in one BankID request. No invitations or user records need to be issued by an administrator. The backend checks the participant's age from the BankID personal number. Adults can upload immediately. A minor chooses one or two guardians and creates a signing link for each; all selected guardians must sign before the app or upload endpoint allows data transfer. Each guardian link starts a BankID order in a browser. The backend checks that the signer matches the requested personal number and is at least 18. This flow verifies identity and age, but it does not independently verify a legal guardianship relationship. Uploads use the authenticated `userId`.

Guardian links need `BANKID_PUBLIC_URL` set to the API's public HTTPS origin when `BANKID_RETURN_URL` uses the app's custom scheme. When the return URL is HTTPS on the API host, the backend derives the origin automatically. Keep this URL stable while links are active. Opening a guardian link creates a BankID order. The guardian can open BankID on the computer, open Mobile BankID on the same phone, or scan the changing QR code with Mobile BankID on another phone. The page checks signing status and offers **Restart signing** when the QR code expires. If a shared link opens inside a messaging app's browser and BankID does not launch, open the link in Safari or Chrome and tap the button there.

For local testing through ngrok, set `BANKID_PUBLIC_URL` to the current ngrok HTTPS origin and start ngrok against the API port. Set `TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128` only when the ngrok agent connects to the API over loopback, so the backend reads the guardian's IP from the proxy's `X-Forwarded-For` header. A new ngrok hostname requires updating `BANKID_PUBLIC_URL` and restarting the API. The BankID app on the signing phone must be configured for the same BankID environment as the API.

## API

| Action | Endpoint |
| --- | --- |
| Current consent | `GET /api/consent/current` |
| Sign consent and enter app | `POST /api/bankid/attempts` with `clientSecret`, `mode`, `consentTextId`, and `documentHash` |
| Attempt status | `GET /api/bankid/attempts/{id}` |
| Signed-in user | `GET /api/me` |
| Guardian status | `GET /api/guardians` (participant session) |
| Create guardian link | `POST /api/guardians` with `personalNumber` and `guardianCount` (1 or 2) |
| Guardian handoff | `GET /guardian/{id}` where `id` is the signing request's numeric `XXX-XXX` ID (starts an order and shows computer, phone, and QR options) |
| Consent receipt | `GET /api/consent/receipt` |
| Withdraw | `POST /api/consent/withdraw` |
| Upload | `POST /data` with `userId`, `chunkIndex`, and `data` |
| Researcher export | `GET /data/{userId}` with `X-API-Key` |

Back up the database, raw upload directory, and keyring before upgrading an older installation. The startup migration converts encrypted evidence and linked users, moves upload directories to user IDs, then removes the old namespace and invitation fields. It stops if required keys or evidence are missing. Unused invitation users are removed. Existing sessions are invalidated; users sign in with BankID again.

Run `go test ./...` in `api/`, plus `flutter analyze` and `flutter test test/bankid` from the repository root. Verify BankID handoff and HealthKit permissions on a physical iPhone before recruiting participants.
