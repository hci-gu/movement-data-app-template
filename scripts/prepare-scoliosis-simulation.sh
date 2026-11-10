#!/usr/bin/env bash
set -euo pipefail
study_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$study_root"
mkdir -p build deploy/secrets
chmod 700 deploy/secrets
if [[ ! -e deploy/secrets/study-keys-simulation-v1.json ]]; then
  python3 scripts/create-study-secrets.py --out deploy/secrets/study-keys-simulation-v1.json
fi
kubectl kustomize deploy-simulation > build/scoliosis-simulation-manifest.yaml
docker buildx build --platform linux/amd64 --load \
  --build-arg GO_BUILD_TAGS=study_simulation \
  --tag registry.k8s.gu.se/appademin/movement-api:0.0.6-scolios-simulation api
echo 'Simulation image and build/scoliosis-simulation-manifest.yaml are prepared. No cluster changes made.'
echo 'Deploy using docs/test-identities.md, then add real-to-test mappings in PocketBase.'
