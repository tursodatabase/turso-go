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
package turso_go

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path"
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
			libName = "libturso_go.dylib"
		case "linux":
			libName = "libturso_go.so"
		case "windows":
			libName = "turso_go.dll"
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

		embedPath := path.Join("libs", platformDir, libName)

		// TODO: remove this debug print
		entries, _ := embeddedLibs.ReadDir(path.Join("libs", platformDir))
		for _, e := range entries {
			fmt.Println("embedded:", e.Name())
		}
		// pick a stable per-user cache dir; then use OS-specific separators
		cacheRoot, _ := os.UserCacheDir()
		if cacheRoot == "" {
			cacheRoot = os.TempDir()
		}
		destDir := filepath.Join(cacheRoot, "turso-go", platformDir)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			extractErr = fmt.Errorf("mkdir %s: %w", destDir, err)
			return
		}
		extractedPath = filepath.Join(destDir, libName)

		// reuse if already extracted
		if fi, err := os.Stat(extractedPath); err == nil && fi.Size() > 0 {
			return
		}

		// open from embed, write to disk
		in, err := embeddedLibs.Open(embedPath)
		if err != nil {
			extractErr = fmt.Errorf("open embedded %s: %w", embedPath, err)
			return
		}
		defer in.Close()

		out, err := os.Create(extractedPath)
		if err != nil {
			extractErr = fmt.Errorf("create %s: %w", extractedPath, err)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, in); err != nil {
			extractErr = fmt.Errorf("copy: %w", err)
			return
		}
		if runtime.GOOS != "windows" {
			_ = os.Chmod(extractedPath, 0o755)
		}
	})
	return extractedPath, extractErr
}
