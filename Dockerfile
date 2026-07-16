# ── Stage 1: Builder ─────────────────────────────────────────────────────────
# Uses the full Go toolchain to compile a statically-linked binary.
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Download dependencies first — cached unless go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build.
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-s -w -extldflags '-static'" \
      -trimpath \
      -o /app/bin/api \
      ./cmd/api

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
# Minimal image — no shell, no package manager, no root user.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy the compiled binary.
COPY --from=builder /app/bin/api /app/api

# Copy migrations so the app can run them at startup.
COPY --from=builder /app/migrations /app/migrations

# The app listens on this port.
EXPOSE 8080

# Run as the built-in nonroot user (uid 65532).
USER nonroot:nonroot

ENTRYPOINT ["/app/api"]
