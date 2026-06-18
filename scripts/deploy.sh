#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"
source gcp.env
source secrets.env
export TF_VAR_project_id="${GCP_PROJECT_ID}"

# ---------------------------------------------------------------------------
# 1. Authenticate
# ---------------------------------------------------------------------------
gcloud auth login
gcloud auth application-default login
gcloud config set project "${GCP_PROJECT_ID}"

# ---------------------------------------------------------------------------
# 2. Infrastructure — Terraform
# ---------------------------------------------------------------------------

# WIF pool and provider survive terraform destroy (GCP soft-deletes them for
# 30 days, or leaves them active if the provider blocked pool deletion).
# Reconcile state before applying: undelete if soft-deleted, import if active.
wif_reconcile() {
  local resource="$1"   # terraform resource address
  local gcp_id="$2"     # full GCP resource ID for import
  local undelete_cmd="$3"  # gcloud undelete command (or empty string)

  (cd terraform && terraform state show "${resource}" &>/dev/null) && return 0

  echo "▶ ${resource} missing from state — reconciling..."
  [[ -n "${undelete_cmd}" ]] && eval "${undelete_cmd}" 2>/dev/null || true
  (cd terraform && terraform import "${resource}" "${gcp_id}") 2>/dev/null || true
}

(cd terraform && terraform init -reconfigure)

wif_reconcile \
  "google_iam_workload_identity_pool.github" \
  "projects/${GCP_PROJECT_ID}/locations/global/workloadIdentityPools/github" \
  "gcloud iam workload-identity-pools undelete github --location=global --project=${GCP_PROJECT_ID} --quiet"

wif_reconcile \
  "google_iam_workload_identity_pool_provider.mrfood_repo" \
  "projects/${GCP_PROJECT_ID}/locations/global/workloadIdentityPools/github/providers/mrfood-repo" \
  "gcloud iam workload-identity-pools providers undelete mrfood-repo --workload-identity-pool=github --location=global --project=${GCP_PROJECT_ID} --quiet"

(
  cd terraform
  terraform plan -out=tfplan
  terraform apply tfplan
  rm tfplan
)

# ---------------------------------------------------------------------------
# 3. Container images — build & push
# ---------------------------------------------------------------------------
gcloud auth configure-docker "${GCP_REGION}-docker.pkg.dev"

./services/build_and_push_images.sh "$(git rev-parse --short HEAD)"

# ---------------------------------------------------------------------------
# 5. Connect to GKE
# ---------------------------------------------------------------------------
gcloud container clusters get-credentials "${GKE_CLUSTER}" \
  --zone "${GCP_ZONE}" \
  --project "${GCP_PROJECT_ID}"

kubectl apply -f kubernetes/namespace.yaml

# ---------------------------------------------------------------------------
# 6. Observability stack (must be up before app services)
# ---------------------------------------------------------------------------
helm upgrade --install otel-collector kubernetes/helm/otel-collector \
  --set gcpProject="${GCP_PROJECT_ID}" \
  --namespace "${K8S_NAMESPACE}"

helm upgrade --install observability kubernetes/helm/observability \
  --namespace "${K8S_NAMESPACE}"

# ---------------------------------------------------------------------------
# 7. Keycloak (must be up before auth)
# ---------------------------------------------------------------------------
helm upgrade --install keycloak kubernetes/helm/keycloak \
  --namespace "${K8S_NAMESPACE}"

kubectl rollout status deployment/keycloak -n "${K8S_NAMESPACE}"

# ---------------------------------------------------------------------------
# 8. Search stack — Elasticsearch + Kafka (must be up before CDC)
# ---------------------------------------------------------------------------
helm upgrade --install elasticsearch kubernetes/helm/elasticsearch \
  --namespace "${K8S_NAMESPACE}"

helm upgrade --install kafka kubernetes/helm/kafka \
  --namespace "${K8S_NAMESPACE}"

kubectl rollout status deployment/zookeeper     -n "${K8S_NAMESPACE}"
kubectl rollout status deployment/kafka         -n "${K8S_NAMESPACE}"
kubectl rollout status deployment/elasticsearch -n "${K8S_NAMESPACE}"

# ---------------------------------------------------------------------------
# 9. CDC (Kafka Connect)
# ---------------------------------------------------------------------------
helm upgrade --install cdc kubernetes/helm/kafka-connect \
  -f kubernetes/values/cdc.yaml \
  --set "gcpProjectId=${GCP_PROJECT_ID}" \
  --namespace "${K8S_NAMESPACE}"

kubectl rollout status deployment/cdc -n "${K8S_NAMESPACE}" --timeout 5m

kubectl exec -n "${K8S_NAMESPACE}" deployment/cdc -- bash -c \
  "curl -sf http://localhost:8083/connectors | grep -q restaurant-postgres-source || \
   curl -sf -X POST http://localhost:8083/connectors \
     -H 'Content-Type: application/json' \
     -d @/connectors/restaurant-source.json"

kubectl exec -n "${K8S_NAMESPACE}" deployment/cdc -- bash -c \
  "curl -sf http://localhost:8083/connectors | grep -q restaurants-elasticsearch-sink || \
   curl -sf -X POST http://localhost:8083/connectors \
     -H 'Content-Type: application/json' \
     -d @/connectors/restaurants-sink.json"

# ---------------------------------------------------------------------------
# 10. Application services + gateway
# ---------------------------------------------------------------------------
bash kubernetes/restart.sh

kubectl get svc gateway -n "${K8S_NAMESPACE}" --watch
