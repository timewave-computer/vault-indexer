package indexer

type StopChannel struct {
	isReorg     bool
	blockNumber int64
	blockHash   string
}

func NewStopChannel() chan StopChannel {
	return make(chan StopChannel)
}
