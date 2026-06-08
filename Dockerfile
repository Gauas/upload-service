FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /build/bin
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /build/bin/http-service ./cmd/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /build/bin/consumer-service ./consumer

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache bash ca-certificates

COPY --from=builder /build/bin/http-service .
COPY --from=builder /build/bin/consumer-service .
COPY entrypoint.sh .

RUN chmod +x entrypoint.sh

EXPOSE 8080 9090

ENTRYPOINT ["./entrypoint.sh"]