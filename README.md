# MrFood

Food delivery platform — Go microservices, gRPC, Kong gateway, Kubernetes on GCP.

**Services:** auth · restaurant · booking · review · payment · sponsor · notification · search  
**Infrastructure:** Keycloak (auth), Elasticsearch + Kafka + CDC (search), Redis (notifications), PostgreSQL per service  
**Observability:** OpenTelemetry → Grafana (Prometheus + Loki + Tempo)

---

## Local Development

Requires Docker and Docker Compose.

### 1. Configure secrets

```bash
make create-env
# Edit services/.env — fill in passwords and the JWT secret
```

For the JWT secret, generate one or use the hardcoded dev value already in `services/gateway/kong/kong.yml`:

```bash
openssl rand -base64 32
```

Set `APP_JWT_ACCESS_TOKEN_SECRET` and `APP_JWT_REFRESH_TOKEN_SECRET` in `services/.env` to the same value as `secret` in `services/gateway/kong/kong.yml`.

### 2. Start services

```bash
make setup       # core services (auth, restaurant, booking, review, payment, sponsor, notification, gateway)
make setup-full  # + Elasticsearch, Kafka, CDC
```

API is available at **`http://localhost:8000`** once running.

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
