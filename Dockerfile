# syntax=docker/dockerfile:1
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/app ./cmd/app
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/web ./cmd/web

# Runtime container
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /app/bin/app /usr/local/bin/app
COPY --from=builder /app/bin/web /usr/local/bin/web

EXPOSE 8080 3001

ENTRYPOINT ["/usr/local/bin/app", "server"]
