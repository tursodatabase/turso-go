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

# Detect libc variant on Linux
LIBC_VARIANT=""
if [[ "$UNAME_S" == Linux* ]]; then
  # Check if we're on musl-based system (Alpine)
  if ldd --version 2>&1 | grep -qi musl; then
    LIBC_VARIANT="_musl"
  elif [ -f /etc/alpine-release ]; then
    # Fallback detection for Alpine Linux
    LIBC_VARIANT="_musl"
  fi
fi

case "$UNAME_S" in
  Linux*)  OS=linux  ;;
  Darwin*) OS=darwin ;;
  MINGW*|MSYS*|CYGWIN*) OS=windows ;;
  *) echo "Unsupported OS: $UNAME_S"; exit 1 ;;
esac
PLATFORM="${OS}${LIBC_VARIANT}_${ARCH}"

case "$OS" in
  linux)
    if [[ "$LIBC_VARIANT" == "_musl" ]]; then
      OUTPUT_NAME="lib${LIB_BASENAME}.a"
    else
      OUTPUT_NAME="lib${LIB_BASENAME}.so"
    fi
    ;;
  darwin)  OUTPUT_NAME="lib${LIB_BASENAME}.dylib" ;;
  windows) OUTPUT_NAME="${LIB_BASENAME}.dll" ;;
esac

# Set Rust target for musl builds
RUST_TARGET=""
if [[ "$LIBC_VARIANT" == "_musl" ]]; then
  case "$ARCH" in
    amd64) RUST_TARGET="x86_64-unknown-linux-musl" ;;
    arm64) RUST_TARGET="aarch64-unknown-linux-musl" ;;
    *) echo "Unsupported musl arch: $ARCH"; exit 1 ;;
  esac
  # Check if the target is installed
  if ! rustup target list --installed | grep -q "$RUST_TARGET"; then
    echo "Installing Rust target: $RUST_TARGET"
    rustup target add "$RUST_TARGET"
  fi
  CARGO_ARGS+=("--target" "$RUST_TARGET")
fi

echo "Building ${CRATE_PACKAGE} ($BUILD_TYPE) for ${PLATFORM}…"
cargo build "${CARGO_ARGS[@]}" --package "${CRATE_PACKAGE}"

# Determine output directory (changes when using --target)
if [[ -n "$RUST_TARGET" ]]; then
  OUT_DIR="target/${RUST_TARGET}/${TARGET_DIR}"
else
  OUT_DIR="target/${TARGET_DIR}"
fi
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
