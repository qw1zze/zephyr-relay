.PHONY: run build test docker-up docker-down tidy

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -o ./bin/relay ./cmd/server

test:
	go test ./...

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

tidy:
	go mod tidy
