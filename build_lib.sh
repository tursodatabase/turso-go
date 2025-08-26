#!/bin/bash
# bindings/go/build_lib.sh

set -e

# Accept build type as parameter, default to release
BUILD_TYPE=${1:-release}

echo "Building turso Go library for current platform (build type: $BUILD_TYPE)..."

# Determine platform-specific details
case "$(uname -s)" in
    Darwin*)
        OUTPUT_NAME="lib_turso_go.dylib"
        # Map x86_64 to amd64 for Go compatibility
        ARCH=$(uname -m)
        if [ "$ARCH" == "x86_64" ]; then
            ARCH="amd64"
        fi
        PLATFORM="darwin_${ARCH}"
        ;;
    Linux*)
        OUTPUT_NAME="lib_turso_go.so"
        # Map x86_64 to amd64 for Go compatibility
        ARCH=$(uname -m)
        if [ "$ARCH" == "x86_64" ]; then
            ARCH="amd64"
        fi
        PLATFORM="linux_${ARCH}"
        ;;
    MINGW*|MSYS*|CYGWIN*)
        OUTPUT_NAME="lib_turso_go.dll"
        if [ "$(uname -m)" == "x86_64" ]; then
            PLATFORM="windows_amd64"
        else
            PLATFORM="windows_386"
        fi
        ;;
    *)
        echo "Unsupported platform: $(uname -s)"
        exit 1
        ;;
esac

# Create output directory
OUTPUT_DIR="libs/${PLATFORM}"
mkdir -p "$OUTPUT_DIR"

# Set cargo build arguments based on build type
if [ "$BUILD_TYPE" == "debug" ]; then
    CARGO_ARGS=""
    TARGET_DIR="debug"
    echo "NOTE: Debug builds are faster to compile but less efficient at runtime."
    echo "      For production use, consider using a release build with: ./build_lib.sh release"
else
    CARGO_ARGS="--release"
    TARGET_DIR="release"
    echo "NOTE: Release builds may take longer to compile and require more system resources."
    echo "      If this build fails or takes too long, try a debug build with: ./build_lib.sh debug"
fi

# Build the library
echo "Running cargo build ${CARGO_ARGS}"
cargo build ${CARGO_ARGS} --package turso-go

# Copy to the appropriate directory
echo "Copying $OUTPUT_NAME to $OUTPUT_DIR/"
cp "target/${TARGET_DIR}/$OUTPUT_NAME" "$OUTPUT_DIR/"

echo "Library built successfully for $PLATFORM ($BUILD_TYPE build)"
