package indexer

import (
	"context"
	"log"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/timewave/vault-indexer/internal/config"
)

type Indexer struct {
	config *config.Config
	client *ethclient.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func New(cfg *config.Config) (*Indexer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client, err := ethclient.Dial(cfg.Ethereum.WebsocketURL)
	if err != nil {
		cancel()
		return nil, err
	}

	return &Indexer{
		config: cfg,
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (i *Indexer) Start() error {
	log.Println("Starting indexer...")

	// TODO: Implement the following:
	// 1. Load historical events for each contract
	// 2. Set up event subscriptions
	// 3. Initialize database connection
	// 4. Start event processing

	return nil
}

func (i *Indexer) Stop() error {
	log.Println("Stopping indexer...")
	i.cancel()
	i.client.Close()
	return nil
}
