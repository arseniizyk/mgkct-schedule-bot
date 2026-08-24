FROM golang:1.27-alpine AS builder

RUN apk add --no-cache git ca-certificates curl

WORKDIR /app/services/scraper

COPY services/scraper/go.mod services/scraper/go.sum ./

COPY libs/ ../../libs/
COPY proto/ ../../proto

RUN go mod download

COPY services/scraper ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/scraper \
    ./cmd/scraper/main.go

FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates curl

ADD https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.56/grpc_health_probe-linux-amd64 /usr/local/bin/grpc_health_probe

RUN chmod +x /usr/local/bin/grpc_health_probe

RUN addgroup -g 1000 appuser \
 && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /app/scraper .

COPY configs /app/configs

RUN chown -R appuser:appuser /app

USER appuser

FROM runtime AS scraper
EXPOSE 9001
ENTRYPOINT ["/app/scraper"]
