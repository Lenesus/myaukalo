package app

import (
	"log"
	nethttp "net/http"

	memory "myaukalo/backend/internal/storage/memory"
	httptransport "myaukalo/backend/internal/transport"
)

type App struct {
	server *nethttp.Server
}

func New() *App {
	mux := nethttp.NewServeMux()
	store := memory.NewMemoryStorage()

	httptransport.RegisterRoutes(mux, store)

	server := &nethttp.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	return &App{
		server: server,
	}
}

func (a *App) Run() error {
	log.Println("myaukalo backend started on :8080")
	return a.server.ListenAndServe()
}
