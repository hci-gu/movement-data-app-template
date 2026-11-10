# Testing age and guardian flows with real BankID

GU confirmed that KontoAPI uses production BankID even on its test host. Test
personas authenticate an explicitly assigned real tester, then use a fictional
identity for the study workflow in an isolated simulation deployment.

`main` still uses direct BankID signing and requires guardians below **18**.
The template has no 10–30 eligibility restriction. This feature preserves those
rules. The `test-scolios-2` application uses KontoAPI followed by explicit consent,
ages 10–30 and guardian approval below **15**. Preserve those rules when porting.

## Configure and start a run

From `api/`:

```sh
go build -tags study_simulation -o app-simulation .
./app-simulation simulation identity --birth-date 2012-11-10 --serial 001
```

The generator produces a date-valid, checksum-valid fixture, not a reserved
national identity. It cannot guarantee that nobody has that number. Use fixtures
only in the simulation database. Choose a distinct three-digit serial for each
persona; its final digit determines registered gender parity for questionnaires.

Copy `deploy/simulation-personas.example.json` into an ignored private file such
as `api/.local/simulation-personas.local.json`. Replace `actorPersonalNumber` with
each tester's real 12-digit identity and `personalNumber` with the fictional
identity. The example actor is synthetic and must be replaced for live BankID.
Never commit the private configuration.

Each real tester may have one participant persona per run. Several testers may
test concurrently. Guardian personas are selected by the exact fictional number
entered when creating their invitations. A participant and both guardians may
be assigned to the same real tester. Unassigned actors are rejected; merely
opening a guardian link never authorizes someone to play that guardian.

```sh
./app-simulation simulation new-run \
  --file .local/simulation-personas.local.json \
  --out .local/age-14-run-1 \
  --consent ../deploy/test-consent.txt
./app-simulation simulation show --file .local/age-14-run-1/personas.json
```

This creates a fresh migrated database, publishes the supplied technical consent,
generates a fresh evidence keyring and freezes assignments under a random run ID.
It refuses an existing output directory. `show` prints only fictional identities,
useful for entering guardian numbers. It never prints real tester identities.

Configure the existing BankID provider using its documented environment variables,
then start the test server with absolute paths substituted below:

```sh
STUDY_DEPLOYMENT=simulation \
STUDY_SIMULATION_FILE=/absolute/path/to/age-14-run-1/personas.json \
STUDY_SECRETS_FILE=/absolute/path/to/age-14-run-1/study-keys.json \
./app-simulation serve --http=0.0.0.0:8091 \
  --dir=/absolute/path/to/age-14-run-1/pb_data
```

Ensure `STUDY_ENCRYPTION_KEYS` and `STUDY_ACTIVE_ENCRYPTION_KEY` do not override the
new keyring. Provider environment and study deployment are independent: real
BankID still needs the existing production provider credentials/configuration.
The feature does not configure credentials, change provider URLs or deploy a
server. Guardians need the usual public HTTPS origin and return-link setup.
Point Flutter's `API_BASE_URL` at this backend using the normal setup instructions.

To change scenarios, stop the old server, edit the private source profile, create
a **new** output directory and serve its database at the same test URL. Previous
tokens, guardian links and attempts cannot be reused. New participant IDs isolate
stored upload completion. Reopening an unchanged run preserves progress. Editing
its frozen profile causes startup to fail; changes are never hot-loaded into
pending attempts. Retain each original keyring with any retained evidence.

## Isolation and evidence

Normal binaries cannot resolve test identities and reject simulation configuration.
They also refuse simulation databases. Simulation startup requires explicit
enablement and rejects databases containing existing participants, evidence,
guardian requests, answers or uploads. Configuration changes require a new run.
Keep simulation hosting and storage separate from participant deployments. No
public API or Flutter preference enables or selects an identity override.

The provider result, signature and OCSP evidence remain unchanged. A separate
encrypted mapping records run, persona, role and effective number. Consent and
guardian validation check the mapping against the authenticated real actor.
User/signature/upload records and evidence exports are marked as simulated.
API responses include `X-Study-Simulation`; Health downloads also use a
`SIMULATION-` filename. Health readings themselves may still be the tester's real
data: this feature changes identity routing, not HealthKit samples.

The app identifies simulation during consent/signing and while signed in.
Guardian pages and the direct BankID signing text also identify the test.
Every role still requires real authentication/confirmation. In the KontoAPI port,
preserve explicit participant consent and guardian approval after authentication.

## Verification and porting to KontoAPI

```sh
go test ./...
go test -tags study_simulation ./...
go test -race -tags study_simulation ./internal/study ./internal/testidentity
go build -o app .
go build -tags study_simulation -o app-simulation .
```

Tests cover evidence preservation, one/two guardians using one actor, wrong actors,
underage/duplicate guardians, birthday routing, failed/risky/expired authentication,
changed consent, frozen configuration, and old sessions/attempts against a new
database. Automated tests use a fake provider, not live BankID.

For the study-app port:

1. Reuse `api/internal/testidentity`. Resolve after successful KontoAPI validation,
   before eligibility checks and user lookup. Keep `kontoapi.Result` unchanged in
   `AuthenticationEvidence`; record the mapping separately.
2. Carry the effective identity through participant consent and guardian approval,
   including subsequent evidence checks. Adapt database guards/markers to
   `authentications`, `consent_acceptances`, `guardian_requests`, and
   `guardian_approvals`. Do not port direct-signature requirements into KontoAPI.
3. Exercise study ages 9, 10, 14, 15, 17, 18, 25, 30, 31 and relevant birthdays
   using its existing age rules. Verify one/two guardian gating, wrong identity,
   withdrawal, changed consent, retries, relaunch and blocked uploads/deep links.
4. Complete real participant and guardian journeys on physical phones. Repeat
   identity mismatch with distinct testers. One actor playing multiple roles
   does not test independent people or prove legal guardianship.

Generate the normal template report on `main`; use `--study` after porting to the
study app with its actual consent and questionnaire content.

## Implementation verification — 2026-11-10

- Normal and simulation backend suites passed, including the race detector for
  study and identity packages. Both backend binaries built successfully.
- Flutter analysis and all 168 tests passed. Android release and iOS simulator
  builds passed. The template report has 85 views/85 screenshots, visually
  reviewed, including simulation consent, signing, checklist and guardian pages.
- The CLI created a fresh local database and keyring, applied migrations and
  published technical consent. With the provider disabled, local HTTP checks
  confirmed simulation response markers. Normal binaries rejected both the
  simulation database and simulation configuration. Existing-run overwrite was
  rejected.
- Live BankID, distinct real testers, physical iPhone handoff and HealthKit remain
  manual checks. No production deployment or KontoAPI study-app port was made.
