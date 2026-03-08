# Stage 1: Build
FROM golang:latest AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /yt-pipe ./cmd/yt-pipe

# Stage 2: Minimal runtime
FROM scratch
COPY --from=builder /yt-pipe /yt-pipe
ENTRYPOINT ["/yt-pipe"]
