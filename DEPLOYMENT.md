# MrFood Deployment Guide

## Overview

| Layer                | Tool                                           | Trigger                                          |
| -------------------- | ---------------------------------------------- | ------------------------------------------------ |
| Infrastructure       | Terraform                                      | Push to `main` → `terraform/**`                  |
| Container images     | Docker + Artifact Registry                     | `services/build_and_push_images.sh` / CI         |
| Kubernetes workloads | Helm                                           | `kubernetes/restart.sh` / CI                     |
| Observability        | Self-hosted (Prometheus, Loki, Tempo, Grafana) | Helm                                             |
| Search               | Elasticsearch + Kafka + Kafka Connect          | Helm (`elasticsearch`, `kafka`, `kafka-connect`) |

---

## Prerequisites

```bash
gcloud --version     # >= 400
terraform --version  # >= 1.5
kubectl version
helm version
docker version
```

### Project ID

The GCP project is controlled by a single env var: `TF_VAR_project_id`. Add it to your shell profile (`~/.zshrc` or `~/.bashrc`) so it's always set:

```bash
export TF_VAR_project_id="mrfood-496807"
```

To switch projects, change that line and reload your shell (`source ~/.zshrc`). Everything below (Terraform, scripts, gcloud commands) will pick it up automatically.

Authenticate locally:

```bash
gcloud auth login
gcloud auth application-default login
gcloud config set project $TF_VAR_project_id
gcloud auth application-default set-quota-project $TF_VAR_project_id
```

Connect to the GKE cluster:

```bash
gcloud container clusters get-credentials mrfood-cluster \
  --zone europe-southwest1-b \
  --project $TF_VAR_project_id
```

---

## 1. Infrastructure — Terraform

Terraform manages: VPC, GKE cluster, Artifact Registry, Cloud SQL instance, Redis, and all Workload Identity service accounts.

### Secrets

DB passwords live in `terraform/terraform.tfvars` (gitignored). Create it before the first apply:

```hcl
# terraform/terraform.tfvars
service_databases = {
  restaurant = { db_name = "mrfood_restaurant", db_user = "mrfood_restaurant_user", db_password = "REPLACE_ME" }
  booking    = { db_name = "mrfood_booking",    db_user = "mrfood_booking_user",    db_password = "REPLACE_ME" }
  review     = { db_name = "mrfood_review",     db_user = "mrfood_review_user",     db_password = "REPLACE_ME" }
  payment    = { db_name = "mrfood_payment",    db_user = "mrfood_payment_user",    db_password = "REPLACE_ME" }
  sponsor    = { db_name = "mrfood_sponsor",    db_user = "mrfood_sponsor_user",    db_password = "REPLACE_ME" }
}
```

> **Note:** `auth` has no Cloud SQL database — user storage is handled entirely by Keycloak.

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
./services/build_and_push_images.sh $(git rev-parse HEAD)

