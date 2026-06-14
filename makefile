.PHONY: run build dev clean test lint tidy \
        migrate-up migrate-down migrate-create

BIN := ./bin/api
SVC ?= user

run:
	go run ./services/$(SVC)/cmd/api/main.go

build:
	@rm -rf $(BIN)
	@mkdir -p bin
	go build -o $(BIN) ./services/$(SVC)/cmd/api/main.go

dev:
	air

clean:
	rm -rf bin tmp

test:
	go test ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

migrate-up:
	./scripts/migrate.sh $(SVC) up

migrate-down:
	./scripts/migrate.sh $(SVC) down

migrate-create:
	migrate create -ext sql -dir database/migrations/$(SVC) -seq $(name)
