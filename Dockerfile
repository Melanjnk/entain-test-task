FROM golang:1.26-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/server ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget
WORKDIR /app

COPY --from=builder /bin/server /app/server
COPY migrations /app/migrations

ENV HTTP_ADDR=:8080
ENV MIGRATIONS_DIR=/app/migrations

EXPOSE 8080
ENTRYPOINT ["/app/server"]
