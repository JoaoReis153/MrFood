# MrFood Deployment Guide

## Overview

| Layer | Tool | Trigger |
|---|---|---|
| Infrastructure | Terraform | Push to `main` → `terraform/**` |
| Container images | Docker + Artifact Registry | `services/build_and_push_images.sh` / CI |
| Kubernetes workloads | Helm | `kubernetes/restart.sh` / CI |
| Observability | Self-hosted (Prometheus, Loki, Tempo, Grafana) | Helm |
| Search | Elasticsearch + Kafka + Kafka Connect | Helm (`elasticsearch`, `kafka`, `kafka-connect`) |

---

## Prerequisites

```bash
gcloud --version     # >= 400
terraform --version  # >= 1.5
kubectl version
helm version
docker version
```

Authenticate locally:

```bash
gcloud auth login
gcloud auth application-default login
gcloud config set project mrfood-490623
```

Connect to the GKE cluster:

```bash
gcloud container clusters get-credentials mrfood-cluster \
  --zone europe-southwest1-b \
  --project mrfood-490623
```

---

## 1. Infrastructure — Terraform

Terraform manages: VPC, GKE cluster, Artifact Registry, Cloud SQL instance, Redis, and all Workload Identity service accounts.

### Secrets

DB passwords live in `terraform/terraform.tfvars` (gitignored). Create it before the first apply:

```hcl
# terraform/terraform.tfvars
service_databases = {
  auth = {
    db_name     = "mrfood_auth"
    db_user     = "mrfood_auth_user"
    db_password = "REPLACE_ME"
  }
  restaurant = {
    db_name     = "mrfood_restaurant"
    db_user     = "mrfood_restaurant_user"
    db_password = "REPLACE_ME"
  }
  booking = {
    db_name     = "mrfood_booking"
    db_user     = "mrfood_booking_user"
    db_password = "REPLACE_ME"
  }
  review = {
    db_name     = "mrfood_review"
    db_user     = "mrfood_review_user"
    db_password = "REPLACE_ME"
  }
  payment = {
    db_name     = "mrfood_payment"
    db_user     = "mrfood_payment_user"
    db_password = "REPLACE_ME"
  }
  sponsor = {
    db_name     = "mrfood_sponsor"
    db_user     = "mrfood_sponsor_user"
    db_password = "REPLACE_ME"
  }
}
```

### Apply

```bash
cd terraform
terraform init
terraform plan    # review before applying
terraform apply
```

**CI:** any push to `main` that touches `terraform/**` triggers `terraform apply` automatically via `.github/workflows/terraform_deploy.yml`.

---

## 2. Container Images — Build & Push

Configure Docker for Artifact Registry (one-time per machine):

```bash
gcloud auth configure-docker europe-southwest1-docker.pkg.dev
```

Build and push all services. The script also updates the `image:` tag in each `kubernetes/values/<service>.yaml` automatically:

```bash
./services/build_and_push_images.sh $(git rev-parse --short HEAD)

# Dry run to preview changes without building
./services/build_and_push_images.sh $(git rev-parse --short HEAD) --dry-run
```

---

## 3. Kubernetes — Helm

### Namespace

```bash
kubectl apply -f kubernetes/namespace.yaml
```

### Observability stack

Deploy first — services depend on the OTel Collector being reachable at `otel-collector:4317`.

```bash
# OTel Collector (receives from services, forwards to Tempo/Loki/Prometheus)
helm upgrade --install otel-collector kubernetes/helm/otel-collector \
  --namespace mrfood

# Prometheus, Loki, Tempo, Grafana
helm upgrade --install observability kubernetes/helm/observability \
  --namespace mrfood
```

### Keycloak

The auth service requires Keycloak. Deploy it before auth:

```bash
helm upgrade --install keycloak kubernetes/helm/keycloak \
  --namespace mrfood

kubectl rollout status deployment/keycloak -n mrfood
# Keycloak takes ~60 s to start and import the mrfood realm
```

The `mrfood` realm and `mrfood-auth` client are imported automatically from `kubernetes/helm/keycloak/files/realm-import.json`.

### Search stack (Elasticsearch + Kafka + CDC)

Elasticsearch and Kafka must be running before deploying `search` or `cdc`.

```bash
helm upgrade --install elasticsearch kubernetes/helm/elasticsearch \
  --namespace mrfood

helm upgrade --install kafka kubernetes/helm/kafka \
  --namespace mrfood

# Wait for all three to be ready before deploying CDC
kubectl rollout status deployment/zookeeper -n mrfood
kubectl rollout status deployment/kafka -n mrfood
kubectl rollout status deployment/elasticsearch -n mrfood
```

