# syntax=docker/dockerfile:1
# Imagen desde el código fuente. La publicada en ghcr.io la construye
# goreleaser con los binarios del release (goreleaser.Dockerfile); esta existe
# para construirla a mano y para la prueba de humo de CI.

FROM node:24-alpine AS cliente
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN mkdir -p ../internal/ui/dist && npm run build

FROM golang:1.25-alpine AS servidor
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=cliente /src/internal/ui/dist/ internal/ui/dist/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /open-tv ./cmd/open-tv

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=servidor /open-tv /open-tv
# distroless no tiene shell para mkdir/chown: /data se crea copiando un
# directorio vacío con dueño nonroot. Un volumen con nombre hereda ese dueño.
COPY --chown=65532:65532 assets/docker/data-vacio /data
ENV LISTEN_ADDR=0.0.0.0:8080 \
    DB_PATH=/data/open-tv.db
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/open-tv", "healthcheck"]
ENTRYPOINT ["/open-tv", "serve", "--no-browser"]
