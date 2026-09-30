package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	nethttp "net/http"
	"time"

	"myaukalo/backend/internal/realtime"
	"myaukalo/backend/internal/storage/sqlite"
	httptransport "myaukalo/backend/internal/transport"
)

const defaultDSN = "myaukalo.db"

type App struct {
	server  *nethttp.Server
	storage *sqlite.Storage
	hub     *realtime.Hub
}

func New() (*App, error) {
	baseStore, err := sqlite.New(defaultDSN)
	if err != nil {
		return nil, err
	}

	hub := realtime.NewHub()

	// Транспорт видит только интерфейс storage.Storage и не знает,
	// что внутри есть рассылка. Магия декоратора.
	store := realtime.NewPublishingStorage(baseStore, hub)

	mux := nethttp.NewServeMux()
	httptransport.RegisterRoutes(mux, store)

	// SSE-эндпоинт регистрируем здесь, минуя RegisterRoutes,
	// чтобы не менять сигнатуру и не ломать routes_test.go.
	mux.HandleFunc("/api/v1/events", realtime.HandleEvents(hub, store))

	server := &nethttp.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	return &App{
		server:  server,
		storage: baseStore,
		hub:     hub,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	log.Println("myaukalo backend started on :8080 (storage: sqlite, realtime: sse)")

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Println("signal received, shutting down gracefully...")

		// Сначала гасим стримы: все SSE-горгутины видят hub.Done() и выходят,
		// запросы завершаются, и server.Shutdown пролетает мгновенно,
		// а не ждёт 5 секунд на бесконечных подписках.
		a.hub.Close()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown error: %v", err)
		}
		<-errCh

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