Deploy the CDC connector (Kafka Connect) using its dedicated chart:

```bash
# Fill in the restaurant DB password in kubernetes/values/cdc.yaml before deploying
helm upgrade --install cdc kubernetes/helm/kafka-connect \
  -f kubernetes/values/cdc.yaml \
  --namespace mrfood
```

After CDC is running, register the connectors (connector configs are baked into the image at `/connectors/`):

```bash
kubectl exec -n mrfood deployment/cdc -- bash -c \
  "curl -sf http://localhost:8083/connectors | grep -q restaurant || \
   curl -sf -X POST http://localhost:8083/connectors \
     -H 'Content-Type: application/json' \
     -d @/connectors/restaurant-source.json"

kubectl exec -n mrfood deployment/cdc -- bash -c \
  "curl -sf http://localhost:8083/connectors | grep -q restaurants-elasticsearch || \
   curl -sf -X POST http://localhost:8083/connectors \
     -H 'Content-Type: application/json' \
     -d @/connectors/restaurants-sink.json"
```

### Application services

Deploys all services and the gateway (skips `cdc` and `search` which have dedicated charts above):

```bash
bash kubernetes/restart.sh
```

### Kong gateway

> **Note:** After updating `services/gateway/kong/kong.yml`, patch the live ConfigMap and restart — Helm does not auto-update it:
> ```bash
> kubectl create configmap kong-config -n mrfood \
>   --from-file=kong.yml=services/gateway/kong/kong.yml \
>   --dry-run=client -o yaml | kubectl apply -f -
> kubectl rollout restart deployment/gateway -n mrfood
> ```

---

## 4. Verification

### Pods

```bash
kubectl get pods -n mrfood
```

All pods should reach `Running`. Common failure modes:

| Symptom | Cause | Fix |
|---|---|---|
| `cloud-sql-proxy` CrashLoopBackOff | Workload Identity not propagated | Wait 60 s, then `kubectl rollout restart deployment/<svc> -n mrfood` |
| Service pod CrashLoopBackOff | Missing env var or wrong DB password | `kubectl logs -n mrfood deployment/<svc>` |
| OTel Collector failing | Loki/Tempo not ready yet | Deploy observability first, then restart collector |
| Loki/Prometheus/Tempo Pending | PVCs not created | `helm upgrade observability kubernetes/helm/observability -n mrfood` |
| Gateway request hanging | Stale kong-config ConfigMap | Patch ConfigMap manually (see Kong gateway note above) |
| Auth requests hanging | Keycloak not running | Deploy Keycloak before auth; restart auth after Keycloak is ready |
| `search` pod CrashLoopBackOff | Elasticsearch not reachable | Deploy elasticsearch chart first, wait for readiness |
| `cdc` pod not ready | Kafka not up or ES not ready | Deploy kafka chart first; CDC readiness probe waits on `/connectors` |
| Connectors not registered | CDC deployed but connectors not POSTed | Run the `kubectl exec` connector registration commands above |

### Kong external IP

```bash
kubectl get svc gateway -n mrfood
# EXTERNAL-IP appears after ~2 min

curl http://<EXTERNAL-IP>/restaurants
```

### Grafana

Grafana is ClusterIP only. Access via port-forward:

```bash
kubectl port-forward -n mrfood svc/grafana 3000:3000
# open http://localhost:3000  (admin / admin)
```

Dashboards provisioned automatically: **MrFood Overview** and **Traces**.

### Observability

```bash
# OTel Collector receiving data
kubectl logs -n mrfood deployment/otel-collector

# Confirm traces appear in Grafana → Explore → Tempo
# Confirm logs appear in Grafana → Explore → Loki
# Confirm metrics appear in Grafana → Explore → Prometheus
```

---

## 5. Local Development

```bash
# Start all core services (Docker Compose)
make setup

# Start with search / CDC
make setup-full

# Run tests
make test

# View logs
make logs
```

See `Makefile` for the full list of commands.

---

## CI / CD Summary

| Workflow | File | Trigger | What it does |
|---|---|---|---|
| Lint & Test | `ci.yml` | PR → `services/**` | Lints and tests changed services only |
| Terraform Validate | `terraform_validation.yml` | PR → `terraform/**` | fmt, validate, plan |
| Terraform Apply | `terraform_deploy.yml` | Push to `main` → `terraform/**` | `terraform apply` |
| Bruno API Tests | `bruno.yml` | PR → `tests/**`, `services/**`, `Makefile` | End-to-end API smoke tests |
