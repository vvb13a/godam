package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:frontend/dist
var frontendDist embed.FS

// newFrontendHandler serves the built Vue single-page app from the embedded
// frontend/dist tree. Paths that resolve to a built file are served directly;
// other extension-less paths fall back to index.html so history-mode routes
// survive a hard refresh. Asset misses and paths with a file extension return
// 404 rather than the SPA shell, and /api/* is never handled here (the API mux
// takes precedence).
func newFrontendHandler() (http.Handler, error) {
	sub, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		return nil, fmt.Errorf("locate embedded frontend: %w", err)
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return nil, fmt.Errorf("embedded frontend is missing index.html; run `make build-web` before `make build`: %w", err)
	}
	files := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		upath := path.Clean("/" + r.URL.Path)
		name := strings.TrimPrefix(upath, "/")

		if name != "" {
			if f, err := sub.Open(name); err == nil {
				info, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil && !info.IsDir() {
					setAssetCache(w, upath)
					files.ServeHTTP(w, r)
					return
				}
			}
		}

		if strings.HasPrefix(upath, "/assets/") || path.Ext(upath) != "" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	}), nil
}

// setAssetCache marks Vite's content-hashed assets as immutable so browsers
// never revalidate them. Everything else keeps the default revalidation.
func setAssetCache(w http.ResponseWriter, upath string) {
	if strings.HasPrefix(upath, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
}
