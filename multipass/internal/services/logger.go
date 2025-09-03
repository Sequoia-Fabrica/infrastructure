package services

import (
	"log"
	"multipass/internal/config"
)

// Logger provides debug logging functionality that respects environment settings
type Logger struct {
	cfg *config.Config
}

// NewLogger creates a new logger instance
func NewLogger(cfg *config.Config) *Logger {
	return &Logger{
		cfg: cfg,
	}
}

// Debug logs a debug message only when in development environment
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.cfg.IsDevelopment() {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Error logs an error message regardless of environment
func (l *Logger) Error(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format, v...)
}

// Info logs an informational message regardless of environment
func (l *Logger) Info(format string, v ...interface{}) {
	log.Printf("[INFO] "+format, v...)
}

// Fatal logs a fatal error message and exits regardless of environment
func (l *Logger) Fatal(format string, v ...interface{}) {
	log.Fatalf("[FATAL] "+format, v...)
}
