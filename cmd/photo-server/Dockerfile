FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum .
COPY internal ./internal
COPY gen ./gen
COPY cmd/photo-server ./cmd/photo-server
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /photo-server ./cmd/photo-server

FROM gcr.io/distroless/static-debian12
COPY --from=build /photo-server /photo-server
EXPOSE 9092 8092
ENTRYPOINT ["/photo-server"]
