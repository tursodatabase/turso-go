#!/bin/bash
# bindings/go/build_lib.sh

set -e

# Accept build type as parameter, default to release
BUILD_TYPE=${1:-release}

echo "Building turso Go library for current platform (build type: $BUILD_TYPE)..."

UNAME_S=$(uname -s)
UNAME_M=$(uname -m)

case "$UNAME_M" in
  x86_64)          ARCH=amd64 ;;
  arm64|aarch64)   ARCH=arm64 ;;
  i386|i686)       ARCH=386   ;;
  *) echo "Unsupported arch: $UNAME_M"; exit 1 ;;
esac

case "$UNAME_S" in
  Darwin*)
    OUTPUT_NAME="libturso_go.dylib"
    PLATFORM="darwin_${ARCH}"
    ;;
  Linux*)
    OUTPUT_NAME="libturso_go.so"
    PLATFORM="linux_${ARCH}"
    ;;
  MINGW*|MSYS*|CYGWIN*)
    OUTPUT_NAME="turso_go.dll"
    if [ "$ARCH" = "amd64" ]; then
      PLATFORM="windows_amd64"
    else
      PLATFORM="windows_386"
    fi
    ;;
  *)
    echo "Unsupported platform: $UNAME_S"
    exit 1
    ;;
esac

OUTPUT_DIR="libs/${PLATFORM}"
mkdir -p "$OUTPUT_DIR"

# Build the library
cargo build ${CARGO_ARGS} --package turso-go

# Verify expected artifact exists
ART="target/${TARGET_DIR}/${OUTPUT_NAME}"
if [ ! -f "$ART" ]; then
  echo "Expected artifact not found: $ART"
  echo "Available files in target/${TARGET_DIR}:"
  find "target/${TARGET_DIR}" -maxdepth 1 -type f -printf "%f\n" 2>/dev/null || ls -la "target/${TARGET_DIR}"
  exit 1
fi

mkdir -p "libs/${PLATFORM}"
cp "$ART" "libs/${PLATFORM}/"

echo "Library built successfully for $PLATFORM ($BUILD_TYPE build)"
