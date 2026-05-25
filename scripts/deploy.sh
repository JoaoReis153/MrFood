#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"
source gcp.env

# ---------------------------------------------------------------------------
# 1. Authenticate
# ---------------------------------------------------------------------------
gcloud auth login
gcloud auth application-default login
gcloud config set project "${GCP_PROJECT_ID}"

# ---------------------------------------------------------------------------
# 2. Infrastructure — Terraform
# ---------------------------------------------------------------------------
(
  cd terraform
  terraform init
  terraform plan -out=tfplan
  terraform apply tfplan
  rm tfplan
)

# ---------------------------------------------------------------------------
# 3. Container images — build & push
# ---------------------------------------------------------------------------
gcloud auth configure-docker europe-southwest1-docker.pkg.dev

./services/build_and_push_images.sh "$(git rev-parse --short HEAD)"

# ---------------------------------------------------------------------------
# 4. Connect to GKE
# ---------------------------------------------------------------------------
gcloud container clusters get-credentials mrfood-cluster \
  --zone europe-southwest1-b \
  --project "${GCP_PROJECT_ID}"

kubectl apply -f kubernetes/namespace.yaml

# ---------------------------------------------------------------------------
# 5. Observability stack (must be up before app services)
# ---------------------------------------------------------------------------
helm upgrade --install otel-collector kubernetes/helm/otel-collector \
  --set gcpProject="${GCP_PROJECT_ID}" \
  --namespace mrfood

helm upgrade --install observability kubernetes/helm/observability \
  --namespace mrfood

# ---------------------------------------------------------------------------
# 6. Keycloak (must be up before auth)
# ---------------------------------------------------------------------------
helm upgrade --install keycloak kubernetes/helm/keycloak \
  --namespace mrfood

kubectl rollout status deployment/keycloak -n mrfood

# ---------------------------------------------------------------------------
# 7. Search stack — Elasticsearch + Kafka (must be up before CDC)
# ---------------------------------------------------------------------------
helm upgrade --install elasticsearch kubernetes/helm/elasticsearch \
  --namespace mrfood

helm upgrade --install kafka kubernetes/helm/kafka \
  --namespace mrfood

kubectl rollout status deployment/zookeeper    -n mrfood
kubectl rollout status deployment/kafka        -n mrfood
kubectl rollout status deployment/elasticsearch -n mrfood

# ---------------------------------------------------------------------------
# 8. CDC (Kafka Connect)
# ---------------------------------------------------------------------------
helm upgrade --install cdc kubernetes/helm/kafka-connect \
  -f kubernetes/values/cdc.yaml \
  --set "gcpProjectId=${GCP_PROJECT_ID}" \
  --namespace mrfood

kubectl rollout status deployment/cdc -n mrfood --timeout 5m

kubectl exec -n mrfood deployment/cdc -- bash -c \
  "curl -sf http://localhost:8083/connectors | grep -q restaurant-postgres-source || \
   curl -sf -X POST http://localhost:8083/connectors \
     -H 'Content-Type: application/json' \
     -d @/connectors/restaurant-source.json"

kubectl exec -n mrfood deployment/cdc -- bash -c \
  "curl -sf http://localhost:8083/connectors | grep -q restaurants-elasticsearch-sink || \
   curl -sf -X POST http://localhost:8083/connectors \
     -H 'Content-Type: application/json' \
     -d @/connectors/restaurants-sink.json"

# ---------------------------------------------------------------------------
# 9. Application services + gateway
# ---------------------------------------------------------------------------
bash kubernetes/restart.sh

kubectl get svc gateway -n mrfood --watch
