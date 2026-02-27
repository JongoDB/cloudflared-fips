package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// EmbeddedStaticHandler returns an http.Handler that serves the embedded
// dashboard frontend assets. Falls back to the provided directory if the
// embedded filesystem is empty (dev mode).
func EmbeddedStaticHandler(devDir string) http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// Fallback to filesystem for development
		return http.FileServer(http.Dir(devDir))
	}

	// Check if embedded FS has content (at least index.html)
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		// Embedded FS is empty — use filesystem fallback
		return http.FileServer(http.Dir(devDir))
	}

	return http.FileServer(http.FS(sub))
}
