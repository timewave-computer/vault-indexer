package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/timewave/vault-indexer/go-indexer/config"
	"github.com/timewave/vault-indexer/go-indexer/indexer"
)

// Example showing how to use the StopProcessors functionality
func main() {
	// Create a channel to receive stop signals
	stopSignalChannel := make(chan struct{})

	// Create your indexer as usual
	ctx := context.Background()
	cfg := &config.Config{
		// Your configuration here
	}

	indexerInstance, err := indexer.New(ctx, cfg)
	if err != nil {
		log.Fatal("Failed to create indexer:", err)
	}

	// Start a goroutine to monitor your specific channel
	go func() {
		// Wait for a message on your specific channel
		<-stopSignalChannel

		// When a message is received, stop the processors
		fmt.Println("Stop signal received! Stopping event ingestion processor and transformer...")
		indexerInstance.StopProcessors()
	}()

	// Start the indexer (this will block until stopped)
	go func() {
		if err := indexerInstance.Start(); err != nil {
			log.Fatal("Failed to start indexer:", err)
		}
	}()

	// Simulate receiving a stop signal after 30 seconds
	// In your real application, this would be replaced with actual channel monitoring logic
	time.Sleep(30 * time.Second)
	stopSignalChannel <- struct{}{}

	// Keep the program running to see the shutdown process
	time.Sleep(5 * time.Second)
	fmt.Println("Example completed")
}

// Example of how you might integrate this with a message queue or other notification system
func monitorSpecificChannel(indexerInstance *indexer.Indexer, channelName string) {
	// This is a placeholder for your actual channel monitoring logic
	// Replace this with your actual implementation (e.g., Redis, RabbitMQ, Kafka, etc.)

	for {
		// Example: Check if a message exists in your specific channel
		messageReceived := checkForMessage(channelName)

		if messageReceived {
			fmt.Printf("Message received on channel '%s', stopping processors...\n", channelName)
			indexerInstance.StopProcessors()
			break
		}

		// Poll every second (adjust as needed)
		time.Sleep(1 * time.Second)
	}
}

// Placeholder function - replace with your actual message checking logic
func checkForMessage(channelName string) bool {
	// Implement your logic here to check if a message exists in the specified channel
	// This could be:
	// - Redis pub/sub
	// - Database polling
	// - Message queue subscription
	// - WebSocket connection
	// - File system watcher
	// etc.
	return false
}
