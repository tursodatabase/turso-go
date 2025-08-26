// Go bindings for the turso database.
//
// This file implements library embedding and extraction at runtime, a pattern
// also used in several other Go projects that need to distribute native binaries:
//
//   - github.com/kluctl/go-embed-python: Embeds a full Python distribution in Go
//     binaries, extracting to temporary directories at runtime. The approach used here
//     was directly inspired by its embed_util implementation.
//
//   - github.com/kluctl/go-jinja2: Uses the same pattern to embed Jinja2 and related
//     Python libraries, allowing Go applications to use Jinja2 templates without
//     external dependencies.
//
// This approach has several advantages:
// - Allows distribution of a single, self-contained binary
// - Eliminates the need for users to set LD_LIBRARY_PATH or other environment variables
// - Works cross-platform with the same codebase
// - Preserves backward compatibility with existing methods
// - Extracts libraries only once per execution via sync.Once
//
// The embedded library is extracted to a user-specific temporary directory and
// loaded dynamically. If extraction fails, the code falls back to the traditional
// method of searching system paths.
package turso

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

//go:embed libs/*
var embeddedLibs embed.FS

var (
	extractOnce   sync.Once
	extractedPath string
	extractErr    error
)

// extractEmbeddedLibrary extracts the library for the current platform
// to a temporary directory and returns the path to the extracted library
func extractEmbeddedLibrary() (string, error) {
	extractOnce.Do(func() {
		// Determine platform-specific details
		var libName string
		var platformDir string

		switch runtime.GOOS {
		case "darwin":
			libName = "lib_turso_go.dylib"
		case "linux":
			libName = "lib_turso_go.so"
		case "windows":
			libName = "lib_turso_go.dll"
		default:
			extractErr = fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
			return
		}

		// Determine architecture suffix
		var archSuffix string
		switch runtime.GOARCH {
		case "amd64":
			archSuffix = "amd64"
		case "arm64":
			archSuffix = "arm64"
		case "386":
			archSuffix = "386"
		default:
			extractErr = fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
			return
		}

		// Create platform directory string
		platformDir = fmt.Sprintf("%s_%s", runtime.GOOS, archSuffix)

		// Create a unique temporary directory for the current user
		tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("turso-go-%d", os.Getuid()))
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			extractErr = fmt.Errorf("failed to create temp directory: %w", err)
			return
		}

		// Path to the library within the embedded filesystem
		libPath := filepath.Join("libs", platformDir, libName)

		// Where the library will be extracted
		extractedPath = filepath.Join(tempDir, libName)

		// Check if library already exists and is valid
		if stat, err := os.Stat(extractedPath); err == nil && stat.Size() > 0 {
			// Library already exists, nothing to do
			return
		}

		// Open the embedded library
		embeddedLib, err := embeddedLibs.Open(libPath)
		if err != nil {
			extractErr = fmt.Errorf("failed to open embedded library %s: %w", libPath, err)
			return
		}
		defer embeddedLib.Close()

		// Create the output file
		outFile, err := os.Create(extractedPath)
		if err != nil {
			extractErr = fmt.Errorf("failed to create output file: %w", err)
			return
		}
		defer outFile.Close()

		// Copy the library to the temporary directory
		if _, err := io.Copy(outFile, embeddedLib); err != nil {
			extractErr = fmt.Errorf("failed to extract library: %w", err)
			return
		}

		// On Unix systems, make the library executable
		if runtime.GOOS != "windows" {
			if err := os.Chmod(extractedPath, 0755); err != nil {
				extractErr = fmt.Errorf("failed to make library executable: %w", err)
				return
			}
		}
	})

	return extractedPath, extractErr
}
