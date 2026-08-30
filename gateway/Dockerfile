# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

# Allow Go to download the exact toolchain declared in go.mod (go 1.26.6).
# Without this, GOTOOLCHAIN=local would refuse to run a newer go directive.
ENV GOTOOLCHAIN=auto

WORKDIR /build

# Download dependencies first so this layer is cached between code changes.
COPY go.mod go.sum ./
RUN go mod download

# Copy source (the Lua script is embedded at compile time via //go:embed).
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o sentinel ./cmd/gateway

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM alpine:3.20

# Run as a non-root user.
RUN adduser -D -u 1000 sentinel
USER sentinel

WORKDIR /app
COPY --from=builder /build/sentinel .

EXPOSE 8080
ENTRYPOINT ["./sentinel"]
