# Multi-stage build для создания легковесного контейнера (<30 МБ)
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Кэширование зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Сборка статического бинарника без CGO (pure Go SQLite)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/bot ./cmd/bot

# Финальный легковесный образ
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Создаем директорию для БД
RUN mkdir -p /app/data

COPY --from=builder /app/bin/bot /app/bot

ENV ENV=prod \
    BOT_HEALTH_PORT=8081 \
    BOT_DB_PATH=/app/data/bot.db

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8081/healthz || exit 1

VOLUME ["/app/data"]

ENTRYPOINT ["/app/bot"]
