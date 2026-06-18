# MrFood

Food delivery platform built with Go microservices, gRPC, and Kubernetes.

## Local Development

Requires Docker and Docker Compose.

### First-time setup

**1. Create the env file:**

```bash
make create-env
# Edit services/.env — set JWT secrets and any other required values
```

Generate a JWT secret:

```bash
openssl rand -base64 32
```

Set `AUTH_JWT_ACCESS_TOKEN_SECRET`, `AUTH_JWT_REFRESH_TOKEN_SECRET`, and the `secret` field in `services/gateway/kong/kong.yml` to the same value.

**2. Generate seed data:**

Download the Kaggle datasets and place them in `data/`:
- [places.csv + reviews.csv + users.csv](https://www.kaggle.com/datasets/danielkumlin/world-restaurants)
- [tripadvisor_european_restaurants.csv](https://www.kaggle.com/datasets/stefanoleone992/tripadvisor-european-restaurants)

```bash
python3 -m venv scripts/.venv
source scripts/.venv/bin/activate
pip install -r scripts/requirements.txt

make generate-csv             # 200 rows (fast)
make generate-csv CSV_FULL=1  # full dataset
```

All generated users have the password `mrfood123`.

**3. Start services:**

```bash
make setup       # core services only
make setup-full  # includes Elasticsearch + Kafka + CDC
```

### Commands

| Command              | What it does                              |
| -------------------- | ----------------------------------------- |
| `make setup`         | Start core services                       |
| `make setup-full`    | Start with Elasticsearch + Kafka + CDC    |
| `make build`         | Build service images                      |
| `make load-local`    | Seed local databases with CSV data        |
| `make load-cloud`    | Load CSV into Cloud SQL via GCS           |
| `make restart`       | Restart all services                      |
| `make stop`          | Stop services (keep containers)           |
| `make down`          | Stop and remove containers                |
| `make logs`          | Tail all service logs                     |
| `make test`          | Run Go unit tests                         |
| `make test-bruno`    | Run Bruno API tests                       |
| `make clean`         | Remove containers, images, and volumes    |

Run `make help` for the full list.

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md).
