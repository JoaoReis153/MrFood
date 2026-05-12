# MrFood

Food delivery platform built with Go microservices, gRPC, and Kubernetes.

## Local Development

Requires Docker and Docker Compose.

### First-time setup

**1. Create the env file and fill in secrets:**

```bash
make create_env
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

Set up the Python environment and generate CSVs:

```bash
python3 -m venv scripts/.venv
source scripts/.venv/bin/activate
pip install -r scripts/requirements.txt

make generate-csv           # 200 rows (fast)
make generate-csv CSV_FULL=1  # full dataset
```

All generated users have the password `mrfood123`.

**3. Start services:**

```bash
make setup        # core services only
make setup-full   # includes Elasticsearch + Kafka + CDC
```

### Common commands

```bash
make logs         # tail logs
make stop         # stop services
make down         # stop and remove containers
make restart      # restart services
make test         # run Go tests
make test-bruno   # run Bruno API tests
make clean        # remove containers, images, volumes
```

See `make help` for the full list.

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for GKE deployment instructions.
