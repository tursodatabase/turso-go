# Contributing to Turso Go Driver

Thank you for your interest in contributing to the Turso Go Driver!

## Building from Source

To build with embedded library support, follow these steps:

```bash
# Clone the repository
git clone https://github.com/tursodatabase/turso-go

# Build the library (defaults to release build)
./build_lib.sh

# Alternatively, for faster builds during development:
./build_lib.sh debug
```

### Build Options:

* Release Build (default): `./build_lib.sh` or `./build_lib.sh release`

    - Optimized for performance and smaller binary size
    - Takes longer to compile and requires more system resources
    - Recommended for production use

* Debug Build: `./build_lib.sh debug`

    - Faster compilation times with less resource usage
    - Larger binary size and slower runtime performance
    - Recommended during development or if release build fails

## Manual Library Setup

If you need to set up the library manually without using the embedded library feature:

### Linux | MacOS

_All commands listed are relative to the repository's root directory._

```bash
cargo build --release

# Your LD_LIBRARY_PATH environment variable must include `target/release` directory
export LD_LIBRARY_PATH="REPO/target/release:$LD_LIBRARY_PATH"
```

### Windows

```bash
cargo build

# You must add turso's `target/release` directory to your PATH
# or you could build + copy the .dll to a location in your PATH
# or just the CWD of your go module

cp turso-go\target\release\turso_go.dll .

go test
```

## Development Setup

**Temporarily** you may have to clone the turso repository and run:

`go mod edit -replace github.com/tursodatabase/turso=/path/to/turso/bindings/go`

## Running Tests

```bash
go test ./...
```
