# MrFood Deployment Guide

## Overview

| Layer                | Tool                                           | Trigger                                          |
| -------------------- | ---------------------------------------------- | ------------------------------------------------ |
| Infrastructure       | Terraform                                      | Push to `main` → `terraform/**`                  |
| Container images     | Docker + Artifact Registry                     | `services/build_and_push_images.sh` / CI         |
| Kubernetes workloads | Helm                                           | `kubernetes/restart.sh` / CI                     |
| Observability        | Self-hosted (Prometheus, Loki, Tempo, Grafana) | Helm                                             |
| Search               | Elasticsearch + Kafka + Kafka Connect          | Helm (`elasticsearch`, `kafka`, `kafka-connect`) |

| Layer                | Tool                                           | Trigger                                          |
| -------------------- | ---------------------------------------------- | ------------------------------------------------ |
| Infrastructure       | Terraform                                      | Push to `main` → `terraform/**`                  |
| Container images     | Docker + Artifact Registry                     | `services/build_and_push_images.sh` / CI         |
| Kubernetes workloads | Helm                                           | `kubernetes/restart.sh` / CI                     |
| Observability        | Self-hosted (Prometheus, Loki, Tempo, Grafana) | Helm                                             |
| Search               | Elasticsearch + Kafka + Kafka Connect          | Helm (`elasticsearch`, `kafka`, `kafka-connect`) |

---

## Prerequisites

> **Changing the GCP project?** Edit one line in **`gcp.env`** at the repo root:
>
> ```
> GCP_PROJECT_ID=mrfood-496807   ← change this
> ```
>
> All scripts, CI workflows, and Helm deploys read from that file automatically.

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
source gcp.env

gcloud auth login
gcloud auth application-default login
gcloud config set project "${GCP_PROJECT_ID}"
```

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
source gcp.env  # exports TF_VAR_project_id for Terraform
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

### Connect to the GKE cluster

```bash
gcloud container clusters get-credentials mrfood-cluster --zone europe-southwest1-b --project "${GCP_PROJECT_ID}"
```

### Namespace

```bash
kubectl apply -f kubernetes/namespace.yaml
```

### Observability stack

Deploy first — services depend on the OTel Collector being reachable at `otel-collector:4317`.

```bash
source gcp.env

# OTel Collector (receives from services, forwards to Tempo/Loki/Prometheus)
helm upgrade --install otel-collector kubernetes/helm/otel-collector \
  --set gcpProject="${GCP_PROJECT_ID}" \
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
source gcp.env
helm upgrade --install cdc kubernetes/helm/kafka-connect \
  -f kubernetes/values/cdc.yaml \
  --set "gcpProjectId=${GCP_PROJECT_ID}" \
  --namespace mrfood

# Wait for CDC to be ready (autoscaler may need to provision a new node — this can take 1-2 min)
kubectl rollout status deployment/cdc -n mrfood --timeout 5m
```

After CDC is running, register the connectors (connector configs are baked into the image at `/connectors/`):

```bash
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
```

### Application services

Deploys all services and the gateway (skips `cdc` and `search` which have dedicated charts above):

```bash
bash kubernetes/restart.sh
```

### Kong gateway

> **Note:** After updating `services/gateway/kong/kong.yml`, patch the live ConfigMap and restart — Helm does not auto-update it:
>
> ```bash
> kubectl delete configmap kong-config -n mrfood --ignore-not-found
> kubectl create configmap kong-config -n mrfood \
>   --from-file=kong.yml=services/gateway/kong/kong.yml
> kubectl rollout restart deployment/gateway -n mrfood
> ```

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
source gcp.env

# List all buckets in the project
gcloud storage buckets list --project="${GCP_PROJECT_ID}"

# Inspect the schema/seed bucket specifically
gsutil ls -l gs://kaggle_bucket_6194
gsutil ls gs://kaggle_bucket_6194/processed_data/
```

### How it works

The script calls `gcloud sql import csv` for each file already present in the bucket, which runs a PostgreSQL `COPY FROM` under the hood. The bucket IAM is already wired by Terraform (`roles/storage.objectViewer` on the Cloud SQL service account).

| CSV file                                              | Database            | Table                   |
| ----------------------------------------------------- | ------------------- | ----------------------- |
| `processed_data/auth/app_user.csv`                    | `mrfood_auth`       | `app_user`              |
| `processed_data/restaurant/restaurants.csv`           | `mrfood_restaurant` | `restaurants`           |
| `processed_data/restaurant/restaurant_categories.csv` | `mrfood_restaurant` | `restaurant_categories` |
| `processed_data/review/review.csv`                    | `mrfood_review`     | `review`                |

### Load all seed data

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
| Workflow           | File                       | Trigger                                    | What it does                          |
| ------------------ | -------------------------- | ------------------------------------------ | ------------------------------------- |
| Lint & Test        | `ci.yml`                   | PR → `services/**`                         | Lints and tests changed services only |
| Terraform Validate | `terraform_validation.yml` | PR → `terraform/**`                        | fmt, validate, plan                   |
| Terraform Apply    | `terraform_deploy.yml`     | Push to `main` → `terraform/**`            | `terraform apply`                     |
| Bruno API Tests    | `bruno.yml`                | PR → `tests/**`, `services/**`, `Makefile` | End-to-end API smoke tests            |
