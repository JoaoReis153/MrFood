# Path to docker-compose file
COMPOSE_FILE=services/docker-compose.yml

# Build Docker Compose services
build:
	docker-compose -f $(COMPOSE_FILE) build

# Run Docker Compose services
run:
	docker-compose -f $(COMPOSE_FILE) up -d

# Stop Docker Compose services
stop:
	docker-compose -f $(COMPOSE_FILE) stop

# Remove Docker Compose services
remove:
	docker-compose -f $(COMPOSE_FILE) down

# Remove all Docker images
remove-images:
	docker rmi -f $$(docker images -q)

# Remove all containers safely
remove-all:
	@if [ "$$(docker ps -aq)" ]; then docker rm -f $$(docker ps -aq); else echo "No containers to remove"; fi
	@if [ "$$(docker images -q)" ]; then docker rmi -f $$(docker images -q); else echo "No images to remove"; fi

# Remove all containers, images, and volumes safely
remove-all-force:
	@if [ "$$(docker ps -aq)" ]; then docker rm -f $$(docker ps -aq); else echo "No containers to remove"; fi
	@if [ "$$(docker images -q)" ]; then docker rmi -f $$(docker images -q); else echo "No images to remove"; fi
	@if [ "$$(docker volume ls -q)" ]; then docker volume rm $$(docker volume ls -q); else echo "No volumes to remove"; fi

# Remove all containers, images, volumes, and networks safely
remove-all-force-all:
	@if [ "$$(docker ps -aq)" ]; then docker rm -f $$(docker ps -aq); else echo "No containers to remove"; fi
	@if [ "$$(docker images -q)" ]; then docker rmi -f $$(docker images -q); else echo "No images to remove"; fi
	@if [ "$$(docker volume ls -q)" ]; then docker volume rm $$(docker volume ls -q); else echo "No volumes to remove"; fi
	@if [ "$$(docker network ls -q -f type=custom)" ]; then docker network rm $$(docker network ls -q -f type=custom); else echo "No custom networks to remove"; fi