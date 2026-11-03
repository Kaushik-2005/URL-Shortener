# syntax=docker/dockerfile:1

FROM golang:1.26.6-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/server \
    ./cmd/server

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/server /server

USER 65532:65532

ENV HTTP_ADDR=0.0.0.0:8080

EXPOSE 8080

ENTRYPOINT ["/server"]