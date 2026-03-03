# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /build

COPY ssp/go.mod ./
RUN go mod download 2>/dev/null || true

COPY ssp/ ./
RUN CGO_ENABLED=0 go build -o ssp .

# Run stage
FROM alpine:3.19
WORKDIR /app

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /build/ssp .
COPY ssp/publisher.html .

EXPOSE 4000

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
	CMD sh -c 'wget -q -O /dev/null "http://127.0.0.1:${PORT:-4000}/health" || exit 1'

CMD ["./ssp"]
