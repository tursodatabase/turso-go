<p align="center">
  <img src="assets/turso.png" alt="Turso Database" width="800"/>
  <h1 align="center">Turso Database Go Driver</h1>
</p>

> **⚠️Warning:** This software is in BETA. It may still contain bugs and unexpected behavior. Use caution with production data and ensure you have backups.

This driver uses the awesome [purego](https://github.com/ebitengine/purego) library to call C (in this case Rust with C ABI) functions from Go without the use of `CGO`.

## Getting Started

Install the Turso Go driver using `go get`:

```bash
go get github.com/tursodatabase/turso-go
```

Then import it in your Go code:

```go
import (
    "database/sql"
    _ "github.com/tursodatabase/turso-go"
)
```

The driver includes an embedded library that is automatically extracted and loaded at runtime, so no additional setup or environment variables are required.

### Example Usage

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

## Embedded Library Support

This driver includes an embedded library feature that allows you to distribute a single binary without requiring users to set environment variables. The library for your platform is automatically embedded, extracted at runtime, and loaded dynamically.

If the embedded library cannot be found or extracted, the driver will fall back to the traditional method of finding the library in system paths.

The embedded library feature was inspired by projects like [go-embed-python](https://github.com/kluctl/go-embed-python), which uses a similar approach for embedding and distributing native libraries with Go applications.

## Contributing

For information on building from source, manual library setup, and development instructions, see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

This project is licensed under the [MIT license].

### Contribution

Unless you explicitly state otherwise, any contribution intentionally submitted
for inclusion in Turso Database by you, shall be licensed as MIT, without any additional
terms or conditions.

[MIT license]: LICENSE.md
