# ── Stage 1: Development (Air hot reload) ──────────────────────────────
FROM golang:1.24-alpine AS dev

WORKDIR /app

# Install Air for hot reload — pin to last version that supports Go <1.25
RUN apk add --no-cache git curl && \
    go install github.com/air-verse/air@v1.52.3

# Copy only go.mod first so module downloads are cached
COPY go.mod ./
# Download deps and generate go.sum (GONOSUMDB allows first-time sum population)
RUN GONOSUMDB=* GOFLAGS=-mod=mod go mod download

COPY . .

# Tidy generates a correct go.sum based on actual imports
RUN go mod tidy

EXPOSE 8080

CMD ["air", "-c", ".air.toml"]

# ── Stage 2: Builder ────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN GONOSUMDB=* GOFLAGS=-mod=mod go mod download

COPY . .
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/server ./cmd/server

# ── Stage 3: Production ─────────────────────────────────────────────────
FROM alpine:3.19 AS production

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -g '' appuser

WORKDIR /app

COPY --from=builder /bin/server /app/server

RUN chown appuser:appuser /app/server

USER appuser

EXPOSE 8080

CMD ["/app/server"]
