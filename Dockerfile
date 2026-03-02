# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /build

COPY ssp/go.mod ./
RUN go mod download 2>/dev/null || true

COPY ssp/ ./
RUN CGO_ENABLED=0 go build -o ssp .

# Run stage
FROM alpine:3.19
WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /build/ssp .
COPY ssp/publisher.html .

ENV PORT=4000
EXPOSE 4000

CMD ["./ssp"]
