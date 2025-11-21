---
agent: vibe
name: 2025-11-20-vibe-turso-go
language: go
context: [
    ./lib.go,
    sh:(go doc -all sql)
]
vibe-out: ./driver.go
---

Generate Go SDK for the Turso - SQLite-compatible embedded database written in Rust.
You must use lib.go bridge from Go to C API.

The SDK should has following components under the hood:
* `type tursoDbConnection struct { }` - wrapper which holds connection to the turso and protect it against concurrent use
* `type tursoDbStatement struct { }` - wrapper which holds prepared statement to the turso and protect it against concurrent use
* `type tursoDbRows struct { }` - wrapper which holds prepared statement and provide sqld/database compatible methods to iterate over rows of the statement
* `type tursoDbDriver struct { }` - type to register in the sql/database as driver
* `type tursoDbResult struct { }` - type implementing `driver.Result` interface
* `type tursoDbTx struct { }` - type implementing `driver.Tx` interface

Be aware, that turso connection and statement are not suitable for concurrent use 
(in Rust, these objects are only safe to Send between threads). 
So, any concurrent use must be prevented and guarded in the Go SDK.

* The package name is "turso_go"
* The generated SDK must be integrated with the native "database/sql" Go module from standard library.
  - Register driver under the name "tursodb"
  - Inspect native docs if needed
* Use only functions available in lib.go to interact with the database
* Define main structs at the top of the bindings file
* Document API methods:
  * What this method do and in which cases it must be called
  * Whare are the implicit requirements and how to use this methods properly (e.g. another method must be called before)
* Document all structs extensively:
  * Describe, what this struct do and how it must be used (how to create, how to reclaim resources)
  * Describe implicit requirements
* Always handle errors and propagate them up
* Turso has only one isolation level (snapshot isolation). 