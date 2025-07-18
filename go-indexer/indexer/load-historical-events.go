package indexer

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/supabase-community/postgrest-go"
	"github.com/timewave/vault-indexer/go-indexer/config"
	"github.com/timewave/vault-indexer/go-indexer/database"
	event_queue "github.com/timewave/vault-indexer/go-indexer/event-ingestion-queue"
)

func (i *Indexer) loadHistoricalEvents(_currentBlock uint64, wg *sync.WaitGroup) error {

	for _, contractConfig := range i.config.Contracts {

		for _, eventName := range contractConfig.Events {
			wg.Add(1)
			// TODO: this is inefficient, consider loading outside of the loop
			parsedABI, err := i.loadAbi(contractConfig)
			if err != nil {
				return fmt.Errorf("failed to load ABI for contract %s: %w", contractConfig.Address, err)
			}
			event := parsedABI.Events[eventName]

			go func(contractConfig config.ContractConfig, event abi.Event) {
				defer wg.Done()

				select {
				case <-i.ctx.Done():
					return
				default:
					// continue
				}

				lastIndexedBlock, err := i.getLastIndexedBlock(contractConfig.Address, event)
				if lastIndexedBlock == nil {
					i.logger.Warn("No last indexed block found, using start block: %d", contractConfig.StartBlock)
					v := int64(contractConfig.StartBlock)
					lastIndexedBlock = &v
				} else {
					i.logger.Info("Last indexed block: %d", *lastIndexedBlock)
				}
				if err != nil {
					// Use centralized error channel
					select {
					case i.eventIngestionErrorChan <- fmt.Errorf("failed to get last indexed block: %w, event: %s, contract: %s", err, event.Name, contractConfig.Address):
					default:
						// Channel is closed or full, ignore
					}
					return
				}
				if lastIndexedBlock == nil {
					v := int64(contractConfig.StartBlock)
					lastIndexedBlock = &v
				}

				fromBlock := big.NewInt(int64(*lastIndexedBlock))
				toBlock := big.NewInt(int64(_currentBlock))

				i.logger.Info("Fetching historical events for %s on contract %s, from block %d to block %d", event.Name, contractConfig.Address, *lastIndexedBlock, _currentBlock)

				contractAddressFormatted := common.HexToAddress(contractConfig.Address)
				query := ethereum.FilterQuery{
					Addresses: []common.Address{contractAddressFormatted},
					Topics:    [][]common.Hash{{event.ID}},
					FromBlock: fromBlock,
					ToBlock:   toBlock,
				}

				logs, err := i.ethClient.FilterLogs(i.ctx, query)
				if err != nil {
					// Use centralized error channel
					select {
					case i.eventIngestionErrorChan <- fmt.Errorf("failed to get historical events: %w, event: %s, contract: %s", err, event.Name, contractConfig.Address):
					default:
						// Channel is closed or full, ignore
					}
					return
				}

				for _, vLog := range logs {
					i.logger.Debug("Received historical event: %v", vLog)
					eventLog := event_queue.EventLog{
						BlockNumber:     vLog.BlockNumber,
						LogIndex:        vLog.Index,
						Event:           event,
						Data:            vLog,
						ContractAddress: contractAddressFormatted,
						BlockHash:       vLog.BlockHash,
					}
					i.eventQueue.Insert(eventLog)
				}
			}(contractConfig, event)
		}
	}
	return nil
}
func (i *Indexer) getLastIndexedBlock(contractAddress string, event abi.Event) (*int64, error) {
	data, _, err := i.supabaseClient.From("events").
		Select("block_number", "", false).
		Eq("contract_address", contractAddress).
		Eq("event_name", event.Name).
		Order("block_number", &postgrest.OrderOpts{Ascending: false}).
		Limit(1, "").
		Execute()

	if err != nil {
		return nil, err
	}

	var indexedEvents []database.PublicEventsSelect
	i.logger.Debug("Getting last indexed block for contract %s, event %s, found: %s", contractAddress, event.Name, string(data))

	if err := json.Unmarshal(data, &indexedEvents); err != nil {
		return nil, err
	}

	if len(indexedEvents) == 0 {
		return nil, nil
	}

	lastIndexedEvent := indexedEvents[0]

	return &lastIndexedEvent.BlockNumber, nil
}

func (i *Indexer) loadAbi(contract config.ContractConfig) (abi.ABI, error) {
	abiPath := filepath.Join("abis", contract.ABIPath)
	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to read ABI file: %w", err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to parse ABI: %w", err)
	}
	return parsedABI, nil
}
