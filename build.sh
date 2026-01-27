#!/usr/bin/env sh
set -e

APP_NAME="elf2vkp-go"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
OUT_DIR="dist"

echo "Building $APP_NAME version $VERSION"

mkdir -p "$OUT_DIR"

build() {
  GOOS=$1
  GOARCH=$2
  EXT=$3

  OUT="$OUT_DIR/${APP_NAME}-${VERSION}-${GOOS}-${GOARCH}${EXT}"
  echo " -> $OUT"

  GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 \
    go build -ldflags="-s -w -X main.version=$VERSION" -o "$OUT"
}

# Linux
build linux amd64 ""
build linux arm64 ""

# Windows
build windows amd64 ".exe"

# macOS
build darwin amd64 ""
build darwin arm64 ""

echo "Done. Binaries are in ./$OUT_DIR"
