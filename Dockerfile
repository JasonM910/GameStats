FROM golang:1.26.8-alpine3.24 AS build

WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /gamestats .

FROM scratch

WORKDIR /app
COPY --from=build /gamestats /app/gamestats
COPY eventos.json /app/eventos.json

ENTRYPOINT ["/app/gamestats"]
