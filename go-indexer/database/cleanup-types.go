package database

// CleanupOperation represents a single cleanup operation
type CleanupOperation struct {
	Type   CleanupOperationType
	Table  string
	Data   interface{}
	Filter []string // fields to use in WHERE clause
}

// CleanupOperationType defines the type of cleanup operation
type CleanupOperationType string

const (
	CleanupDelete CleanupOperationType = "DELETE"
	CleanupUpdate CleanupOperationType = "UPDATE"
)
