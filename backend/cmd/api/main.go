package main

import (
	"log"

	"myaukalo/backend/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := application.Shutdown(); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}
