package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"myaukalo/backend/internal/app"
)

func main() {
	// ctx отменится сам, когда процесс получит SIGINT (Ctrl+C) или SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	if err := application.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
