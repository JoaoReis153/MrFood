#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"
source gcp.env

NAMESPACE="mrfood"

gcloud config set project "${GCP_PROJECT_ID}"
gcloud container clusters get-credentials mrfood-cluster \
  --zone europe-southwest1-b \
  --project "${GCP_PROJECT_ID}"

# Uninstall all Helm releases in the namespace
for release in $(helm list -n "${NAMESPACE}" -q); do
  echo "Uninstalling ${release}..."
  helm uninstall "${release}" -n "${NAMESPACE}"
done

# Delete the namespace (removes any remaining resources)
kubectl delete namespace "${NAMESPACE}" --ignore-not-found
