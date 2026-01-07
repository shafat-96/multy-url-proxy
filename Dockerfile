# ---------- Build stage ----------
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git (required for GOPROXY=direct)
RUN apk add --no-cache git ca-certificates

# Fix Go module TLS / proxy issues
ENV GOPROXY=direct
ENV GOSUMDB=off

# Copy go mod files first (better caching)
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o proxy-server .

# ---------- Runtime stage ----------
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/proxy-server .

EXPOSE 3000

CMD ["./proxy-server"]
