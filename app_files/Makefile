.PHONY: build_start down

# Docker build and start the local docker stack
build_start:
	docker-compose -f docker-compose.yml build --no-cache && docker-compose -f docker-compose.yml up -d --force-recreate $(c)

# Stop the local docker stack
down:
	docker-compose -f docker-compose.yml down $(c)