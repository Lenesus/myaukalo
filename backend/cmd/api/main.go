package main

import (
	"log"

	"myaukalo/backend/internal/app"
)

func main() {
	config := app.Config{
		Port: "8080",
	}

	application := app.New(config)

	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}
