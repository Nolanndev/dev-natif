IMAGE ?= dev-natif-api:latest

.PHONY: build run up down logs tidy test docker-build docker-run clean

## Local Go (requires Go toolchain)
tidy:
	go mod tidy

build:
	CGO_ENABLED=0 go build -o bin/api ./cmd/api

run: build
	./bin/api

test:
	go test ./...

## Container workflow (no local Go required)
docker-build:
	docker build -t $(IMAGE) .

docker-run: docker-build
	docker run --rm -p 8080:8080 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v natif-data:/data \
		$(IMAGE)

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f api

clean:
	rm -rf bin out *.db
