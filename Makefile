# Config
PROJECT_NAME := mrfood
COMPOSE_FILE := services/docker-compose.yml
TEST_PACKAGES := ./services/auth/... ./services/booking/... ./services/restaurant/... ./services/review/... ./services/sponsor/...

# Load environment variables from .env file
-include services/.env

DC := docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE)
PYTHON := $(if $(wildcard scripts/.venv/bin/python),scripts/.venv/bin/python,python3)
CSV_SERVICES ?= all
CSV_ROWS ?= 200
CSV_FULL ?=

.PHONY: help create_env generate-csv generate-csv-auth generate-csv-restaurant generate-csv-review load-auth load-restaurant load-reviews load-all setup build run stop down restart logs test clean clean-volumes clean-all test test-bruno

help:
	@echo "MrFood Make Commands"
	@echo ""
	@echo "Setup & Data:"
	@echo "  make create_env                         - Create services/.env from template"
	@echo "  make setup                              - Start services and load all data"
	@echo "  make generate-csv                       - Generate CSV seed data (default 200 rows)"
	@echo "  make generate-csv CSV_FULL=1            - Generate CSV seed data (full dataset)"
	@echo "  make load-reviews                       - Load review seed data into database"
	@echo "  make load-all                           - Load all seed data into databases"
	@echo ""
	@echo "Service Management:"
	@echo "  make run                                - Start all services (detached)"
	@echo "  make stop                               - Stop services"
	@echo "  make down                               - Stop and remove services"
	@echo "  make restart                            - Restart services"
	@echo "  make logs                               - View service logs"
	@echo ""
	@echo "Build & Test:"
	@echo "  make build                              - Build service images"
	@echo "  make test                               - Run all Go tests"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean                              - Remove containers & images"
	@echo "  make clean-volumes                      - Remove containers, images, volumes"
	@echo "  make clean-all                          - Full reset (all containers, images, volumes)"

## Run Bruno REST CI smoke tests
test-bruno:
	mkdir -p tests/mrfood-api/reports
	cd tests/mrfood-api/collections/ci && npx --yes @usebruno/cli@latest run -r --tests-only --reporter-junit ../../reports/bruno-junit.xml --reporter-json ../../reports/bruno-report.json

## Build services

# ============================================================================
# ENVIRONMENT
# ============================================================================

## Create services/.env from services/env.tmpl
create_env:
	@if [ -f services/.env ]; then \
		echo "services/.env already exists. No changes made."; \
	else \
		cp services/env.tmpl services/.env; \
		echo "Created services/.env from services/env.tmpl"; \
		echo "Fill AUTH_JWT_ACCESS_TOKEN_SECRET and AUTH_JWT_REFRESH_TOKEN_SECRET in services/.env"; \
	fi

# ============================================================================
# DATA GENERATION
# ============================================================================

## Generate CSV seed data (default 200 rows, use CSV_FULL=1 for full dataset)
generate-csv:
	$(PYTHON) scripts/process_data.py --services $(CSV_SERVICES) $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,)

## Generate only auth CSV seed files
generate-csv-auth:
	$(PYTHON) scripts/process_data.py --services auth $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,)

## Generate only restaurant CSV seed files
generate-csv-restaurant:
	$(PYTHON) scripts/process_data.py --services restaurant $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,)

## Generate only review CSV seed files
generate-csv-review:
	$(PYTHON) scripts/process_data.py --services review $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,)

# ============================================================================
# DATA LOADING
# ============================================================================

## Load auth data into database
load-auth:
	$(DC) exec -T auth_db psql -U "$(AUTH_POSTGRES_USER)" -d "$(AUTH_POSTGRES_DB)" -c "TRUNCATE TABLE app_user CASCADE;"
	$(DC) exec -T auth_db psql -U "$(AUTH_POSTGRES_USER)" -d "$(AUTH_POSTGRES_DB)" -c "\\copy app_user(user_id, username, password, email) FROM STDIN WITH (FORMAT csv, HEADER true)" < scripts/processed_data/auth/app_user.csv

