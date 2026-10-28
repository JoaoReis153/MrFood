# MrFood

Restaurant discovery and booking platform — Go microservices, gRPC, Kong gateway, Kubernetes on GCP.

**Services:** auth · restaurant · booking · review · payment · sponsor · notification · search  
**Infrastructure:** Keycloak (auth), Elasticsearch + Kafka + CDC (search), Redis (notifications), PostgreSQL per service  
**Observability:** OpenTelemetry → Grafana (Prometheus + Loki + Tempo)

---

## Local Development

Requires Docker and Docker Compose. Optional: Python 3 (seed data), Node.js (`make test-bruno`) and Go (`make test`).

### 1. Configure secrets

```bash
make create-env
```

This creates `services/.env` from `services/env.tmpl`, pre-filled with local-development defaults, so no edits are needed to run locally. `SMTP_*` and `STRIPE_*` are empty: set them only to test email or payments.

`APP_JWT_ACCESS_TOKEN_SECRET` must match the `secret` in `services/gateway/kong/kong.yml`, and `KEYCLOAK_CLIENT_SECRET` must match `services/keycloak/realm-import.json`. The defaults already do. To use your own JWT secret, generate one with `openssl rand -base64 32` and put it in **both** `services/.env` and `kong.yml`.

> `scripts/deploy.sh` also reads `services/.env`. Replace the defaults with real secrets before deploying to GCP.

### 2. Start services

```bash
make setup       # core services (auth, restaurant, booking, review, payment, sponsor, notification, gateway)
make setup-full  # + Elasticsearch, Kafka, CDC
```

API is available at **`http://localhost:8000`** once running.

Check that it is up:

```bash
curl http://localhost:8000/auth/ping/1
```

Then register and log in (endpoints are described in [`docs/mrfoodapi.yaml`](docs/mrfoodapi.yaml)):

```bash
curl -X POST http://localhost:8000/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo","email":"demo@example.com","password":"StrongPassword123"}'

curl -X POST http://localhost:8000/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"StrongPassword123"}'
```

Other endpoints need the returned access token as `Authorization: Bearer <token>`.

Other local ports: Keycloak admin console `8080`, Grafana `3000`, Prometheus `9090`, Elasticsearch `9200` (with `setup-full`).

### 3. Seed data (optional)

The app runs without seed data — databases start empty. To populate with realistic restaurant data:

```bash
# Download the Kaggle datasets into data/:
#   https://www.kaggle.com/datasets/danielkumlin/world-restaurants
#   https://www.kaggle.com/datasets/stefanoleone992/tripadvisor-european-restaurants

python3 -m venv scripts/.venv
source scripts/.venv/bin/activate
pip install -r scripts/requirements.txt

make generate-csv             # 200 rows (fast)
make generate-csv CSV_FULL=1  # full dataset

make load-local               # import into running containers
```

All generated users have the password `mrfood123`.

---

### Commands

| Command           | What it does                                |
| ----------------- | ------------------------------------------- |
| `make setup`      | Start core services                         |
| `make setup-full` | Start with Elasticsearch + Kafka + CDC      |
| `make build`      | Build service images                        |
| `make restart`    | Restart all services                        |
| `make stop`       | Stop services (keep containers)             |
| `make down`       | Stop and remove containers                  |
| `make logs`       | Tail all service logs                       |
| `make load-local` | Seed local databases with CSV data          |
| `make test`       | Run Go unit tests                           |
| `make test-bruno` | Run Bruno API tests against `localhost:8000` |
| `make clean`      | Remove containers, images, and volumes      |

Run `make help` for the full list including cloud and stress-test commands.

---

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md).
