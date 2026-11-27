// Package web serves the panel's React app, embedded at build time.
// Build it with `npm --prefix web run build`; without a build, Handler
// returns nil and the API serves a placeholder page instead.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

func Handler() http.Handler {
	return handlerFS(dist)
}

func handlerFS(fsys fs.FS) http.Handler {
	root, err := fs.Sub(fsys, "dist")
	if err != nil {
		return nil
	}
	index, err := fs.ReadFile(root, "index.html")
	if err != nil {
		return nil
	}
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(root, name); err == nil && !st.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite puts a content hash in these names.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
		}
		// Client-side routes all get the app.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
