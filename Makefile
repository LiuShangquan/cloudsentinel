.PHONY: fmt vet test check build run-api run-worker docker-build compose-config compose-up compose-down compose-logs compose-ps clean

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

check: fmt vet test
	go mod verify

build:
	go build -o bin/cloudsentinel-api ./cmd/api
	go build -o bin/cloudsentinel-worker ./cmd/worker

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

docker-build:
	docker compose build

compose-config:
	docker compose config

compose-up:
	docker compose up -d

compose-down:
	docker compose down

compose-logs:
	docker compose logs --tail=200

compose-ps:
	docker compose ps

clean:
	go clean
	@if exist bin rmdir /s /q bin
