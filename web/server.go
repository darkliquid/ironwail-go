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

		if strings.HasSuffix(path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}

		http.ServeFile(w, r, filepath.Join(dir, path))
	})


	fmt.Printf("Ironwail-Go WASM Web Server running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

type styleWriter interface {
	http.ResponseWriter
}
