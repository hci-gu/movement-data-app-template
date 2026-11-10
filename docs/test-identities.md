# Editable test identities

In an explicitly enabled simulation deployment, configure real-to-test personal
number mappings in PocketBase's **test_identity_mappings** collection. Sign in
with a real BankID and the backend uses the assigned fictional identity for
age, gender, participant progress and guardian checks. The real, unmodified
provider result remains in encrypted evidence with a separate mapping revision.

`main` retains the template's direct BankID signing implementation and under-18
guardian rule. `test-scolios-2` ports this collection to KontoAPI authentication,
explicit in-app consent and guardian approval, with study eligibility 10–30 and
guardians under 15. That branch includes `deploy-simulation/` and
`scripts/prepare-scoliosis-simulation.sh`; see its `docs/test-identities.md` for
the ready-to-deploy scoliosis configuration. Do not replace one provider's
authentication/consent workflow with the other when porting changes.

## Set up a separate simulation backend

Build from `api/` using `go build -tags study_simulation -o app-simulation .`.
Configure the provider and evidence keyring through the existing documented
interface in [BankID testing](bankid-testing.md). Add both environment values:

```sh
STUDY_DEPLOYMENT=simulation
STUDY_SIMULATION_ID=template-test-v1
```

Use a new database directory and a separate keyring. Publish the intended test
consent to that directory with `bankid publish-consent`, then run
`app-simulation serve --dir=PATH_TO_NEW_DATABASE`. Configure the app and guardian
public URL to reach this backend. `STUDY_SIMULATION_ID` identifies the persistent
test database; retain it and the keyring across redeployments. No persona file
is needed. Provider environment/certificates are independent of simulation mode:
use the provider configuration that authenticates the real tester's BankID.

Normal builds cannot enable simulation. Collection API rules remain locked to
superusers. There is no database marker collection or corresponding startup check.

## Configure and switch test identities

Open the test server's `/_/` PocketBase dashboard as a superuser. Create a row:

| Field | Value |
| --- | --- |
| `realPersonalNumber` | Your real BankID personal number, `YYYYMMDDXXXX` or `YYYYMMDD-XXXX`. |
| `testPersonalNumber` | The fictional number, including the birth date to test, in the same format. |
| `role` | `participant` for app login; `guardian` for guardian invitations. |

Save, sign out and sign in again with real BankID. Mapping takes place only after
successful provider verification; rejected, expired or high-risk results cannot
grant a test session. Unassigned identities are rejected in simulation mode.
The original provider evidence is not changed. The app, guardian pages and BankID
text use the ordinary interface without simulation banners. The optional hyphen
and surrounding whitespace are removed on save; invalid fields show an explanation.
Leave the separate `id` field empty to generate it automatically.

One real identity can have one participant mapping and multiple guardian
mappings. All test numbers must differ from each other and from every real
actor's number. The collection accepts at most 50 mappings and validates dates.
To play a child and both guardians, add three rows with your real number, using
one child test number and two distinct adult guardian numbers. Enter the test
guardian numbers in the app's invitations, then use your real BankID for each.
Direct BankID requests constrain the guardian to the mapped real actor; no
fictional guardian number is sent as the provider identity requirement.

Every mapping save rotates a server-owned revision. Existing sessions and
signatures using that revision stop authorizing access. A participant's next
login starts with a new participant and empty progress, including when saving
unchanged values or changing back to an earlier number. Historical data/evidence
remain; other testers are unaffected. A guardian edit requires a new signature.
Delete a mapping to disable it. Do not expose the collection to participants or
put real identities into Git or shared reports.

Generate a checksum-correct fixture with:

```sh
STUDY_SKIP_DOTENV=1 ./app-simulation simulation identity --birth-date 2012-11-10
```

These are local fixtures, not nationally reserved personal numbers. Only use
them for simulated study data; BankID always verifies the real actor. Choose
dates on both sides of the relevant birthdays; the server calendar is unchanged.

## Upgrading the earlier file-based implementation

Migration `1794310000_collection_test_identities.go` adds the collection and
participant revision isolation without deleting retained data. Replace
`STUDY_SIMULATION_FILE` with `STUDY_SIMULATION_ID` using the existing run ID if
retaining that simulation database, then enter mappings in PocketBase. Old
file-based sessions/signatures have no collection revision and cannot authorize
new activity; signing in creates a fresh scenario. Keep the old evidence keyring
to read history. Alternatively start a new database and new deployment ID.

Migration `1794320000_remove_simulation_state.go` removes the former
`simulation_state` collection while retaining mappings and other study records.

## Verification

Run `go test ./...`, `go test -race -tags study_simulation ./...`, and both normal
and simulation builds from `api/`. Tests cover mapping validation and isolation,
original evidence preservation, role/guardian separation, real actor restrictions,
one/two guardian flows, failure paths and revocation after edits. Complete the
[manual device checks](manual-review.md) with real BankID; automated fixtures do
not verify provider handoff, signing on the phone or HealthKit permissions.
