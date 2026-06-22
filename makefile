.PHONY: run build dev clean test lint tidy \
        migrate-up migrate-down migrate-create

SVC ?= user

run:
	go run ./services/$(SVC)/cmd/api/main.go

build:
	@mkdir -p bin
	go build -o ./bin/$(SVC) ./services/$(SVC)/cmd/api/main.go

dev:
	air -c services/$(SVC)/.air.toml

clean:
	rm -rf bin tmp

test:
	go test ./...

lint:
	golangci-lint run ./...

tidy:
	cd pkg && go mod tidy
	cd services/auth && go mod tidy
	cd services/user && go mod tidy

migrate-up:
	./scripts/migrate.sh $(SVC) up

migrate-down:
	./scripts/migrate.sh $(SVC) down

migrate-create:
	migrate create -ext sql -dir database/migrations/$(SVC) -seq $(name)
