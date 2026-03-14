package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed api
var apiFS embed.FS

// docsHandler serves Swagger UI and the OpenAPI spec.
// GET /docs       → docs.html (Swagger UI)
// GET /docs/openapi.yaml → OpenAPI spec
func docsHandler() http.Handler {
	sub, _ := fs.Sub(apiFS, "api")
	mux := http.NewServeMux()
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			r.URL.Path = "/docs.html"
		}
		fileServer.ServeHTTP(w, r)
	})
	return mux
}
