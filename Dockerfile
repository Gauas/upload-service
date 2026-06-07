FROM golang:1.25-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o http-service ./main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o consumer-service ./consumer/main.go

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=builder /build/http-service .
COPY --from=builder /build/consumer-service .
COPY entrypoint.sh .

RUN chmod +x entrypoint.sh

USER app

EXPOSE 8080 9090

ENTRYPOINT ["./entrypoint.sh"]
