# Stage 1: Build the binary
FROM golang:alpine AS builder

WORKDIR /build

# Install certificates and git
RUN apk add --no-cache ca-certificates git tzdata

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary without CGO
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bin/bot ./cmd/bot

# Stage 2: Minimal runtime image
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (TLS root certs + timezone data)
RUN apk add --no-cache ca-certificates tzdata && \
    mkdir -p /app/data

# Set default timezone to WIB (Asia/Jakarta)
ENV TZ=Asia/Jakarta

# Copy compiled binary from builder
COPY --from=builder /build/bin/bot /app/bot

# Expose HTTP status port
EXPOSE 5005

ENTRYPOINT ["/app/bot"]
