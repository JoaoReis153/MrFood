# MrFood Deployment Guide

## Overview

| Layer | Tool | Trigger |
|---|---|---|
| Infrastructure | Terraform | Push to `main` → `terraform/**` |
| Container images | Docker + Artifact Registry | Manual / CI |
| Kubernetes workloads | Helm | `kubernetes/restart.sh` / CI |
| Observability | Self-hosted (Prometheus, Loki, Tempo, Grafana) | Helm |

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
  --region europe-southwest1 \
  --project mrfood-490623
```

---

## 1. Infrastructure — Terraform

Terraform manages: VPC, GKE cluster, Artifact Registry, Cloud SQL instances (×6), Redis (×2), and all Workload Identity service accounts.

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

Build and push all services:

```bash
REGISTRY=europe-southwest1-docker.pkg.dev/mrfood-490623/mrfood-repo
TAG=$(git rev-parse --short HEAD)

for svc in auth restaurant booking review payment sponsor notification search cdc; do
  docker buildx build --platform linux/amd64 \
    -t "${REGISTRY}/${svc}:${TAG}" \
    -t "${REGISTRY}/${svc}:latest" \
    "services/${svc}" \
    --push
done
```

Update the image tag in each `kubernetes/values/<service>.yaml` before deploying:

```yaml
image: europe-southwest1-docker.pkg.dev/mrfood-490623/mrfood-repo/auth:$TAG
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

### Application services

```bash
for svc in auth restaurant booking review payment sponsor notification search cdc; do
  helm upgrade --install $svc kubernetes/helm/mrfood-service \
    -f kubernetes/values/$svc.yaml \
    --namespace mrfood
done
```

Or use the existing script (redeploys auth, booking, notification, payment, restaurant, review, sponsor, and gateway — **skips `cdc` and `search`**):

```bash
bash kubernetes/restart.sh
```

### Kong gateway

```bash
helm upgrade --install gateway kubernetes/helm/kong \
  -f kubernetes/values/gateway.yaml \
  --namespace mrfood
```

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

---

## 6. Local Development

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
