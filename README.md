# MrFood

## 📦 Setup

To run the services locally, Docker and Docker Compose are required.

Recommended first-time setup order:

1. Create env file: make create_env
2. Set JWT secrets in services/.env
3. Generate CSV data: make generate-csv
4. Start and load services: make setup

### 📂 Data Preparation

Download the data from Kaggle and place it in the `data` folder.

- Only the following files are required:
  - [places.csv](https://www.kaggle.com/datasets/danielkumlin/world-restaurants?select=places.csv)
  - [reviews.csv](https://www.kaggle.com/datasets/danielkumlin/world-restaurants?select=reviews.csv)
  - [users.csv](https://www.kaggle.com/datasets/danielkumlin/world-restaurants?select=users.csv)
  - [tripadvisor_european_restaurants.csv](https://www.kaggle.com/datasets/stefanoleone992/tripadvisor-european-restaurants)

#### 📝 Python Environment

Use the following commands to create, activate and install the Python environment:

```bash
python3 -m venv scripts/.venv
source scripts/.venv/bin/activate
pip install -r scripts/requirements.txt
```

#### 📝 Data Preparation

Generate seed CSV files using make:

```bash
make generate-csv
```

**Options:**

- Default: Generates 200 rows per service (fast, for testing)
- Full dataset: `make generate-csv CSV_FULL=1` (processes all data)
- Custom rows: `make generate-csv CSV_ROWS=1000`
- Specific service: `make generate-csv CSV_SERVICES=restaurant`

**Default Credentials:**

All generated users have the password `mrfood123`. See [SEED_DATA_CREDENTIALS.md](SEED_DATA_CREDENTIALS.md) for details.

Generated files:

- `scripts/processed_data/auth/app_user.csv`
- `scripts/processed_data/restaurant/restaurants.csv`
- `scripts/processed_data/restaurant/restaurant_working_hours.csv`
- `scripts/processed_data/restaurant/restaurant_categories.csv`
- `scripts/processed_data/review/review.csv`

### 🚀 Quick Setup (Recommended)

Start everything with one command - starts all services and loads all data:

```bash
make setup
```

Before running make setup for the first time, make sure you already ran:

```bash
make create_env
make generate-csv
```

This runs:

1. `make run` - Starts all services and databases
2. `make load-all` - Loads seed data into databases

### 🗄️ Manual Setup (Step-by-Step)

If you prefer to do it step by step:

**Step 1: Start services**

```bash
make run
```

**Step 2: Load data**

```bash
make load-all
```

If you changed database schemas and are re-running setup, reset volumes first so init SQL is reapplied:

```bash
make clean-volumes
make run
make load-all
```

Or load individual services:

```bash
make load-auth
make load-restaurant
```

`restaurant_slots` is automatically populated by the `handle_booking_insert` trigger defined in `services/booking/db_setup.sql`.

### 🔧 Environment

Create your environment file:

```bash
make create_env
```

This creates `services/.env` from the template with sensible defaults.

#### 🔐 JWT Secret

Generate JWT secrets using:

```bash
openssl rand -base64 32
```

Set these values in `services/.env`:

- `AUTH_JWT_ACCESS_TOKEN_SECRET`
- `AUTH_JWT_REFRESH_TOKEN_SECRET`

## 🚀 Running the Services

### Available Commands

View all available make commands:

```bash
make help
```

### Quick Start

Build and start everything with seed data:

```bash
make build
make setup
```

### Manual Control

Build services:

```bash
make build
```

Start services:

```bash
make run
```

View logs:

```bash
make logs
```

Stop services:

```bash
make stop
```

Restart services:

```bash
make restart
```

## 🧹 Cleanup

Stop and remove containers:

```bash
make down
```

Remove containers and images (project only):

```bash
make clean
```

Remove everything including volumes (⚠️ deletes data):

```bash
make clean-volumes
```

Full reset (containers, images, volumes):

```bash
make clean-all
```
