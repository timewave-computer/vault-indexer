package logger

import (
	"fmt"
	"log"
	"os"
)

// Logger is a custom logger that supports prefixes
type Logger struct {
	Prefix string
	Logger *log.Logger
}

// NewLogger creates a new logger with the given prefix
func NewLogger(prefix string) *Logger {
	return &Logger{
		Prefix: fmt.Sprintf("[%s] ", prefix),
		Logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

// Printf logs a formatted message with the prefix
func (l *Logger) Printf(format string, v ...interface{}) {
	l.Logger.Printf(l.Prefix+format, v...)
}

// Println logs a message with the prefix
func (l *Logger) Println(v ...interface{}) {
	l.Logger.Println(append([]interface{}{l.Prefix}, v...)...)
}
