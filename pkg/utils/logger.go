package utils

import (
	"log"
)

// Logger provides logging functionality
type Logger struct {
	verbose bool
}

// NewLogger creates a new logger instance
func NewLogger(verbose bool) *Logger {
	return &Logger{
		verbose: verbose,
	}
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...interface{}) {
	if l.verbose {
		log.Printf("[INFO] "+format, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.verbose {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	log.Printf("[WARNING] "+format, args...)
}
