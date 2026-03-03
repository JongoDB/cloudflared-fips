package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed static
var staticFS embed.FS

// EmbeddedStaticHandler returns an http.Handler that serves the embedded
// dashboard frontend assets with SPA fallback. For client-side routes
// (like /fleet, /node) that don't correspond to actual files, it serves
// index.html so React Router can handle the routing.
func EmbeddedStaticHandler(devDir string) http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return spaHandler(http.Dir(devDir))
	}

	// Check if embedded FS has content (at least index.html)
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		// Embedded FS is empty — use filesystem fallback
		return spaHandler(http.Dir(devDir))
	}

	return spaHandler(http.FS(sub))
}

// spaHandler wraps an http.FileSystem to provide SPA (Single Page Application)
// fallback. If the requested path is not a real file and has no file extension,
// index.html is served instead so the client-side router can handle it.
func spaHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		p := path.Clean(r.URL.Path)
		if p == "" {
			p = "/"
		}

		// Try to open the file
		f, err := fsys.Open(p)
		if err != nil {
			// File doesn't exist — check if this is a client-side route
			// (no file extension) vs a missing asset (has extension)
			if !hasFileExtension(p) {
				// SPA fallback: serve index.html for client-side routes
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			// Missing asset — let FileServer return 404
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		// File exists — serve it normally
		fileServer.ServeHTTP(w, r)
	})
}

// hasFileExtension checks if a path has a file extension (e.g., .js, .css, .png).
func hasFileExtension(p string) bool {
	base := path.Base(p)
	return strings.Contains(base, ".")
}
