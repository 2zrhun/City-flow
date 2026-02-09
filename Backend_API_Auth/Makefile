.PHONY: help build run test clean migrate-up migrate-down docker-up docker-down install-deps

help:
	@echo "Available commands:"
	@echo "  make build          - Build the application"
	@echo "  make run            - Run the application"
	@echo "  make test           - Run tests"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make docker-up      - Start Docker containers"
	@echo "  make docker-down    - Stop Docker containers"
	@echo "  make migrate-up     - Run database migrations"
	@echo "  make migrate-down   - Rollback database migrations"
	@echo "  make install-deps   - Download Go dependencies"

install-deps:
	go mod tidy
	go mod download

build:
	go build -o traffic-api ./cmd/api

run:
	go run ./cmd/api

test:
	go test -v ./...

clean:
	rm -f traffic-api

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

migrate-up:
	migrate -path migrations -database "postgres://traffic_user:traffic_secure_password_123@localhost:5432/traffic_prediction_db?sslmode=disable" -verbose up

migrate-down:
	migrate -path migrations -database "postgres://traffic_user:traffic_secure_password_123@localhost:5432/traffic_prediction_db?sslmode=disable" -verbose down

install-migrate:
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

docker-build:
	docker build -t traffic-prediction-api:latest .

docker-run:
	docker run -p 8080:8080 --env-file .env traffic-prediction-api:latest