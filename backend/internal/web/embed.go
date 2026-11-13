package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	nethttp "net/http"
	"strings"
)

//go:embed static
var staticFS embed.FS

// Register монтирует раздачу клиента на тот же ServeMux, что и API.
//
// Почему одним сервером, а не отдельным статик-хостом:
//  1. //go:embed шьёт фронт в ЕДИНСТВЕННЫЙ бинарник — столп «локальность»;
//  2. клиент и API на одном origin (localhost:PORT) => CORS не нужен вообще,
//     fetch и EventSource работают same-origin из коробки.
func Register(mux *nethttp.ServeMux) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// static/ отсутствует в бинарнике — невозможно при корректной сборке.
		// Panic здесь громче и честнее, чем тихая 404 в рантайме.
		panic(err)
	}

	files := nethttp.FileServer(nethttp.FS(sub))

	mux.HandleFunc("/", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodGet && r.Method != nethttp.MethodHead {
			nethttp.Error(w, "method not allowed", nethttp.StatusMethodNotAllowed)
			return
		}

		// Не отдаём HTML вместо JSON-404 для неизвестных API-путей:
		// иначе GET /api/v1/typo вернёт index.html и собьёт с толку.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(nethttp.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}

		// Корень "/" FileServer сам отдаёт static/index.html.
		files.ServeHTTP(w, r)
	})
}
