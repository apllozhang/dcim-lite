.PHONY: run build test test-integration tidy compose-up compose-down

run:
	go run ./cmd/server

build:
	go build -o bin/cabinet-server.exe ./cmd/server

test:
	go test ./...

# 集成测试需要真实 PostgreSQL：
#   TEST_DATABASE_URL=postgres://it:it@localhost:5432/it?sslmode=disable make test-integration
test-integration:
	go test -tags integration ./internal/integration/ -v -count=1

tidy:
	go mod tidy

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down
