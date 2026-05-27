#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"
source gcp.env

echo "⚠️  This will destroy ALL infrastructure in project ${GCP_PROJECT_ID}."
read -r -p "Type the project ID to confirm: " confirm

if [[ "${confirm}" != "${GCP_PROJECT_ID}" ]]; then
  echo "Aborted."
  exit 1
fi

(
  cd terraform
  terraform init
  terraform destroy
)
