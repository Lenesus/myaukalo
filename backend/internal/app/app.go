package app

import (
	"log"
	nethttp "net/http"

	"myaukalo/backend/internal/storage/sqlite"
	httptransport "myaukalo/backend/internal/transport"
)

// defaultDSN — имя файла базы данных SQLite рядом с запущенным сервером.
const defaultDSN = "myaukalo.db"

type App struct {
	server  *nethttp.Server
	storage *sqlite.Storage
}

func New() (*App, error) {
	store, err := sqlite.New(defaultDSN)
	if err != nil {
		return nil, err
	}

	mux := nethttp.NewServeMux()
	httptransport.RegisterRoutes(mux, store)

	server := &nethttp.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	return &App{
		server:  server,
		storage: store,
	}, nil
}

func (a *App) Run() error {
	log.Println("myaukalo backend started on :8080 (storage: sqlite)")
	return a.server.ListenAndServe()
}

// Shutdown освобождает ресурсы приложения (закрывает БД).
func (a *App) Shutdown() error {
	return a.storage.Close()
}
