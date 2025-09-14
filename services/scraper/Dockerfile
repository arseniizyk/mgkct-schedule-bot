FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git

WORKDIR /app/services/scraper

COPY services/scraper/go.mod services/scraper/go.sum ./

COPY libs/proto ../../libs/proto

RUN go mod download

COPY services/scraper ./

RUN go build -o /app/scraper ./cmd/main.go

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /app/scraper .

RUN apk add --no-cache ca-certificates

EXPOSE 8000 9001
CMD ["./scraper"]
