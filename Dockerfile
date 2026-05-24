FROM golang:1.26.3-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
ARG BUILD_VERSION=dev
ARG BUILD_REPOSITORY=https://github.com/xsyetopz/go-mamacord
ARG BUILD_DESCRIPTION="A nurturing and protective Discord app."
ARG BUILD_DEVELOPER_URL=
ARG BUILD_SUPPORT_SERVER_URL=
ARG BUILD_MASCOT_IMAGE_URL=
RUN BUILD_DESCRIPTION_BASE64="$(printf '%s' "$BUILD_DESCRIPTION" | base64 | tr -d '\n')" && \
  go build -trimpath \
    -ldflags="-s -w \
      -X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.Version=${BUILD_VERSION}' \
      -X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.Repository=${BUILD_REPOSITORY}' \
      -X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.DescriptionBase64=${BUILD_DESCRIPTION_BASE64}' \
      -X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.DeveloperURL=${BUILD_DEVELOPER_URL}' \
      -X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.SupportServerURL=${BUILD_SUPPORT_SERVER_URL}' \
      -X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.MascotImageURL=${BUILD_MASCOT_IMAGE_URL}'" \
    -o /out/mamacord ./cmd/mamacord


FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
  && rm -rf /var/lib/apt/lists/*

ARG UID=1000
ARG GID=1000
RUN groupadd -g "${GID}" mamacord && useradd -u "${UID}" -g "${GID}" -m -s /usr/sbin/nologin mamacord

WORKDIR /app

COPY --from=builder /out/mamacord /usr/local/bin/mamacord
COPY migrations ./migrations
COPY locales ./locales
COPY plugins ./plugins
COPY config ./config

RUN mkdir -p /data/plugins /data/marketplace_cache /data/bundles/store /data/bundles/cache && chown -R mamacord:mamacord /data

USER mamacord:mamacord

ENV MAMACORD_STORAGE_BACKEND=postgres
ENV MAMACORD_POSTGRES_DSN=postgres://mamacord:secret@postgres:5432/mamacord?sslmode=disable
ENV LOCALES_DIR=/app/locales
ENV MAMACORD_BUNDLED_PLUGINS_DIR=/app/plugins
ENV MAMACORD_USER_PLUGINS_DIR=/data/plugins
ENV MAMACORD_MARKETPLACE_CACHE_DIR=/data/marketplace_cache
ENV MAMACORD_BUNDLE_STORE_DIR=/data/bundles/store
ENV MAMACORD_BUNDLE_CACHE_DIR=/data/bundles/cache
ENV MAMACORD_PERMISSIONS_FILE=/app/config/permissions.json

ENTRYPOINT ["mamacord"]
