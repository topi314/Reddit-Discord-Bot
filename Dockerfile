FROM golang:1.16.2-alpine AS build

WORKDIR /tmp/app

COPY . .

RUN apk add --no-cache git && \
    go mod download && \
    go mod verify && \
    go build -o reddit-discord-bot

FROM alpine:latest

WORKDIR /home/reddit-discord-bot

COPY --from=build /tmp/app/reddit-discord-bot /home/reddit-discord-bot/

EXPOSE 80

ENTRYPOINT ./reddit-discord-bot