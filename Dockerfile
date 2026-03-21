# Stage 1: Build
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /mcpproxy \
    ./cmd/mcpproxy

# Stage 2: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /mcpproxy /usr/local/bin/mcpproxy
COPY configs/example.yaml /etc/mcpproxy/config.yaml
EXPOSE 8080
ENTRYPOINT ["mcpproxy"]
CMD ["--config", "/etc/mcpproxy/config.yaml"]
