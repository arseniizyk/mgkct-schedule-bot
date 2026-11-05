package static

import (
	"io/fs"
	"net/http"
	"strings"
)

// SPAHandler returns an HTTP handler that serves single page app static assets,
// falling back to index.html for client-side routing.
func SPAHandler(files fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(files))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Проверяем, существует ли файл в встроенной ФС
		f, err := files.Open(path)
		if err == nil {
			_ = f.Close()
			if path != "index.html" && (strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".ico") || strings.HasSuffix(path, ".svg")) {
				w.Header().Set("Cache-Control", "public, max-age=3600")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// Если файл не найден (например, глубокий маршрут SPA), отдаём index.html без мутации исходного r
		rClone := r.Clone(r.Context())
		rClone.URL.Path = "/"
		fileServer.ServeHTTP(w, rClone)
	})
}
