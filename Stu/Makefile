GO ?= go
DEV_COMPOSE ?= deploy/docker-compose.dev.yml
PROD_COMPOSE ?= deploy/docker-compose.prod.yml

.PHONY: all build test lint fmt dev prod up down tidy tools

all: build

build:
	$(GO) build ./...

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

lint:
	@if ! command -v golangci-lint > /dev/null; then echo "golangci-lint не найден, установите: https://golangci-lint.run/usage/install/"; exit 1; fi
	golangci-lint run ./...

tidy:
	$(GO) mod tidy

dev:
	docker compose -f $(DEV_COMPOSE) up --build

prod:
	docker compose -f $(PROD_COMPOSE) up -d --build

down:
	docker compose -f $(DEV_COMPOSE) down
