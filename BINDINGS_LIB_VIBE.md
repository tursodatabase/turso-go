---
agent: vibe
name: 2025-11-20-vibe-turso-go-lib
language: go
context: [
    https://raw.githubusercontent.com/tursodatabase/turso/refs/heads/main/sqlite3/src/lib.rs,
    https://raw.githubusercontent.com/ebitengine/purego/refs/heads/main/func.go::RegisterFunc#comment,
    https://raw.githubusercontent.com/ebitengine/purego/refs/heads/main/func.go::RegisterLibFunc#comment,
    ./turso_unix.go,
    ./turso_windows.go,
    file:./embedded.go::extractEmbeddedLibrary#comment
]
vibe-out: ./lib.go
---

Generate Go functions bindings for the Turso - SQLite-compatible embedded database written in Rust.
You must use C API bridge implemented in Rust which expose ABI compatible functions to interact with the database.

## Important details (must be followed)

Naming:
* The package name is "turso_go"
* Use `sqlite3_.*` or `libsql_.*` naming convention for function names
  - Foc C functions, prepend `c_` prefix to the variable name
* Use example same name for functions as in Rust bindings (with `c_` prefix for purego functions). Do not add anything extra

Structure:
* Introduce necessary constants which are used in the public API of the bindings (return codes, parameters, default strings, etc)
* Define necessary constants (error codes, strings) at the top of the bindings file
* Define main structs at the top of the bindings file
* Wrap bindings in go functions which make them more ergonomic:
  * Return error as last output parameter instead of int32 error code
  * Return value (if any) instead of out parameter
* Do not expose any additional methods / split Rust methods in parts - just translate methods in 1:1 relation to Go
* All library methods **must** be registered on it as this is slow operation and we must not do that after startup

Bindings gotchas:
* Remember, that purego already do marshaling of string parameters (by converting them to zero-terminated string and keep alive parameters)
  * Inspect docstrings for purego functions to get more details

Recommendations:
* Load the library with the `loadLibrary` function (already implemented - you can freely use it)
  * Panic immediately if you can't load the library or functions due to whatever reason
  * Initialization must either succeed or panic
* Use `RegisterLibFunc` to register foreign function (note, that it returns no error!)
* Add explicit `registerBindings` function which must ensure that all necessary functions are registered in Go
* Follow same order of definition as sqlite3 bindings in Rust
* Introduce few type aliases for extra type safety (convert them manually to pointers in the method implementations if needed):
  * `TursoDb` - transparent wrapper on top of db pointer
  * `TursoStatement` - composite type which holds statement and db pointers (db pointer can be used to extract proper error)
  * `TursoStep` enumeration for step result
  * Never return or accept row `unsafe.Pointer` in the go wrappers - always either use some custom or native type (e.g. []byte, string, etc)
* Do not handle any callbacks (even if Rust has them). Validate that callbacks are nils and return MISUSE if they are not
* Be careful with `sqlite3_free` usage as it free pointer with libc function (so if it were some internal Rust object - this will be wrong). 
  * In case when you can't properly free resources with current FFI layer - you **must** leave TODO comment (e.g. // todo(agent): ...)
* Add links to the external resources if applicable or cite purego/tursodb docs in non-trivial places
* Introduce special error type `type TursoError struct { .. }` which will hold code, extended and message
  * Return `error` interface which will hold either `nil` or `&TursoError` as error result everywhere where it can be done
* Always return step result from `sqlite3_step() (TursoStep, error)` function but in appropriate cases set non-nil error too

Code style:
* Do not add unnecessary docstrings which just duplicate obvious things (like function name, enumerate args, etc)
* Do not add unnecessary helper methods / lambdas if their code can be easily inlined without any downsides
* Avoid unnecessary allocations
* Avoid usage of global variables if possible
* Always check error first and later return value + nil if there were no error

Dependencies:
* Use "github.com/ebitengine/purego" - do not use CGo
* Do not use any other external dependencies - only purego and Go native stdlib

