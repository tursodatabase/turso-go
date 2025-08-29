<p align="center">
  <img src="assets/turso.png" alt="Turso Database" width="800"/>
  <h1 align="center">Turso Database Go Driver</h1>
</p>

**NOTE:** This driver, and `turso`, are currently in Alpha and are not yet in a usable, production ready state.

This driver uses the awesome [purego](https://github.com/ebitengine/purego) library to call C (in this case Rust with C ABI) functions from Go without the use of `CGO`.

## Embedded Library Support

This driver includes an embedded library feature that allows you to distribute a single binary without requiring users to set environment variables. The library for your platform is automatically embedded, extracted at runtime, and loaded dynamically.

### Building from Source

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

* Release Build (default): ./build_lib.sh or ./build_lib.sh release

    - Optimized for performance and smaller binary size
    - Takes longer to compile and requires more system resources
    - Recommended for production use

* Debug Build: ./build_lib.sh debug

    - Faster compilation times with less resource usage
    - Larger binary size and slower runtime performance
    - Recommended during development or if release build fails

If the embedded library cannot be found or extracted, the driver will fall back to the traditional method of finding the library in the system paths.

## To use: (_UNSTABLE_ testing or development purposes only)

### Option 1: Using the embedded library (recommended)

Build the driver with the embedded library as described above, then simply import and use. No environment variables needed!



### Option 2: Manual library setup

#### Linux | MacOS

_All commands listed are relative to the repository's root directory.

```
cargo build --release

# Your LD_LIBRARY_PATH environment variable must include `target/release` directory

export LD_LIBRARY_PATH="REPO/target/release:$LD_LIBRARY_PATH"

```

#### Windows

```
cargo build

# You must add turso's `target/release` directory to your PATH
# or you could built + copy the .dll to a location in your PATH
# or just the CWD of your go module

cp turso-go\target\release\turso_go.dll .

go test


```
**Temporarily** you may have to clone the turso repository and run:

`go mod edit -replace github.com/tursodatabase/turso=/path/to/turso/bindings/go`

```go
package main

import (
	"database/sql"
	"fmt"
	"os"
	_ "github.com/tursodatabase/turso-go"
)

func main() {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	sql := "CREATE table go_turso (foo INTEGER, bar TEXT)"
	_, _ = conn.Exec(sql)

	sql = "INSERT INTO go_turso (foo, bar) values (?, ?)"
	stmt, _ := conn.Prepare(sql)
	defer stmt.Close()
	_, _ = stmt.Exec(42, "turso")
	rows, _ := conn.Query("SELECT * from go_turso")
	defer rows.Close()
	for rows.Next() {
		var a int
		var b string
		_ = rows.Scan(&a, &b)
		fmt.Printf("%d, %s", a, b)
	}
}

```

## Implementation Notes

The embedded library feature was inspired by projects like [go-embed-python](https://github.com/kluctl/go-embed-python), which uses a similar approach for embedding and distributing native libraries with Go applications.
