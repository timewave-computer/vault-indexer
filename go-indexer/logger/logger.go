package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	levelNames = map[LogLevel]string{
		DEBUG: "DEBUG",
		INFO:  "INFO",
		WARN:  "WARN",
		ERROR: "ERROR",
	}

	globalLogLevel LogLevel
	once           sync.Once
)

// InitGlobalLogLevel initializes the global log level from the given level string
// This should be called once at the start of the application
func InitGlobalLogLevel(level string) {
	once.Do(func() {
		globalLogLevel = parseLogLevel(level)
	})
}

// GetGlobalLogLevel returns the current global log level
func GetGlobalLogLevel() LogLevel {
	return globalLogLevel
}

// parseLogLevel converts a string log level to LogLevel type
func parseLogLevel(level string) LogLevel {
	level = strings.ToUpper(level)
	switch level {
	case "DEBUG":
		return DEBUG
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// Logger is a custom logger that supports prefixes and log levels
type Logger struct {
	Prefix   string
	Logger   *log.Logger
	LogLevel LogLevel
}

// NewLogger creates a new logger with the given prefix
func NewLogger(prefix string) *Logger {
	return &Logger{
		Prefix:   fmt.Sprintf("[%s] ", prefix),
		Logger:   log.New(os.Stdout, "", log.LstdFlags),
		LogLevel: globalLogLevel,
	}
}

// SetLogLevel sets the minimum log level for the logger
func (l *Logger) SetLogLevel(level LogLevel) {
	l.LogLevel = level
}

// logWithLevel logs a message if the given level is at least the minimum level
func (l *Logger) logWithLevel(level LogLevel, format string, v ...interface{}) {
	if level >= l.LogLevel {
		levelPrefix := fmt.Sprintf("[%s] ", levelNames[level])
		l.Logger.Printf(l.Prefix+levelPrefix+format, v...)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	l.logWithLevel(DEBUG, format, v...)
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	l.logWithLevel(INFO, format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.logWithLevel(WARN, format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.logWithLevel(ERROR, format, v...)
}

// Printf is kept for backward compatibility, maps to Info level
func (l *Logger) Printf(format string, v ...interface{}) {
	l.Info(format, v...)
}
