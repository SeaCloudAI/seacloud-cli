#!/usr/bin/env bash
set -euo pipefail

GO="${GO:-$(which go 2>/dev/null || echo /opt/homebrew/bin/go)}"

APP="seacloud"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "unknown")}"
DIST="dist"

# Production URLs — use online defaults, allow override via env when needed.
SEACLOUD_BASE_URL="${SEACLOUD_BASE_URL:-https://vtrix.ai}"
SEACLOUD_MODELS_URL="${SEACLOUD_MODELS_URL:-https://cloud-model-spec.vtrix.ai}"
SEACLOUD_GENERATION_URL="${SEACLOUD_GENERATION_URL:-$SEACLOUD_BASE_URL}"
SEACLOUD_SKILLHUB_URL="${SEACLOUD_SKILLHUB_URL:-https://skill-hub.vtrix.ai/api/v1}"
SEACLOUD_FOLKOS_PROXY_URL="${SEACLOUD_FOLKOS_PROXY_URL:-http://folkos-gateway.dev.folkos.ai/folkos-proxy}"
SEACLOUD_PROXY_URL="${SEACLOUD_PROXY_URL:-}"

LDFLAGS="-s -w \
  -X github.com/SeaCloudAI/seacloud-cli/internal/buildinfo.Version=${VERSION} \
  -X github.com/SeaCloudAI/seacloud-cli/internal/auth.BaseURL=${SEACLOUD_BASE_URL} \
  -X github.com/SeaCloudAI/seacloud-cli/internal/models.BaseURL=${SEACLOUD_MODELS_URL} \
  -X github.com/SeaCloudAI/seacloud-cli/internal/generation.BaseURL=${SEACLOUD_GENERATION_URL} \
  -X github.com/SeaCloudAI/seacloud-cli/internal/images.BaseURL=${SEACLOUD_PROXY_URL} \
  -X github.com/SeaCloudAI/seacloud-cli/internal/skillhub.BaseURL=${SEACLOUD_SKILLHUB_URL} \
  -X github.com/SeaCloudAI/seacloud-cli/internal/config.DefaultFolkosProxyBaseURL=${SEACLOUD_FOLKOS_PROXY_URL}"

TARGETS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

rm -rf "$DIST"
mkdir -p "$DIST"

echo "Building $APP $VERSION (prod)"
echo "  BaseURL:          $SEACLOUD_BASE_URL"
echo "  ModelsBaseURL:    $SEACLOUD_MODELS_URL"
echo "  GenerationBaseURL: $SEACLOUD_GENERATION_URL"
echo "  ProxyBaseURL:     $SEACLOUD_PROXY_URL"
echo "  SkillhubBaseURL:  $SEACLOUD_SKILLHUB_URL"
echo "  FolkosProxyBaseURL: $SEACLOUD_FOLKOS_PROXY_URL"
echo ""

for target in "${TARGETS[@]}"; do
  OS="${target%/*}"
  ARCH="${target#*/}"

  BIN="$APP"
  [[ "$OS" == "windows" ]] && BIN="${APP}.exe"

  OUT_DIR="$DIST/${APP}_${OS}_${ARCH}"
  mkdir -p "$OUT_DIR"

  echo "  -> $OS/$ARCH"
  GOOS="$OS" GOARCH="$ARCH" CGO_ENABLED=0 "$GO" build \
    -ldflags="${LDFLAGS}" \
    -o "$OUT_DIR/$BIN" .

  if [[ "$OS" == "windows" ]]; then
    (cd "$DIST" && zip -q "${APP}_${OS}_${ARCH}.zip" "${APP}_${OS}_${ARCH}/${BIN}")
  else
    tar -czf "$DIST/${APP}_${OS}_${ARCH}.tar.gz" -C "$DIST" "${APP}_${OS}_${ARCH}"
  fi

  rm -rf "$OUT_DIR"
done

echo ""
echo "Artifacts in ./$DIST/:"
ls -lh "$DIST/"
