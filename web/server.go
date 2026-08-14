// Command web-server serves the WASM deployment harness locally over HTTP.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func findRepoRoot() string {
	dir, err := filepath.Abs(".")
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

func main() {
	port := "8080"
	dir := "."
	repoRoot := findRepoRoot()

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
		case strings.HasSuffix(path, ".pak"):
			w.Header().Set("Content-Type", "application/octet-stream")
		}

		target := filepath.Join(dir, path)
		if strings.HasSuffix(path, "pak0.pak") {
			candidates := []string{
				target,
				filepath.Join(repoRoot, "testdata", "id1", "pak0.pak"),
				filepath.Join(repoRoot, "quake-data", "id1", "pak0.pak"),
				filepath.Join(repoRoot, "id1", "pak0.pak"),
			}
			for _, cand := range candidates {
				if _, err := os.Stat(cand); err == nil {
					target = cand
					break
				}
			}
		}

		http.ServeFile(w, r, target)
	})

	fmt.Printf("Ironwail-Go WASM Web Server running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
