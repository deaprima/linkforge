# Satu Dockerfile untuk semua service Go. Pilih service lewat build arg:
#   docker build -f services/Dockerfile --build-arg SERVICE=gateway -t linkforge/gateway services
# SERVICE = gateway | auth | link | analytics
# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
ARG SERVICE
WORKDIR /src
# go.mod dulu: layer dependency tidak dibangun ulang kalau hanya kode yang berubah
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO mati = binary statis; -trimpath/-s -w = tanpa path lokal dan symbol debug
RUN test -n "$SERVICE" || (echo "build arg SERVICE is required" && exit 1) \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service ./${SERVICE}/cmd/server

FROM alpine:3.21
# ca-certificates: SMTP/TLS dan Google OAuth; tzdata: zona waktu; wget (busybox) dipakai HEALTHCHECK
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 app
COPY --from=build /out/service /usr/local/bin/service
USER app
ENTRYPOINT ["service"]
