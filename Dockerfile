# Stage 1: Build
FROM golang:1.24-alpine AS builder

LABEL org.opencontainers.image.title="MCP Zero Trust Proxy"
LABEL org.opencontainers.image.version="1.0.0"
LABEL org.opencontainers.image.vendor="keith-aykira"

RUN apk add --no-cache git ca-certificates tzdata
ENV GOTRACEBACK=crash

WORKDIR /app

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code  
COPY . .

# Build arguments
ARG VERSION=dev
ARG BUILD_DATE=2026-01-01T00:00:00Z
ARG VCS_REF=unknown

# Production build with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static' -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.vcsRef=${VCS_REF}" \
    -o /mcpproxy \
    ./cmd/mcpproxy

# Stage 2: Runtime
FROM alpine:3.19

# Add labels
LABEL org.opencontainers.image.title="MCP Zero Trust Proxy"
LABEL org.opencontainers.image.version="1.0.0"
LABEL org.opencontainers.image.vendor="keith-aykira"

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /
COPY configs/example.yaml /etc/mcpproxy/config.yaml

COPY --from=builder /mcpproxy /usr/local/bin/mcpproxy

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

USER nobody

ENTRYPOINT ["/usr/local/bin/mcpproxy"]
CMD ["--config", "/etc/mcpproxy/config.yaml"]
