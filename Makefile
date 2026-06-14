.PHONY: build run test docker-up docker-down clean

build:
	go build -o bin/shortlink ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -v

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

clean:
	rm -rf bin/
