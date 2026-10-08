package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"myaukalo/backend/internal/app"
	"myaukalo/backend/internal/config"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	if err := application.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
