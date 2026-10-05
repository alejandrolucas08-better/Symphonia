package server

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// StaticHandler serves the production build; API paths never fall back to HTML.
func StaticHandler(directory string) (http.Handler, error) {
	index := filepath.Join(directory, "index.html")
	info, err := os.Stat(index)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("FRONTEND_DIST must contain index.html")
	}
	files := http.FileServer(http.Dir(directory))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		if clean == "/api" || strings.HasPrefix(clean, "/api/") {
			http.NotFound(w, r)
			return
		}
		for _, segment := range strings.Split(clean, "/") {
			if strings.HasPrefix(segment, ".") || strings.Contains(segment, "\\") {
				http.NotFound(w, r)
				return
			}
		}
		file, err := os.Stat(filepath.Join(directory, filepath.FromSlash(clean)))
		if err == nil && !file.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		if path.Ext(clean) != "" || strings.HasPrefix(clean, "/assets/") || clean == "/assets" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, index)
	}), nil
}
