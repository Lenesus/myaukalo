package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	nethttp "net/http"
	"strings"
)

var staticFS embed.FS

func Register(mux *nethttp.ServeMux) {
	sub, err := fs.Sub(staticFS, "static")

	if err != nil {
		panic(err)
	}

	files := nethttp.FileServer(nethttp.FS(sub))

	mux.HandleFunc("/", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodGet && r.Method != nethttp.MethodHead {
			nethttp.Error(w, "method not allowed", nethttp.StatusMethodNotAllowed)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(nethttp.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}

		files.ServeHTTP(w, r)
	})
}
