package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/timewave/vault-indexer/internal/config"
	"github.com/timewave/vault-indexer/internal/indexer"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize indexer
	idx, err := indexer.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize indexer: %v", err)
	}

	// Start the indexer
	if err := idx.Start(); err != nil {
		log.Fatalf("Failed to start indexer: %v", err)
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Graceful shutdown
	if err := idx.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
