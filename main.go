package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/timewave/vault-indexer/go-indexer/config"
	"github.com/timewave/vault-indexer/go-indexer/indexer"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

func main() {

	// Load configuration
	configLoader := config.New(logger.NewLogger("Config"))
	cfg, err := config.Load(configLoader)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize global log level from config
	logger.InitGlobalLogLevel(cfg.LogLevel)

	// Create root context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create indexer with parent context
	idx, err := indexer.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}

	// Start indexer in a goroutine
	go func() {
		if err := idx.Start(); err != nil {
			log.Printf("Indexer stopped with error: %v", err)
			cancel() // trigger shutdown if Start() fails
		}
	}()

	// Listen for OS signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Received signal: %s", sig)
		cancel()
	case <-ctx.Done():
		// If Start() failed, we exit here
	}

	// Graceful shutdown
	if err := idx.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Shutdown complete")
}
