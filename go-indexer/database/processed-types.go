package database

// DatabaseOperations represents database operations to be performed
type DatabaseOperations struct {
	Inserts []DBOperation
	Updates []DBOperation
}

// DBOperation represents a single database operation
type DBOperation struct {
	Table string
	Data  any
}

// EventData represents the raw event data map
type EventData map[string]interface{}
