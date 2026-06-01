#!/usr/bin/env bash
set -euo pipefail

CARGO="${CARGO:-cargo}"
APP="seacloud"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "unknown")}"
DIST="dist"

# Production URLs — use online defaults, allow override via env when needed.
SEACLOUD_BASE_URL="${SEACLOUD_BASE_URL:-https://cloud.seaart.ai}"
SEACLOUD_MODELS_URL="${SEACLOUD_MODELS_URL:-https://cloud-model-spec.vtrix.ai}"
SEACLOUD_GENERATION_URL="${SEACLOUD_GENERATION_URL:-$SEACLOUD_BASE_URL}"
SEACLOUD_SKILLHUB_URL="${SEACLOUD_SKILLHUB_URL:-https://skill-hub.vtrix.ai/api/v1}"
SEACLOUD_FOLKOS_PROXY_URL="${SEACLOUD_FOLKOS_PROXY_URL:-}"

# Format: os/arch/rust-target. Override TARGETS to build a subset.
DEFAULT_TARGETS=(
  "darwin/amd64/x86_64-apple-darwin"
  "darwin/arm64/aarch64-apple-darwin"
  "linux/amd64/x86_64-unknown-linux-gnu"
  "linux/arm64/aarch64-unknown-linux-gnu"
  "windows/amd64/x86_64-pc-windows-gnu"
)

if [[ -n "${TARGETS:-}" ]]; then
  read -r -a TARGET_LIST <<< "$TARGETS"
else
  TARGET_LIST=("${DEFAULT_TARGETS[@]}")
fi

rm -rf "$DIST"
mkdir -p "$DIST"

echo "Building $APP $VERSION (prod)"
echo "  BaseURL:           $SEACLOUD_BASE_URL"
echo "  ModelsBaseURL:     $SEACLOUD_MODELS_URL"
echo "  GenerationBaseURL: $SEACLOUD_GENERATION_URL"
echo "  SkillhubBaseURL:   $SEACLOUD_SKILLHUB_URL"
echo "  FolkosProxyBaseURL: ${SEACLOUD_FOLKOS_PROXY_URL:-<empty>}"
echo ""

for target in "${TARGET_LIST[@]}"; do
  IFS="/" read -r OS ARCH RUST_TARGET <<< "$target"
  BIN="$APP"
  [[ "$OS" == "windows" ]] && BIN="${APP}.exe"

  echo "  -> $OS/$ARCH ($RUST_TARGET)"
  if command -v rustup >/dev/null 2>&1; then
    rustup target add "$RUST_TARGET" >/dev/null
  fi

  SEACLOUD_BUILD_VERSION="$VERSION" \
  SEACLOUD_BASE_URL="$SEACLOUD_BASE_URL" \
  SEACLOUD_MODELS_URL="$SEACLOUD_MODELS_URL" \
  SEACLOUD_GENERATION_URL="$SEACLOUD_GENERATION_URL" \
  SEACLOUD_SKILLHUB_URL="$SEACLOUD_SKILLHUB_URL" \
  SEACLOUD_FOLKOS_PROXY_URL="$SEACLOUD_FOLKOS_PROXY_URL" \
    "$CARGO" build --release --target "$RUST_TARGET"

  OUT_DIR="$DIST/${APP}_${OS}_${ARCH}"
  mkdir -p "$OUT_DIR"
  cp "target/${RUST_TARGET}/release/${BIN}" "$OUT_DIR/$BIN"

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
