FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum .
COPY cmd/control ./cmd/control
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /control ./cmd/control

# Not distroless: it needs ssh (and ssh-keygen for the first start).
FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends openssh-client && rm -rf /var/lib/apt/lists/*
COPY --from=build /control /control
EXPOSE 8093
ENTRYPOINT ["/control"]
