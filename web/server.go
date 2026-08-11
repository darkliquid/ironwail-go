// Command web-server serves the WASM deployment harness locally over HTTP.
package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

func main() {
	port := "8080"
	dir := "."

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		if path == "/" || path == "." {
			path = "/index.html"
		}

		switch {
		case strings.HasSuffix(path, ".wasm"):
			w.Header().Set("Content-Type", "application/wasm")
		case strings.HasSuffix(path, ".js"):
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case strings.HasSuffix(path, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case strings.HasSuffix(path, ".json"):
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		case strings.HasSuffix(path, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}

		http.ServeFile(w, r, filepath.Join(dir, path))
	})

	fmt.Printf("Ironwail-Go WASM Web Server running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

type styleWriter interface {
	http.ResponseWriter
}
