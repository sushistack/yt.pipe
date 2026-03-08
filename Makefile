.PHONY: build test generate lint docker run clean

BINARY := bin/yt-pipe
MODULE := github.com/jay/youtube-pipeline

build:
	go build -o $(BINARY) ./cmd/yt-pipe

test:
	go test ./...

generate:
	go generate ./...

lint:
	go vet ./...

docker:
	docker build -t yt-pipe .

run:
	go run ./cmd/yt-pipe serve

clean:
	rm -rf bin/
