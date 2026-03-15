.PHONY: build test test-e2e test-integration generate lint docker docker-up docker-down docker-logs run clean

BINARY := bin/yt-pipe
MODULE := github.com/sushistack/yt.pipe
IMAGE  := yt-pipe
TAG    := latest

build:
	go build -o $(BINARY) ./cmd/yt-pipe

test:
	go test ./...

test-e2e:
	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
	go test -tags=e2e -timeout 300s ./tests/e2e/...

test-integration:
	go test -tags=integration -timeout 600s ./...

generate:
	go generate ./...

lint:
	go vet ./...

docker:
	docker build -t $(IMAGE):$(TAG) .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

run:
	go run ./cmd/yt-pipe serve

clean:
	rm -rf bin/
