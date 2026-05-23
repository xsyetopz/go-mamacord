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
RUN go build -trimpath \
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

RUN mkdir -p /data && chown -R mamacord:mamacord /data

USER mamacord:mamacord

ENV SQLITE_PATH=/data/mamacord.sqlite
ENV MIGRATIONS_DIR=/app/migrations/sqlite
ENV LOCALES_DIR=/app/locales
ENV PLUGINS_DIR=/app/plugins
ENV MAMACORD_PERMISSIONS_FILE=/app/config/permissions.json

ENTRYPOINT ["mamacord"]
