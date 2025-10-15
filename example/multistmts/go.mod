module example-multistmts

go 1.23.4

require github.com/tursodatabase/turso-go v0.2.0

require (
	github.com/ebitengine/purego v0.8.3-0.20250507171810-1638563e3615 // indirect
	golang.org/x/sys v0.29.0 // indirect
)

// Use the local instead
replace github.com/tursodatabase/turso-go => ../../
