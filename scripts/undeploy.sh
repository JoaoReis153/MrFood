#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"
source gcp.env

gcloud config set project "${GCP_PROJECT_ID}"
gcloud container clusters get-credentials "${GKE_CLUSTER}" \
  --zone "${GCP_ZONE}" \
  --project "${GCP_PROJECT_ID}"

# Uninstall all Helm releases in the namespace
for release in $(helm list -n "${K8S_NAMESPACE}" -q); do
  echo "Uninstalling ${release}..."
  helm uninstall "${release}" -n "${K8S_NAMESPACE}"
done

# Delete the namespace (removes any remaining resources)
kubectl delete namespace "${K8S_NAMESPACE}" --ignore-not-found
