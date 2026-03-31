# Config
PROJECT_NAME := mrfood
COMPOSE_FILE := services/docker-compose.yml
TEST_PACKAGES := ./services/auth/... ./services/restaurant/...

# Load environment variables from .env file
-include services/.env

DC := docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE)
PYTHON := $(if $(wildcard scripts/.venv/bin/python),scripts/.venv/bin/python,python3)
CSV_SERVICES ?= all
CSV_ROWS ?= 200
CSV_FULL ?=
CSV_MAX_BOOKINGS ?=

.PHONY: help create_env generate-csv generate-csv-auth generate-csv-restaurant generate-csv-booking load-auth load-restaurant load-booking load-all setup build run stop down restart logs test clean clean-volumes clean-all

help:
	@echo "MrFood Make Commands"
	@echo ""
	@echo "Setup & Data:"
	@echo "  make create_env                         - Create services/.env from template"
	@echo "  make setup                              - Start services and load all data"
	@echo "  make generate-csv                       - Generate CSV seed data (default 200 rows)"
	@echo "  make generate-csv CSV_FULL=1            - Generate CSV seed data (full dataset)"
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
	$(PYTHON) scripts/process_data.py --services $(CSV_SERVICES) $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,) $(if $(CSV_MAX_BOOKINGS),--max-bookings $(CSV_MAX_BOOKINGS),)

## Generate only auth CSV seed files
generate-csv-auth:
	$(PYTHON) scripts/process_data.py --services auth $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,) $(if $(CSV_MAX_BOOKINGS),--max-bookings $(CSV_MAX_BOOKINGS),)

## Generate only restaurant CSV seed files
generate-csv-restaurant:
	$(PYTHON) scripts/process_data.py --services restaurant $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,) $(if $(CSV_MAX_BOOKINGS),--max-bookings $(CSV_MAX_BOOKINGS),)

## Generate only booking CSV seed files
generate-csv-booking:
	$(PYTHON) scripts/process_data.py --services booking $(if $(CSV_ROWS),--rows $(CSV_ROWS),) $(if $(CSV_FULL),--full,) $(if $(CSV_MAX_BOOKINGS),--max-bookings $(CSV_MAX_BOOKINGS),)

# ============================================================================
# DATA LOADING
# ============================================================================

## Load auth data into database
load-auth:
	$(DC) exec -T auth_db psql -U "$(AUTH_POSTGRES_USER)" -d "mrfood_auth" -c "TRUNCATE TABLE app_user RESTART IDENTITY CASCADE;"
	$(DC) exec -T auth_db psql -U "$(AUTH_POSTGRES_USER)" -d "mrfood_auth" -c "\\copy app_user(user_id, username, password, email) FROM STDIN WITH (FORMAT csv, HEADER true)" < scripts/processed_data/auth/app_user.csv
	$(DC) exec -T auth_db psql -U "$(AUTH_POSTGRES_USER)" -d "mrfood_auth" -c "SELECT setval(pg_get_serial_sequence('app_user', 'user_id'), COALESCE((SELECT MAX(user_id) FROM app_user), 1), (SELECT COUNT(*) > 0 FROM app_user));"

## Load restaurant data into database
load-restaurant:
	$(DC) exec -T restaurant_db psql -U "$(RESTAURANT_POSTGRES_USER)" -d "$(RESTAURANT_POSTGRES_DB)" -c "TRUNCATE TABLE restaurant_categories, restaurant_working_hours, restaurants RESTART IDENTITY CASCADE;"
	$(DC) exec -T restaurant_db psql -U "$(RESTAURANT_POSTGRES_USER)" -d "$(RESTAURANT_POSTGRES_DB)" -c "\\copy restaurants(id, name, latitude, longitude, address, media_url, max_slots, owner_id, owner_name, sponsor_tier) FROM STDIN WITH (FORMAT csv, HEADER true)" < scripts/processed_data/restaurant/restaurants.csv
	$(DC) exec -T restaurant_db psql -U "$(RESTAURANT_POSTGRES_USER)" -d "$(RESTAURANT_POSTGRES_DB)" -c "\\copy restaurant_working_hours(restaurant_id, time_start, time_end) FROM STDIN WITH (FORMAT csv, HEADER true)" < scripts/processed_data/restaurant/restaurant_working_hours.csv
	$(DC) exec -T restaurant_db psql -U "$(RESTAURANT_POSTGRES_USER)" -d "$(RESTAURANT_POSTGRES_DB)" -c "\\copy restaurant_categories(restaurant_id, category) FROM STDIN WITH (FORMAT csv, HEADER true)" < scripts/processed_data/restaurant/restaurant_categories.csv
	$(DC) exec -T restaurant_db psql -U "$(RESTAURANT_POSTGRES_USER)" -d "$(RESTAURANT_POSTGRES_DB)" -c "SELECT setval(pg_get_serial_sequence('restaurants', 'id'), COALESCE((SELECT MAX(id) FROM restaurants), 1), (SELECT COUNT(*) > 0 FROM restaurants));"

## Load booking data into database
load-booking:
	$(DC) exec -T booking_db psql -U "$(BOOKING_POSTGRES_USER)" -d "$(BOOKING_POSTGRES_DB)" -c "TRUNCATE TABLE booking, restaurant_slots RESTART IDENTITY CASCADE;"
	$(DC) exec -T booking_db psql -U "$(BOOKING_POSTGRES_USER)" -d "$(BOOKING_POSTGRES_DB)" -c "\\copy booking(id, user_id, restaurant_id, time_start, time_end, people_count) FROM STDIN WITH (FORMAT csv, HEADER true)" < scripts/processed_data/booking/booking.csv
	$(DC) exec -T booking_db psql -U "$(BOOKING_POSTGRES_USER)" -d "$(BOOKING_POSTGRES_DB)" -c "SELECT setval(pg_get_serial_sequence('booking', 'id'), COALESCE((SELECT MAX(id) FROM booking), 1), (SELECT COUNT(*) > 0 FROM booking));"

## Load all seed data into databases

load-all:
	@$(MAKE) --no-print-directory -j 3 load-auth load-restaurant load-booking
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

## Start all services (detached)
run:
	$(DC) up -d

## Stop services
stop:
	$(DC) stop

## Stop and remove services
down:
	$(DC) down

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