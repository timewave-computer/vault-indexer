package indexer

import (
	"fmt"
	"log"
	"os"
)

// Logger is a custom logger that supports prefixes
type Logger struct {
	prefix string
	logger *log.Logger
}

// NewLogger creates a new logger with the given prefix
func NewLogger(prefix string) *Logger {
	return &Logger{
		prefix: fmt.Sprintf("[%s] ", prefix),
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

// Printf logs a formatted message with the prefix
func (l *Logger) Printf(format string, v ...interface{}) {
	l.logger.Printf(l.prefix+format, v...)
}

// Println logs a message with the prefix
func (l *Logger) Println(v ...interface{}) {
	l.logger.Println(append([]interface{}{l.prefix}, v...)...)
}
