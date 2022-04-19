FROM golang:1.18-alpine AS build

WORKDIR /tmp/app

COPY .. .

RUN apk add --no-cache git && \
    go mod download && \
    go mod verify && \
    go build -o reddit-discord-bot

FROM alpine:latest

WORKDIR /app

COPY --from=build /tmp/app/reddit-discord-bot /app/reddit-discord-bot

EXPOSE 80

ENTRYPOINT ./reddit-discord-bot