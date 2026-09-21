FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod .
COPY internal ./internal
COPY cmd/nowplaying ./cmd/nowplaying
RUN CGO_ENABLED=0 go build -o /nowplaying ./cmd/nowplaying

FROM gcr.io/distroless/static-debian12
COPY --from=build /nowplaying /nowplaying
EXPOSE 8090
ENTRYPOINT ["/nowplaying"]
