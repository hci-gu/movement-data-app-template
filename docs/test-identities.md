# Test identities with real BankID

The university's KontoAPI test host uses production BankID (confirmed by the
study owner). Authenticate with your real BankID. In a simulation deployment,
the backend then maps the verified personal number to a test personal number
before applying age, gender, consent and guardian rules. The original encrypted
KontoAPI response is preserved alongside the mapping and its revision.

## Change the identity in PocketBase

Sign in as a PocketBase superuser at the test API's `/_/` administration page.
Open **test_identity_mappings**, choose **New record**, and fill in:

| Field | Value |
| --- | --- |
| `realPersonalNumber` | Your real BankID identity, 12 digits `YYYYMMDDXXXX`. |
| `testPersonalNumber` | The fictional identity whose birth date you want to test, also 12 digits. |
| `role` | `participant` for app login; `guardian` for a guardian invitation. |

Save, sign out of the app, and authenticate again with your real BankID. No
restart, rebuild, redeploy or private JSON configuration is required. The
mapping is selected when authentication completes. An unassigned identity or
role is rejected; it never falls through to a real participant in simulation.
The app and guardian browser show **TESTSIMULERING**. The app displays the test
personal number, never the mapping's real personal number.

Only superusers can list, view, create, edit or delete these records. Do not
relax the collection rules. An actor can have one participant mapping and
multiple guardian mappings; test personal numbers must be unique and cannot
equal any actor's real number. The deployment supports up to 50 mappings.
Dates must be valid and not in the future. No real numbers are seeded in Git.

Every save creates a new server-owned revision. Previous sessions, consent and
guardian approval using that revision immediately stop authorizing requests.
The next participant login creates a fresh participant and empty progress, even
if you change back to an earlier number or save the same values. Old evidence
and data are retained as test history, not rewritten. Other testers' mappings
are unaffected. Deleting a mapping disables it. Editing a guardian mapping
requires that guardian to authenticate and explicitly approve again; editing
the participant mapping requires new consent and new guardian invitations.

## Age and guardian cases for this study

The scoliosis study includes ages **10–30**. Ages **10–14** require the selected
one or two adult guardians; ages **15–30** consent for themselves. Guardians
must be at least **18**. The dates below assume 2026-11-10; ages advance with
the server's real calendar, so adjust dates for birthday boundary tests.

| Scenario | Test birth date |
| --- | --- |
| Below minimum age | 2017-11-10 |
| Guardian required | 2012-11-10 |
| Just old enough to consent independently | 2011-11-10 |
| Age 18 | 2008-11-10 |
| Age 30 | 1996-11-10 |
| Above maximum age | 1995-11-10 |

Generate a checksum-correct fixture with the simulation binary:

```sh
cd api
go build -tags study_simulation -o app-simulation .
STUDY_SKIP_DOTENV=1 ./app-simulation simulation identity --birth-date 2012-11-10
```

These are test fixtures, not nationally reserved identities. Only enter them in
the simulation collection and app forms; BankID always authenticates the real
person. The test number is never sent to KontoAPI as an identity requirement.

To test guardians alone, create three mapping rows using your same real number:
one `participant` with a child date and two `guardian` rows with distinct adult
test numbers. In the app's guardian form, enter those **test guardian numbers**.
Open each invitation and use your real BankID, then explicitly confirm consent.
Authentication alone creates neither participant consent nor guardian approval.
Also test a different, unassigned tester, partial approval, withdrawal, changed
consent and both sides of the 10/15/18/31 birthdays.

## Local setup

```sh
bash scripts/run-scoliosis-local.sh --simulation --prepare-only
bash scripts/run-scoliosis-local.sh --simulation
flutter run --dart-define-from-file=deploy/flutter-local.json
```

Preparation needs no provider credentials. Running uses the existing documented
KontoAPI credential interface and GU VPN. Data and a new evidence keyring live in
`api/.local/scoliosis-simulation/`; the ordinary local database is untouched.
Create the first PocketBase superuser using the private setup link from startup,
then add mappings in `http://127.0.0.1:8091/_/`. For phone/guardian testing, use
the public URL and ingress options in [KontoAPI testing](bankid-testing.md).
Retain the keyring with any retained test evidence.

## Prepare and deploy the scoliosis test environment

Use the **deploy-simulation** Kustomize overlay. It keeps the existing public
URL and single replica, but uses `/pb/pb_data/simulation-v1` on the existing PVC
and a separate `movement-study-keys-simulation-v1` Secret. It preserves the old
database, publishes the actual study consent, and seeds the study questionnaire.
It starts with an empty mapping collection, ready for configuration in PocketBase.

```sh
# Build the explicit simulation image and render the deployment:
bash scripts/prepare-scoliosis-simulation.sh
```

The helper creates a fresh private keyring only if absent, builds
`registry.k8s.gu.se/appademin/movement-api:0.0.5-scolios-simulation` for
`linux/amd64`, and writes `build/scoliosis-simulation-manifest.yaml`. It does not
push an image or change the cluster. Use the cluster login/network/storage
prerequisites in [test deployment](test-deployment.md), then deploy:

```sh
oc whoami -t | docker login registry.k8s.gu.se --username "$(oc whoami)" --password-stdin
docker push registry.k8s.gu.se/appademin/movement-api:0.0.5-scolios-simulation
oc -n appademin create secret generic movement-study-keys-simulation-v1 \
  --from-file=study-keys.json=deploy/secrets/study-keys-simulation-v1.json \
  --dry-run=client -o yaml | oc -n appademin apply -f -
oc -n appademin apply --dry-run=server -f build/scoliosis-simulation-manifest.yaml
oc -n appademin apply -f build/scoliosis-simulation-manifest.yaml
oc -n appademin rollout status deployment/movement-api --timeout=180s
curl --fail --include https://movement-api.test.appadem.in/api/health
```

Expect `X-Study-Simulation: scoliosis-test-v1`. Existing KontoAPI credentials are
referenced by Secret name; they are not embedded in the image. Create a new
superuser for the fresh database through its private initial setup link, then
configure mappings at `https://movement-api.test.appadem.in/_/`. Run the iPhone
with `--dart-define-from-file=deploy/flutter-test.json` and sign out of any old
session. The existing combined study consent remains for testing only, subject
to the [study's release conditions](scoliosis-study.md#conditions-before-participant-use).

Subsequent deployments retain the simulation directory, deployment ID and keyring.
To return to the ordinary test app, deploy the base `deploy/` manifest; it still
points at the original database and original keyring. Do not serve simulation
data as ordinary study data. A normal binary cannot enable mapping even through
environment variables; startup also rejects a database marked as a simulation.
The simulation image requires both `STUDY_DEPLOYMENT=simulation` and
`STUDY_SIMULATION_ID`, and first startup refuses an existing participant database.

## Verification

Run `go test ./...` and `go test -race -tags study_simulation ./...` from `api/`.
Coverage includes untouched provider evidence, explicit consent, one/two guardian
approvals by one actor, wrong/unassigned identities, mapping revision revocation,
fresh progress after edits, birthday boundaries, failed/expired/cancelled orders,
private collection rules and deployment isolation. Run the full Flutter suite
and regenerate the study report for the app and browser simulation indicators.
Complete the [physical-device checklist](manual-review.md) with real BankID and
HealthKit before using the test deployment for an end-to-end review.
