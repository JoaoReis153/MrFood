# Config
PROJECT_NAME := mrfood
COMPOSE_FILE := services/docker-compose.yml
TEST_PACKAGES := ./services/auth/... ./services/restaurant/... ./services/test_grpc/...

DC := docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE)

.PHONY: test build run stop down logs restart clean clean-volumes clean-all

## Run all Go tests
test:
	go test -v -race $(TEST_PACKAGES)

## Build services
build:
	$(DC) build

## Start services (detached)
run:
	$(DC) up -d

## Stop services
stop:
	$(DC) stop

## Stop and remove services
down:
	$(DC) down

## View logs
logs:
	$(DC) logs -f

## Restart services
restart: down run

## Remove only this project's containers + images
clean:
	$(DC) down --rmi local --remove-orphans

clean-volumes:
## Remove containers + images + volumes ( deletes DB data)
	$(DC) down --rmi local --volumes --remove-orphans

## Full reset
clean-all:
	$(DC) down --rmi all --volumes --remove-orphans