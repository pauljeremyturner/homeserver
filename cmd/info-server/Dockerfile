FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum .
COPY internal ./internal
COPY gen ./gen
COPY cmd/info-server ./cmd/info-server
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /info-server ./cmd/info-server

FROM gcr.io/distroless/static-debian12
COPY --from=build /info-server /info-server
EXPOSE 9090
ENTRYPOINT ["/info-server"]
