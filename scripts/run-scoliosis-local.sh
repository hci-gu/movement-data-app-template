#!/usr/bin/env bash
set -euo pipefail
study_root="$(cd "$(dirname "$0")/.." && pwd)"
exec python3 "$study_root/scripts/run-scoliosis-kontoapi.py" "$@"
