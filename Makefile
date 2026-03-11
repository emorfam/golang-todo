.PHONY: run build test test-integration lint migrate migrate-down swagger

run:
	go run ./main.go

build:
	mkdir -p bin
	go build -o bin/server ./main.go

test:
	go test ./...

test-integration:
	go test -tags integration ./repository/...

lint:
	golangci-lint run

# Detect DB driver: default to sqlite3 for goose; map "postgres" to "postgres".
GOOSE_DRIVER ?= $(if $(filter postgres,$(DB_DRIVER)),postgres,sqlite3)
GOOSE_DSN    ?= $(if $(filter postgres,$(DB_DRIVER)),$(DATABASE_URL),$(or $(DATABASE_URL),./dev.db))

migrate:
	goose -dir db/migrations $(GOOSE_DRIVER) "$(GOOSE_DSN)" up

migrate-down:
	goose -dir db/migrations $(GOOSE_DRIVER) "$(GOOSE_DSN)" down

swagger:
	swag init -g main.go --output docs
