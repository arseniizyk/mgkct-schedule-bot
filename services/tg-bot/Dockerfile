FROM golang:1.27-alpine AS builder

RUN apk add --no-cache git ca-certificates curl

WORKDIR /app/services/tg-bot

COPY services/tg-bot/go.mod services/tg-bot/go.sum ./

COPY libs/ ../../libs/
COPY proto/ ../../proto

RUN go mod download

COPY services/tg-bot ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/tg-bot \
    ./cmd/tg-bot/main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/migrate \
    ./cmd/migrate/main.go

FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates curl

RUN addgroup -g 1000 appuser \
 && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /app/tg-bot .
COPY --from=builder /app/migrate .

COPY configs /app/configs
COPY migrations /app/migrations

RUN chown -R appuser:appuser /app
RUN chmod +x /app/migrate

USER appuser

FROM runtime AS tg-bot
EXPOSE 8000 8081
ENTRYPOINT ["/app/tg-bot"]

FROM runtime AS migrate 
ENTRYPOINT [ "/app/migrate" ]
