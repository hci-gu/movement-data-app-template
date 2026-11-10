# Deploy the KontoAPI backend to OpenShift test

For editable real-to-test BankID mappings, use the [simulation deployment](test-identities.md#prepare-and-deploy-the-scoliosis-test-environment)
and `bash scripts/prepare-scoliosis-simulation.sh`. It keeps the existing data
intact and starts a separate database with a private mapping collection.

> **Scoliosis checkout:** The current deployment configuration uses image `0.0.4-scolios-test` and consent `scoliosis-kontoapi-v1` from `deploy/consent/scoliosis-consent.txt`. The technical-consent commands and 2026-11-03 deployment details below are historical template guidance; use the checked-in manifest and [scoliosis deployment notes](scoliosis-study.md#remote-test-deployment) for this checkout.

This deploys the backend to `https://movement-api.test.appadem.in` in namespace
`appademin`, initially using **KontoAPI test**. The iPhone connects to that public
HTTPS address. Only the backend needs network access to ADFS and KontoAPI;
neither the phone nor the development Mac needs VPN for normal app testing.
Cluster administration and registry access may still need the GU network.

This is a technical test deployment with synthetic identities/data, not a study
launch. See [the workflow](workflow.md) before adapting it for participants.

## Prepared configuration

| File | Purpose |
| --- | --- |
| `deploy/kustomization.yaml` | Combines the test resources in namespace `appademin`; hashes configuration so changes trigger a rollout. |
| `deploy/api.yaml` | One replica with Recreate rollout, HTTPS Route, health probes, runtime secrets and consent initialization. |
| `deploy/kontoapi-test.env` | Nonsecret provider URLs, public guardian URL and trusted ingress peers. |
| `deploy/test-consent.txt` | Explicitly labelled technical consent mounted into the init container. |
| `deploy/flutter-test.json` | Flutter's remote API URL; contains no provider credentials. |

The existing deployment/service/route names and existing 2 GiB
`movement-api-pv-claim` are reused. The claim holds the database, raw uploads and
PocketBase files. It is managed separately and is not recreated by Kustomize.
The initial schema replacement requires an empty database; the owner authorized
discarding all pre-production data. Stop the API before that one-time reset.
Normal redeployments must retain the data. Restarting interrupts pending
authentication attempts; persistent identities/consent survive.

The init container runs `bankid publish-consent --if-missing` against the same
volume the API uses. Repeated initialization reuses an identical version without
creating duplicates or reactivating a retired document. Changed text/title under
the same version fails initialization; publish a new version instead. The normal
publication command still rejects duplicates unless `--if-missing` is supplied.

## Before rollout

The cluster needs:

- The existing `movement-api-pv-claim` in Bound state. It is a 2 GiB
  ReadWriteOnce volume using `block-standard` on this cluster.
- Permission for the namespace service account to pull from
  `registry.k8s.gu.se/appademin/movement-api`; retain the existing registry pull
  credentials or have the cluster operator provision them.
- DNS and outbound HTTPS from the **pod** to `idptest.it.gu.se:443` and
  `sptest.it.gu.se:9444`. Health checks alone do not prove provider access.
- Valid public TLS for `movement-api.test.appadem.in`. The manifest retains the
  existing ACME annotation and HTTP-to-HTTPS redirect.
- Verified ingress peer CIDRs in `deploy/kontoapi-test.env`. The checked-in
  addresses were previously observed on this cluster; confirm with its operator.
  The route replaces forwarded headers, and the API trusts only these peers to
  derive the end-user IP. Do not broaden this to all addresses.

The image supports arbitrary OpenShift user IDs and root-group access. Leave
UID/fsGroup assignment to the namespace's security policy. The mounted data
volume must be writable and the mounted keyring readable under that policy.
See [Red Hat's image guidance](https://developers.redhat.com/articles/2021/11/11/best-practices-building-images-pass-red-hat-container-certification).

## Credentials and evidence keyring

On the development Mac, the supplied test credentials have been copied without
printing values into ignored `deploy/secrets/kontoapi-test.env`. A separate
keyring for this fresh remote test database is in
`deploy/secrets/study-keys.json`. Both files are private and excluded from Git;
the image build context is `api/`, so neither enters the image.

On another checkout, prepare those files before continuing:

```sh
mkdir -p deploy/secrets
chmod 700 deploy/secrets
# Only if the destination does not already exist:
cp -n deploy/kontoapi-credentials.env.example deploy/secrets/kontoapi-test.env
chmod 600 deploy/secrets/kontoapi-test.env
# Edit the three values using the KontoAPI test credentials.
python3 scripts/create-study-secrets.py --out deploy/secrets/study-keys.json
```

The key generator refuses to overwrite a file. Retain this keyring with any
retained database/evidence; do not generate a new one on each deployment.

After logging in to the intended cluster, provision the two Secrets. These
commands stream their contents directly between `oc` processes:

```sh
oc login https://api.k8s.gu.se:6443
oc project appademin
oc whoami

oc -n appademin create secret generic movement-kontoapi-test \
  --from-env-file=deploy/secrets/kontoapi-test.env \
  --dry-run=client -o yaml | oc -n appademin apply -f -
oc -n appademin create secret generic movement-study-keys-test \
  --from-file=study-keys.json=deploy/secrets/study-keys.json \
  --dry-run=client -o yaml | oc -n appademin apply -f -
```

Do not run these with placeholder values. The first Secret contains only
`KONTOAPI_CLIENT_ID`, `KONTOAPI_CLIENT_SECRET`, and `KONTOAPI_API_KEY`.
KontoAPI credentials are never passed to Flutter. Later credential changes need
an API rollout to refresh its environment variables.

## Build, publish and apply

Run from the repository root. The image is built for `linux/amd64` even on an
Apple Silicon Mac; confirm that architecture matches the cluster. Both the init
container and API use the same image/tag.

```sh
docker buildx build --platform linux/amd64 --load \
  --tag registry.k8s.gu.se/appademin/movement-api:0.0.3-kontoapi-test api
oc whoami -t | docker login registry.k8s.gu.se \
  --username "$(oc whoami)" --password-stdin
docker push registry.k8s.gu.se/appademin/movement-api:0.0.3-kontoapi-test

mkdir -p build
kubectl kustomize deploy > build/kontoapi-test-manifest.yaml
oc -n appademin apply --dry-run=server -f build/kontoapi-test-manifest.yaml
oc -n appademin apply -f build/kontoapi-test-manifest.yaml
oc -n appademin rollout status deployment/movement-api --timeout=180s
oc -n appademin get pvc movement-api-pv-claim
```

Use a modern `kubectl` to render, then `oc` to apply. The old `oc` on this Mac
cannot render the `envs` ConfigMap generator. Applying `deploy/api.yaml` alone
is insufficient: its generated ConfigMaps are also required.
[Kubernetes documents the rendering and name-reference behavior](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/).

The registry uses the OpenShift login token, streamed to Docker without printing
it or placing it in a command argument. Renew `oc login` first if authentication
fails; see [OpenShift registry authentication](https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/registry/accessing-the-registry).

For later image builds use a new tag and update **both** image references, or
set an `images` transformer in `deploy/kustomization.yaml`. Reusing an unchanged
tag/configuration does not trigger a new rollout by itself.

If rollout stalls, inspect the init container first:

```sh
oc -n appademin logs deployment/movement-api -c publish-test-consent
oc -n appademin logs deployment/movement-api -c movement-api --tail=80
oc -n appademin get pods -l app=movement-api
oc -n appademin describe pvc movement-api-pv-claim
```

## Verify before using the phone

```sh
curl --fail --show-error https://movement-api.test.appadem.in/api/health
curl --silent --output /dev/null --write-out '%{http_code}\n' \
  https://movement-api.test.appadem.in/api/me
curl --silent --output /dev/null --write-out '%{http_code}\n' \
  https://movement-api.test.appadem.in/api/consent/current

oc -n appademin exec deployment/movement-api -c movement-api -- \
  /pb/app bankid check-kontoapi --dir=/pb/pb_data
```

Expect health 200 and both unauthenticated routes 401. The in-pod diagnostic
must report an authentication order, QR image, and pending status. It uses the
deployed credentials, TLS trust and network path without printing identity or
credential data. It creates no study session or consent. The unused upstream
order expires naturally because KontoAPI documents no cancellation endpoint.

Confirm the init log reports `kontoapi-technical-v1` with `current=true` on this
fresh deployment. Then complete identity verification on the phone and check
that `/consent` displays that document before explicit acceptance. Test guardian
invitations using the same public HTTPS origin and the appropriate test identities.
Follow [manual review](manual-review.md) for the remaining flows and HealthKit.

## Run the iPhone against test

```sh
PATH="$(xcode-select -p)/usr/bin:$PATH" \
  flutter run -d 00008120-001A19393693C01E \
  --dart-define-from-file=deploy/flutter-test.json
```

The device ID above is the connected development iPhone; use `flutter devices`
on another machine. Rebuild/relaunch when changing a Dart define; hot reload
does not replace the compiled API URL. Sign out of any old local-backend session
and authenticate against the new database. Return manually from BankID.

The phone can use ordinary Wi-Fi or cellular internet. The Mac needs its normal
device debugging connection, not a LAN connection to the backend.

## KontoAPI production access

The backend already accepts `KONTOAPI_ENVIRONMENT=production`. That flag does
not select provider URLs automatically: supply the confirmed production
`KONTOAPI_URL`, `KONTOAPI_TOKEN_URL` and all three matching production credentials
together. Production endpoint/credential values are intentionally not guessed.
Keep production-connected tests in a separate deployment/database/keyring so
test and real identities do not share state. Replace the technical consent's
test-identity wording with suitable reviewed text before using real identities.

## Preparation verification — 2026-11-03

- All backend tests and the native Go build passed, including restart-safe
  consent initialization, rejection of changed content and preservation of a
  later current document.
- The `linux/amd64` image was built locally as
  `registry.k8s.gu.se/appademin/movement-api:0.0.3-kontoapi-test`.
- Container checks passed with UID `1001230000`, group `0`, a read-only root
  filesystem and disposable data: repeated consent initialization, API health
  200, and anonymous session/consent requests 401. These used synthetic
  credentials, without contacting KontoAPI.
- Kustomize rendering and configuration/volume references passed local checks.
  Both private deployment files are mode `0600` and Git-ignored.

## Deployed verification — 2026-11-03

- Deployed in `appademin` at `https://movement-api.test.appadem.in`, with one
  ready replica of `0.0.3-kontoapi-test`. Published image digest:
  `sha256:0b52e7ac43746ac85030532e8d4b9ba87f2d0e1fb75933849d84566d20577a6d`.
- Reused the existing Bound 2 GiB `movement-api-pv-claim`. Its previous test data
  was cleared once, with the API stopped, as authorized by the owner. The reset
  job was deleted after success. No additional data claim remains.
- Runtime Secrets are installed. Consent `kontoapi-technical-v1` was published
  as current by the init container (record `lk9s8phcmurk9i1`).
- Server-side manifest validation and rollout passed. Public HTTPS health
  returned 200; anonymous `/api/me` and `/api/consent/current` returned 401.
- The in-pod KontoAPI diagnostic created a real test authentication order,
  retrieved its QR image, and received pending status.
- Through the public HTTPS API, both QR and same-device orders started, status
  polling worked, QR PNG/BankID launch URL formats were valid, and local stop
  returned cancelled. No identity was completed, session granted or consent
  accepted by these diagnostics.
- All three configured ingress peer IPs were observed among the pod's HTTP
  connections during public endpoint checks.

Still requiring manual verification: completion of a test BankID identity,
explicit participant consent and receipt, guardian approval, and native HealthKit
permission/read/upload flows on the iPhone. Relaunch Flutter using the remote
define file above before testing; an already installed local build keeps its
compiled LAN API address.
