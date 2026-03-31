# MrFood

## 📦 Setup

To run the services locally, Docker and Docker Compose are required.

### 🐳 Docker

Install Docker on your machine following the instructions [here](https://docs.docker.com/get-docker/).

### 🐳 Docker Compose

Install Docker Compose on your machine following the instructions [here](https://docs.docker.com/compose/install/).

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
- Booking cap override: `make generate-csv CSV_MAX_BOOKINGS=500000`

Notes:

- Booking generation is bounded by default on large datasets to avoid out-of-memory runs.
- Use `CSV_MAX_BOOKINGS` if you need more or fewer booking rows.

Generated files:

- `scripts/processed_data/auth/app_user.csv`
- `scripts/processed_data/restaurant/restaurants.csv`
- `scripts/processed_data/restaurant/restaurant_working_hours.csv`
- `scripts/processed_data/restaurant/restaurant_categories.csv`
- `scripts/processed_data/booking/booking.csv`

### 🚀 Quick Setup (Recommended)

Start everything with one command - starts all services and loads all data:

```bash
make setup
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

Or load individual services:

```bash
make load-auth
make load-restaurant
make load-booking
```

`restaurant_slots` is automatically populated by the `handle_booking_insert` trigger defined in `services/booking/db_setup.sql`.

### 🔧 Environment

Create your environment file:

```bash
cp services/env.tmpl services/.env
```

Update the configuration inside /services/.env as needed.

#### 🔐 JWT Secret

Generate a JWT secret using:

```bash
openssl rand -base64 32
```

Add it to your `/services/.env.`

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
