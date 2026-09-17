# syntax=docker/dockerfile:1
# La usa goreleaser (dockers_v2): el binario ya viene compilado por plataforma
# en $TARGETPLATFORM/open-tv, con el cliente web embebido.
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/open-tv /open-tv
COPY --chown=65532:65532 assets/docker/data-vacio /data
ENV LISTEN_ADDR=0.0.0.0:8080 \
    DB_PATH=/data/open-tv.db
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/open-tv", "healthcheck"]
ENTRYPOINT ["/open-tv", "serve", "--no-browser"]
