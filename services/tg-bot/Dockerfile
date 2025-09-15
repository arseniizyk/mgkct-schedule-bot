FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app/services/tg-bot

COPY services/tg-bot/go.mod services/tg-bot/go.sum ./

COPY libs/proto ../../libs/proto

RUN go mod download

COPY services/tg-bot ./

RUN go build -o /app/tg-bot ./cmd/main.go

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /app/tg-bot .

# RUN echo "https://mirror.yandex.ru/mirrors/alpine/v3.22/main/" > /etc/apk/repositories && \
#     echo "https://mirror.yandex.ru/mirrors/alpine/v3.22/community/" >> /etc/apk/repositories

RUN apk update && apk add --no-cache ca-certificates

CMD ["./tg-bot"]