# Dry run to preview changes without building
./services/build_and_push_images.sh $(git rev-parse HEAD) --dry-run
```

---

## 3. Kubernetes — Helm

Run the deployment script — it handles namespace, observability, Keycloak, Elasticsearch, Kafka, CDC, connector registration, application services, and the Kong gateway in the correct order:

```bash
bash kubernetes/deploy_helm.sh
```

Options:

```
--skip-connectors   Skip CDC connector registration (if already present)
--skip-restart      Skip application services (kubernetes/restart.sh)
--timeout <dur>     Rollout timeout for kubectl (default: 5m)
```

> **Note:** The Kong ConfigMap is not auto-updated by Helm. The script patches it on every run, but if you update `services/gateway/kong/kong.yml` independently, re-run the script (or use `--skip-connectors --skip-restart` to only patch the gateway).

---

## 4. Verification

### Pods

```bash
kubectl get pods -n mrfood
```

All pods should reach `Running`. Common failure modes:

| Symptom                            | Cause                                  | Fix                                                                  |
| ---------------------------------- | -------------------------------------- | -------------------------------------------------------------------- |
| `cloud-sql-proxy` CrashLoopBackOff | Workload Identity not propagated       | Wait 60 s, then `kubectl rollout restart deployment/<svc> -n mrfood` |
| Service pod CrashLoopBackOff       | Missing env var or wrong DB password   | `kubectl logs -n mrfood deployment/<svc>`                            |
| OTel Collector failing             | Loki/Tempo not ready yet               | Deploy observability first, then restart collector                   |
| Loki/Prometheus/Tempo Pending      | PVCs not created                       | `helm upgrade observability kubernetes/helm/observability -n mrfood` |
| Gateway request hanging            | Stale kong-config ConfigMap            | Patch ConfigMap manually (see Kong gateway note above)               |
| Auth requests hanging              | Keycloak not running                   | Deploy Keycloak before auth; restart auth after Keycloak is ready    |
| `search` pod CrashLoopBackOff      | Elasticsearch not reachable            | Deploy elasticsearch chart first, wait for readiness                 |
| `cdc` pod not ready                | Kafka not up or ES not ready           | Deploy kafka chart first; CDC readiness probe waits on `/connectors` |
| Connectors not registered          | CDC deployed but connectors not POSTed | Run the `kubectl exec` connector registration commands above         |

### Kong external IP

```bash
kubectl get svc gateway -n mrfood
# EXTERNAL-IP appears after ~2 min

curl http://<EXTERNAL-IP>/restaurants
```

### Grafana

```bash
kubectl get svc grafana -n mrfood
# EXTERNAL-IP appears after ~2 min — open http://<EXTERNAL-IP>  (admin / admin)
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

# Generate seed data and load into local containers
make generate-csv
make load-local

# Run tests
make test

# View logs
make logs
```

See `Makefile` for the full list of commands.

---

## 6. Seed Data

Processed CSV files live under `scripts/processed_data/` and are generated by `make generate-csv`.

### Local

Loads Keycloak users via the Admin API and seeds Postgres containers directly via `psql COPY`. Tables are truncated before each load, making it idempotent.

```bash

# List all buckets in the project
gcloud storage buckets list --project=$TF_VAR_project_id

# Preview without executing
./scripts/load_seed_data_local.sh --dry-run
```

### Cloud (Cloud SQL via GCS)

Uploads CSVs to GCS and imports them into Cloud SQL via `gcloud sql import`. Databases and tables must already exist (schemas are applied by `terraform apply`).

```bash
# List buckets
gcloud storage buckets list --project=$TF_VAR_project_id
gsutil ls gs://kaggle_bucket_6194/processed_data/

# Preview without executing
./scripts/load_seed_data_cloud.sh --dry-run

# Run
./scripts/load_seed_data_cloud.sh
```

| CSV file                                              | Destination                                         |
| ----------------------------------------------------- | --------------------------------------------------- |
| `processed_data/auth/users.csv`                       | Keycloak `mrfood` realm (Admin API)                 |
| `processed_data/restaurant/restaurants.csv`           | Cloud SQL `mrfood_restaurant.restaurants`           |
| `processed_data/restaurant/restaurant_categories.csv` | Cloud SQL `mrfood_restaurant.restaurant_categories` |
| `processed_data/review/review.csv`                    | Cloud SQL `mrfood_review.review`                    |

See `SEED_DATA_CREDENTIALS.md` for the default test password (`mrfood123`).

---

## CI / CD Summary

| Workflow           | File                       | Trigger                                    | What it does                          |
| ------------------ | -------------------------- | ------------------------------------------ | ------------------------------------- |
| Lint & Test        | `ci.yml`                   | PR → `services/**`                         | Lints and tests changed services only |
| Terraform Validate | `terraform_validation.yml` | PR → `terraform/**`                        | fmt, validate, plan                   |
| Terraform Apply    | `terraform_deploy.yml`     | Push to `main` → `terraform/**`            | `terraform apply`                     |
| Bruno API Tests    | `bruno.yml`                | PR → `tests/**`, `services/**`, `Makefile` | End-to-end API smoke tests            |
