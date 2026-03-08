.PHONY: build test generate lint docker docker-up docker-down docker-logs run clean

BINARY := bin/yt-pipe
MODULE := github.com/jay/youtube-pipeline
IMAGE  := yt-pipe
TAG    := latest

build:
	go build -o $(BINARY) ./cmd/yt-pipe

test:
	go test ./...

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