## Load restaurant data into database
load-restaurant:
	$(DC) exec -T restaurant_db psql -U "$(RESTAURANT_POSTGRES_USER)" -d "$(RESTAURANT_POSTGRES_DB)" -c "TRUNCATE TABLE restaurant_categories, restaurants RESTART IDENTITY CASCADE;"
	$(DC) exec -T restaurant_db psql -U "$(RESTAURANT_POSTGRES_USER)" -d "$(RESTAURANT_POSTGRES_DB)" -c "\\copy restaurants(id, name, latitude, longitude, address, opening_time, closing_time, media_url, max_slots, owner_id, owner_name, sponsor_tier) FROM STDIN WITH (FORMAT csv, HEADER true)" < scripts/processed_data/restaurant/restaurants.csv
	$(DC) exec -T restaurant_db psql -U "$(RESTAURANT_POSTGRES_USER)" -d "$(RESTAURANT_POSTGRES_DB)" -c "\\copy restaurant_categories(restaurant_id, category) FROM STDIN WITH (FORMAT csv, HEADER true)" < scripts/processed_data/restaurant/restaurant_categories.csv

## Load all seed data into databases

load-reviews:
	@if ! $(DC) ps --services | grep -qx "review_db"; then \
		echo "Skipping load-reviews: review_db service is not configured in $(COMPOSE_FILE)"; \
	elif ! $(DC) ps --services --status running | grep -qx "review_db"; then \
		echo "Skipping load-reviews: review_db service is not running"; \
	else \
		$(DC) exec -T review_db psql -U "$(REVIEW_POSTGRES_USER)" -d "$(REVIEW_POSTGRES_DB)" -c "TRUNCATE TABLE review, restaurant_stats RESTART IDENTITY CASCADE;"; \
		$(DC) exec -T review_db psql -U "$(REVIEW_POSTGRES_USER)" -d "$(REVIEW_POSTGRES_DB)" -c "\\copy review(review_id, restaurant_id, user_id, comment, rating, created_at) FROM STDIN WITH (FORMAT csv, HEADER true)" < scripts/processed_data/review/review.csv; \
		$(DC) exec -T review_db psql -U "$(REVIEW_POSTGRES_USER)" -d "$(REVIEW_POSTGRES_DB)" -c "SELECT setval(pg_get_serial_sequence('review', 'review_id'), COALESCE((SELECT MAX(review_id) FROM review), 1), (SELECT COUNT(*) > 0 FROM review));"; \
	fi

load-all:
	@$(MAKE) --no-print-directory -j 3 load-auth load-restaurant load-reviews 
	@echo "✓ All data loaded successfully"

## Complete setup: start services and load all data
setup: run load-all
	@echo "✓ Setup complete! Services running and data loaded"

# ============================================================================
# SERVICE MANAGEMENT
# ============================================================================

## Build service images
build:
	$(DC) build

build-no-cache:
	$(DC) build --no-cache

## Start all services (detached)
run:
	$(DC) up -d

## Stop services
stop:
	$(DC) stop

## Stop and remove services
down:
	$(DC) down

down-volumes:
	$(DC) down -v

## Restart services
restart: down run

## View service logs
logs:
	$(DC) logs -f

# ============================================================================
# TESTING & BUILDING
# ============================================================================

## Run all Go tests
test:
	go test -v -race $(TEST_PACKAGES)

# ============================================================================
# CLEANUP
# ============================================================================

## Remove only this project's containers + images
clean:
	$(DC) down --rmi local --remove-orphans

## Remove containers + images + volumes (deletes DB data)
clean-volumes:
	$(DC) down --rmi local --volumes --remove-orphans

## Full reset (containers, images, volumes)
clean-all:
	$(DC) down --rmi all --volumes --remove-orphans