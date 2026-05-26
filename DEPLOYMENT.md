# MrFood Deployment Guide

## First Deploy

Three commands, in order:

```bash
# 1. Infrastructure + schemas
cd terraform && terraform init && terraform apply

# 2. All Kubernetes workloads
./scripts/deploy.sh

# 3. Seed data (first deploy only)
./scripts/bootstrap_cloud.sh
```

That's it. Everything below is reference detail for each step.

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

Authenticate locally:

```bash
source gcp.env

gcloud auth login
gcloud auth application-default login
gcloud config set project "${GCP_PROJECT_ID}"
```

---

## Step 1 — Infrastructure (`terraform apply`)

Terraform manages: VPC, GKE cluster, Artifact Registry, Cloud SQL instance + schemas, Redis, and all Workload Identity service accounts.

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
source gcp.env
cd terraform
terraform init
terraform plan    # review before applying
terraform apply
```

Terraform also applies the DB schemas (`db_setup.sql` for each service) via a `local-exec` provisioner after Cloud SQL is ready.

**CI:** pushes to `main` that touch `terraform/**` run `terraform plan` automatically via `.github/workflows/terraform_deploy.yml`. Apply remains manual.

---

## Step 2 — Kubernetes (`./scripts/deploy.sh`)

`deploy.sh` does everything in the right order: builds and pushes images, connects to GKE, deploys the observability stack, Keycloak, Elasticsearch, Kafka, CDC, and all application services.

```bash
./scripts/deploy.sh
```

### What it deploys, in order

| Step | What                           | Notes                                              |
| ---- | ------------------------------ | -------------------------------------------------- |
| 1    | Authenticate                   | `gcloud auth login` + ADC                          |
| 2    | Terraform                      | Idempotent re-apply to catch any drift             |
| 3    | Build & push images            | Tags from `git rev-parse --short HEAD`             |
| 4    | Connect to GKE                 | `get-credentials` for `mrfood-cluster`             |
| 5    | Namespace                      | `kubectl apply -f kubernetes/namespace.yaml`       |
| 6    | Observability                  | OTel Collector, Prometheus, Loki, Tempo, Grafana   |
| 7    | Keycloak                       | Waits for rollout; realm imported automatically    |
| 8    | Elasticsearch + Kafka          | Waits for all three (zookeeper, kafka, ES)         |
| 9    | CDC (Kafka Connect)            | Registers source + sink connectors after readiness |
| 10   | Application services + gateway | `kubernetes/restart.sh`                            |

### Kong gateway config

After updating `services/gateway/kong/kong.yml`, patch the live ConfigMap manually — Helm does not auto-update it:

```bash
kubectl delete configmap kong-config -n mrfood --ignore-not-found
kubectl create configmap kong-config -n mrfood \
  --from-file=kong.yml=services/gateway/kong/kong.yml
kubectl rollout restart deployment/gateway -n mrfood
```

---

## Step 3 — Seed Data (`./scripts/bootstrap_cloud.sh`)

Imports the processed CSVs into Cloud SQL. Run once on a fresh database.

```bash
./scripts/bootstrap_cloud.sh

# Preview without executing
./scripts/bootstrap_cloud.sh --dry-run
```

| CSV file                                              | Destination                                         |
| ----------------------------------------------------- | --------------------------------------------------- |
| `processed_data/restaurant/restaurants.csv`           | Cloud SQL `mrfood_restaurant.restaurants`           |
| `processed_data/restaurant/restaurant_categories.csv` | Cloud SQL `mrfood_restaurant.restaurant_categories` |
| `processed_data/review/review.csv`                    | Cloud SQL `mrfood_review.review`                    |

> **Note:** The import uses PostgreSQL `COPY FROM` internally and will fail on duplicate primary keys. This is intentional — it prevents accidental re-seeding of a live database.

---

## Verification

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

### Gateway

```bash
kubectl get svc gateway -n mrfood
# EXTERNAL-IP appears after ~2 min

curl http://<EXTERNAL-IP>/restaurants
```

### Grafana

```bash
kubectl get svc grafana -n mrfood
# open http://<EXTERNAL-IP>  (admin / admin)
```

Dashboards provisioned automatically: **MrFood Overview** and **Traces**.

---

## Local Development

```bash
make setup        # start all core services (Docker Compose)
make setup-full   # start with search / CDC
make generate-csv # generate seed data
make load-local   # seed local containers
make test         # run tests
make logs         # view logs
```

See `Makefile` for the full list of commands.

---

## CI / CD Summary

| Workflow           | File                       | Trigger                                    | What it does                          |
| ------------------ | -------------------------- | ------------------------------------------ | ------------------------------------- |
| Lint & Test        | `ci.yml`                   | PR → `services/**`                         | Lints and tests changed services only |
| Terraform Validate | `terraform_validation.yml` | PR → `terraform/**`                        | fmt, validate, plan                   |
| Terraform Plan     | `terraform_deploy.yml`     | Push to `main` → `terraform/**`            | `terraform plan` (apply is manual)    |
| Bruno API Tests    | `bruno.yml`                | PR → `tests/**`, `services/**`, `Makefile` | End-to-end API smoke tests            |
