# Stage 1: Build
FROM golang:1.25 AS builder
LABEL stage=builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /yt-pipe ./cmd/yt-pipe

# Stage 2: Runtime with FFmpeg
FROM alpine:3.21

LABEL maintainer="jay"
LABEL description="SCP YouTube video pipeline automation"

# FFmpeg for direct video rendering, CA certs for HTTPS, timezone data
RUN apk add --no-cache ffmpeg ca-certificates tzdata

# Non-root user
RUN adduser -D -u 65534 appuser

# Binary
COPY --from=builder /yt-pipe /yt-pipe

# Template files for scenario pipeline
COPY --from=builder /app/templates /templates

USER appuser

EXPOSE 8080

ENTRYPOINT ["/yt-pipe"]
