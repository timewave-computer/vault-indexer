package indexer

import (
	"sync"
	"time"

	"github.com/timewave/vault-indexer/go-indexer/logger"
)

// HaltManager provides a centralized halt mechanism for processors
type HaltManager struct {
	haltCh   chan struct{}
	isHalted bool
	haltMu   sync.Mutex
	logger   *logger.Logger
}

// NewHaltManager creates a new halt manager
func NewHaltManager() *HaltManager {
	return &HaltManager{
		haltCh:   make(chan struct{}, 1),
		isHalted: false,
		logger:   logger.NewLogger("HaltManager"),
	}
}

// Halt halts all processors that are listening to this halt manager
func (hm *HaltManager) Halt() {
	hm.haltMu.Lock()
	defer hm.haltMu.Unlock()
	if !hm.isHalted {
		hm.isHalted = true
		// non-blocking send in case multiple Halt() calls
		select {
		case hm.haltCh <- struct{}{}:
		default:
		}
		hm.logger.Info("Halt requested")
	}
}

// WaitHalted waits for the halt signal with a timeout
func (hm *HaltManager) WaitHalted(timeout time.Duration) bool {
	select {
	case <-hm.haltCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

// IsHalted returns whether the halt manager is currently halted
func (hm *HaltManager) IsHalted() bool {
	hm.haltMu.Lock()
	defer hm.haltMu.Unlock()
	return hm.isHalted
}

// HaltChannel returns the channel that processors should listen to for halt signals
func (hm *HaltManager) HaltChannel() <-chan struct{} {
	return hm.haltCh
}

// Close closes the halt channel
func (hm *HaltManager) Close() {
	hm.haltMu.Lock()
	defer hm.haltMu.Unlock()
	if !hm.isHalted {
		close(hm.haltCh)
		hm.isHalted = true
	}
}
