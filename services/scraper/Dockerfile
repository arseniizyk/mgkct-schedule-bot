FROM golang:1.25-alpine AS builder

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

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/migrate \
    ./cmd/migrate/main.go

FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates curl

RUN addgroup -g 1000 appuser \
 && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /app/scraper .
COPY --from=builder /app/migrate .

COPY configs /app/configs
COPY migrations /app/migrations

RUN chown -R appuser:appuser /app
RUN chmod +x /app/migrate

USER appuser

FROM runtime AS scraper
EXPOSE 8000 9001
ENTRYPOINT ["/app/scraper"]

FROM runtime AS migrate 
ENTRYPOINT [ "/app/migrate" ]
