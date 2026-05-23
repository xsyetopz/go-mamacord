#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_PATH="${1:-"$ROOT_DIR/dist/mamacord"}"

VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || git -C "$ROOT_DIR" rev-parse --short HEAD)}"
REPOSITORY="${REPOSITORY:-https://github.com/xsyetopz/go-mamacord}"
DESCRIPTION="${DESCRIPTION:-A nurturing and protective Discord app.}"
DEVELOPER_URL="${DEVELOPER_URL:-UNKNOWN}"
SUPPORT_SERVER_URL="${SUPPORT_SERVER_URL:-UNKNOWN}"
MASCOT_IMAGE_URL="${MASCOT_IMAGE_URL:-UNKNOWN}"
DESCRIPTION_BASE64="$(printf '%s' "$DESCRIPTION" | base64 | tr -d '\n')"

mkdir -p "$(dirname "$OUT_PATH")"

LDFLAGS=(
  "-s"
  "-w"
  "-X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.Version=${VERSION}'"
  "-X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.Repository=${REPOSITORY}'"
  "-X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.DescriptionBase64=${DESCRIPTION_BASE64}'"
  "-X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.DeveloperURL=${DEVELOPER_URL}'"
  "-X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.SupportServerURL=${SUPPORT_SERVER_URL}'"
  "-X 'github.com/xsyetopz/go-mamacord/internal/buildinfo.MascotImageURL=${MASCOT_IMAGE_URL}'"
)

(
  cd "$ROOT_DIR"
  CGO_ENABLED=0 go build -trimpath -ldflags="${LDFLAGS[*]}" -o "$OUT_PATH" ./cmd/mamacord
)

printf 'built %s\n' "$OUT_PATH"
