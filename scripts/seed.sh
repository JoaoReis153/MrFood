#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"
source gcp.env

echo "▶ Bootstrapping Cloud SQL (schemas + seed CSVs)..."
bash "${SCRIPT_DIR}/bootstrap_cloud.sh" "$@"
