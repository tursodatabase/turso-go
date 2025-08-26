#!/usr/bin/env bash
set -euo pipefail

BUILD_TYPE="${1:-release}"
CRATE_PACKAGE="${CRATE_PACKAGE:-turso-go}"
LIB_BASENAME="${LIB_BASENAME:-turso_go}"
GO_LIB_DIR="${GO_LIB_DIR:-libs}"

if [[ "$BUILD_TYPE" == "release" ]]; then
  CARGO_ARGS=(--release)
  TARGET_DIR="release"
else
  CARGO_ARGS=()
  TARGET_DIR="debug"
fi

UNAME_S="$(uname -s)"
UNAME_M="$(uname -m)"

case "$UNAME_M" in
  x86_64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  i386|i686) ARCH=386 ;;
  *) echo "Unsupported arch: $UNAME_M"; exit 1 ;;
esac
case "$UNAME_S" in
  Linux*)  OS=linux  ;;
  Darwin*) OS=darwin ;;
  MINGW*|MSYS*|CYGWIN*) OS=windows ;;
  *) echo "Unsupported OS: $UNAME_S"; exit 1 ;;
esac
PLATFORM="${OS}_${ARCH}"

case "$OS" in
  linux)   OUTPUT_NAME="lib_${LIB_BASENAME}.so" ;;
  darwin)  OUTPUT_NAME="lib_${LIB_BASENAME}.dylib" ;;
  windows) OUTPUT_NAME="_${LIB_BASENAME}.dll" ;;
esac

echo "Building ${CRATE_PACKAGE} ($BUILD_TYPE) for ${PLATFORM}…"
cargo build "${CARGO_ARGS[@]}" --package "${CRATE_PACKAGE}"

OUT_DIR="target/${TARGET_DIR}"
ART="${OUT_DIR}/${OUTPUT_NAME}"

if [[ ! -f "$ART" ]]; then
  echo "Expected artifact not found: $ART"
  echo "Contents of ${OUT_DIR}:"
  ls -la "${OUT_DIR}" || true
  exit 1
fi

DEST="${GO_LIB_DIR}/${PLATFORM}"
mkdir -p "${DEST}"
cp -f "${ART}" "${DEST}/"

echo "Wrote ${DEST}/$(basename "$ART")"
