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
  # Remove the private VPC connection from state before destroy — GCP doesn't
  # allow Terraform to delete it while peering connections still exist.
  terraform state rm module.cloudsql_foundation.google_service_networking_connection.private_vpc_connection 2>/dev/null || true
  terraform destroy
)
