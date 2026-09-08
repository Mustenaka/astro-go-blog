// Package adminui embeds the built admin SPA (admin/dist copied to internal/adminui/dist) and
// serves it under /admin/. The directory is git-ignored except for .gitkeep; run
// `pnpm build:admin` at the repo root before `go build`.
package adminui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the SPA. Unknown paths fall back to index.html (hash router, so rarely needed).
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/admin")
		p = strings.TrimPrefix(p, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			p = "index.html"
		}
		if p == "index.html" {
			// http.FileServer redirects "/index.html" to "/", which would loop; serve it directly.
			body, err := fs.ReadFile(sub, "index.html")
			if err != nil {
				http.Error(w, "admin UI missing", http.StatusNotFound)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: http: https:; connect-src 'self' http: https:; font-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + p
		fileServer.ServeHTTP(w, r2)
	})
}

// Available reports whether a built SPA is embedded (index.html exists).
func Available() bool {
	_, err := fs.Stat(dist, "dist/index.html")
	return err == nil
}
