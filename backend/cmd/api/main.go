package main

import (
	"log"

	"myaukalo/backend/internal/app"
)

func main() {
	application := app.New()

	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}
