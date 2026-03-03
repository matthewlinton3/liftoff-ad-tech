# Build stage
FROM golang:1.25 AS builder
WORKDIR /build

COPY go.mod ./
RUN go mod download 2>/dev/null || true

COPY . ./
RUN CGO_ENABLED=0 go build -o exchange .

# Run stage
FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/exchange .

EXPOSE 3000

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
	CMD sh -c 'wget -q -O /dev/null "http://127.0.0.1:${PORT:-3000}/health" || exit 1'

CMD ["./exchange"]
