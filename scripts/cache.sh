#!/usr/bin/env bash
set -euo pipefail

GOOS="$(go env GOOS 2>/dev/null || echo unknown)"
GOARCH="$(go env GOARCH 2>/dev/null || echo unknown)"

if [[ -n "${TURSO_GO_CACHE_DIR:-}" ]]; then
  CACHE_ROOT="${TURSO_GO_CACHE_DIR}"
else
  case "$GOOS" in
    darwin)  CACHE_ROOT="${HOME}/Library/Caches" ;;
    linux)   CACHE_ROOT="${XDG_CACHE_HOME:-${HOME}/.cache}" ;;
    windows)
      CACHE_ROOT="${LOCALAPPDATA:-${APPDATA:-}}"
      if command -v cygpath >/dev/null 2>&1 && [[ -n "${CACHE_ROOT}" ]]; then
        CACHE_ROOT="$(cygpath -u "$CACHE_ROOT")"
      fi
      [[ -n "${CACHE_ROOT}" ]] || CACHE_ROOT="${TMP:-/tmp}"
      ;;
    *)       CACHE_ROOT="${TMPDIR:-${TMP:-/tmp}}" ;;
  esac
fi

PLATFORM="${GOOS}_${GOARCH}"
BASE_DIR="${CACHE_ROOT}/turso-go"
DEST_DIR="${BASE_DIR}/${PLATFORM}"

case "$GOOS" in
  linux)   LIB="libturso_go.so" ;;
  darwin)  LIB="libturso_go.dylib" ;;
  windows) LIB="turso_go.dll" ;;
  *)       echo "Unsupported GOOS=$GOOS"; exit 2 ;;
esac

print_paths() {
  echo "cache_dir: ${DEST_DIR}"
  echo "lib_path : ${DEST_DIR}/${LIB}"
}

clean_cache() {
  echo "removing: ${DEST_DIR}"
  rm -rf -- "${DEST_DIR}"
}

usage() {
  cat <<EOF
Usage: $(basename "$0") <print|clean>
Environment (optional):
  TURSO_GO_CACHE_DIR   override cache root
EOF
}

case "${1:-}" in
  print) print_paths ;;
  clean) clean_cache ;;
  *)     usage; exit 1 ;;
esac
