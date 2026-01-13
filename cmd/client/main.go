package main

import (
	"context"
	"flag"
	"log"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	app, err := NewDronnayak(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	if err := app.Run(context.Background()); err != nil {
		log.Fatalf("Application error: %v", err)
	}

	log.Println("Shutdown complete")
}
