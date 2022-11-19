FROM golang:1.18-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o app .

FROM alpine

WORKDIR /app

COPY --from=build /build/app /app/bin

EXPOSE 80

ENTRYPOINT ["/app/bin"]