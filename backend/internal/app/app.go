package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	nethttp "net/http"
	"time"

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

// Run запускает HTTP-сервер и ждёт одного из двух событий:
//   - ctx отменён (пришли SIGINT/SIGTERM) — гасим сервер gracefully;
//   - сервер сам упал с ошибкой — возвращаем её наверх.
func (a *App) Run(ctx context.Context) error {
	log.Println("myaukalo backend started on :8080 (storage: sqlite)")

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Println("signal received, shutting down gracefully...")

		// Даём текущим запросам не более 5 секунд на дозавершение.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown error: %v", err)
		}
		<-errCh // дожидаемся завершения горутины сервера

	case err := <-errCh:
		if err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			return fmt.Errorf("server failed: %w", err)
		}
	}

	if err := a.storage.Close(); err != nil {
		return fmt.Errorf("close storage: %w", err)
	}

	log.Println("server stopped, database closed. meow 🐾")
	return nil
}
