package indexer

/*
This is a sorted queue that is used to store events.
It is used to store events in the order of their block number and log index.
*/

import (
	"container/heap"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type EventLog struct {
	BlockNumber     uint64
	LogIndex        uint
	Event           abi.Event
	Data            types.Log
	ContractAddress common.Address
}

func (e EventLog) Less(other EventLog) bool {
	if e.BlockNumber == other.BlockNumber {
		return e.LogIndex < other.LogIndex
	}
	return e.BlockNumber < other.BlockNumber
}

type EventQueue struct {
	items []EventLog
	mu    sync.Mutex
}

func NewEventQueue() *EventQueue {
	q := &EventQueue{}
	heap.Init(q)
	return q
}

func (q *EventQueue) Len() int { return len(q.items) }

func (q *EventQueue) Less(i, j int) bool {
	return q.items[i].Less(q.items[j])
}

func (q *EventQueue) Swap(i, j int) {
	q.items[i], q.items[j] = q.items[j], q.items[i]
}

func (q *EventQueue) Push(x interface{}) {
	q.items = append(q.items, x.(EventLog))
}

func (q *EventQueue) Pop() interface{} {
	n := len(q.items)
	item := q.items[n-1]
	q.items = q.items[0 : n-1]
	return item
}

// public API
func (q *EventQueue) Insert(event EventLog) {
	q.mu.Lock()
	defer q.mu.Unlock()
	heap.Push(q, event)
}

func (q *EventQueue) Next() (EventLog, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.Len() == 0 {
		return EventLog{}, false
	}
	ev := heap.Pop(q).(EventLog)
	return ev, true
}
