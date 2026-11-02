# Backend tasks. Go 1.27.3 is expected in PATH (e.g. export PATH=$HOME/.local/go/bin:$PATH).
GO            ?= go
TEST_PG_PORT  ?= 55432
TEST_DATABASE_URL ?= postgres://postgres:postgres@127.0.0.1:$(TEST_PG_PORT)/opensex_test?sslmode=disable

.PHONY: build test test-unit test-integration test-db test-db-stop lint vuln cover run-server run-worker migrate-up migrate-down keys docker

build:
	$(GO) build ./...

test-unit:
	$(GO) test ./pkg/... ./internal/...

test: test-db
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' $(GO) test -race ./...

# Starts a disposable PostgreSQL for the integration tests.
test-db:
	@docker inspect opensex-pg-test >/dev/null 2>&1 || docker run -d --name opensex-pg-test \
		-e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=opensex_test -p 127.0.0.1:$(TEST_PG_PORT):5432 postgres:17-alpine >/dev/null
	@until docker exec opensex-pg-test pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done

test-integration: test-db
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' $(GO) test -race ./test/integration/ ./internal/database/

test-db-stop:
	docker rm -f opensex-pg-test

cover: test-db
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' $(GO) test -coverpkg=./internal/...,./pkg/... -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

lint:
	$(GO) vet ./...
	golangci-lint run ./...

vuln:
	govulncheck ./...

run-server:
	$(GO) run ./cmd/server

run-worker:
	$(GO) run ./cmd/worker

migrate-up:
	$(GO) run ./cmd/ops migrate up

migrate-down:
	$(GO) run ./cmd/ops migrate down 1

keys:
	$(GO) run ./cmd/ops keys generate --dir $${JWT_KEYS_DIR:-./tmp/jwt} --kid $${KID:-$$(date +%Y-%m)}

docker:
	docker build -t opensex/backend:dev .
