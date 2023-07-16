FROM --platform=$BUILDPLATFORM golang:1.20-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG COMMIT

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -ldflags="-X 'main.Version=$VERSION' -X 'main.Commit=$COMMIT'" -o reddit-discord-bot github.com/topi314/reddit-discord-bot/v2

FROM alpine

COPY --from=build /build/reddit-discord-bot /bin/reddit-discord-bot

EXPOSE 80

ENTRYPOINT ["/bin/reddit-discord-bot"]

CMD ["-config", "/var/lib/reddit-discord-bot/config.yml", "-database.sqlite.path", "/var/lib/reddit-discord-bot/database.db"]
