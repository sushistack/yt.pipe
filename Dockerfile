# Stage 1: Build
FROM golang:1.25 AS builder
LABEL stage=builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /yt-pipe ./cmd/yt-pipe

# Stage 2: Minimal runtime
FROM scratch

LABEL maintainer="jay"
LABEL description="SCP YouTube video pipeline automation"

# CA certificates for HTTPS API calls
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Binary
COPY --from=builder /yt-pipe /yt-pipe

# Template files for scenario pipeline
COPY --from=builder /app/templates /templates

# Non-root user
USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/yt-pipe"]
