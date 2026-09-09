.PHONY: run build test tidy compose-up compose-down

run:
	go run ./cmd/server

build:
	go build -o bin/cabinet-server.exe ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down
