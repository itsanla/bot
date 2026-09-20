.PHONY: build run docker-build docker-up docker-down clean

BINARY_NAME=bin/bot

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/bot

run: build
	./$(BINARY_NAME)

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

clean:
	rm -rf bin/
