package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	nethttp "net/http"
	"sync"
	"time"

	"myaukalo/backend/internal/config"
	"myaukalo/backend/internal/realtime"
	"myaukalo/backend/internal/storage/sqlite"
	httptransport "myaukalo/backend/internal/transport"
)

const expiryInterval = 30 * time.Second

type App struct {
	server  *nethttp.Server
	storage *sqlite.Storage
	hub     *realtime.Hub
}

func New(cfg config.Config) (*App, error) {
	baseStore, err := sqlite.New(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	hub := realtime.NewHub()
	store := realtime.NewPublishingStorage(baseStore, hub)

	mux := nethttp.NewServeMux()
	httptransport.RegisterRoutes(mux, store)
	mux.HandleFunc("/api/v1/events", realtime.HandleEvents(hub, store))

	server := &nethttp.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	return &App{
		server:  server,
		storage: baseStore,
		hub:     hub,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	log.Printf("myaukalo backend started on %s (storage: sqlite, realtime: sse, ttl cleanup: %s)", a.server.Addr, expiryInterval)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go a.expiryLoop(runCtx, &wg)

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Println("signal received, shutting down gracefully...")

		a.hub.Close()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown error: %v", err)
		}
		<-errCh

	case err := <-errCh:
		if err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			cancel()
			wg.Wait()
			_ = a.storage.Close()
			return fmt.Errorf("server failed: %w", err)
		}
	}

	cancel()
	wg.Wait()

	if err := a.storage.Close(); err != nil {
		return fmt.Errorf("close storage: %w", err)
	}

	log.Println("server stopped, database closed. meow 🐾")
	return nil
}

func (a *App) expiryLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	a.cleanupExpired()

	ticker := time.NewTicker(expiryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.cleanupExpired()
		}
	}
}

func (a *App) cleanupExpired() {
	removed, err := a.storage.DeleteExpiredMessages(time.Now().UTC())
	if err != nil {
		log.Printf("expiry cleanup error: %v", err)
		return
	}

	if removed > 0 {
		log.Printf("expired messages removed: %d", removed)
	}
}
