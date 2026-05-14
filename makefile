run:
	go run cmd/api/main.go

build:
	mkdir -p bin
	go build -o bin/api cmd/api/main.go

clean:
	rm -rf bin

# Migrations
DB_URL=postgres://postgres:postgres@localhost:5433/rest_db?sslmode=disable

migrate-up:
	migrate -path db/migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path db/migrations -database "$(DB_URL)" down

migrate-force:
	migrate -path db/migrations -database "$(DB_URL)" force $(version)