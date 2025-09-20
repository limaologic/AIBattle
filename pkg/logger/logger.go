// Package logger provides structured logging functionality for the Reverse Challenge System.
// Built on top of zerolog for high-performance structured logging with contextual fields.
// Supports different log levels and provides convenience methods for common use cases.
// Supports dual output to console and structured log files with timestamped naming.
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	// Global variables for file logging
	logFileMutex        sync.Mutex
	sequenceCounter     = make(map[string]int)
	serviceLoggers      = make(map[ServiceType]*os.File)
	serviceMultiWriters = make(map[ServiceType]io.Writer)
)

// LogCategory represents different types of log events
type LogCategory string

const (
	Startup  LogCategory = "startup"
	Callback LogCategory = "callback"
	Worker   LogCategory = "worker"
	Request  LogCategory = "request"
	GRPC     LogCategory = "grpc"
	Error    LogCategory = "error"
	General  LogCategory = "general"
)

// ServiceType represents the service generating the logs
type ServiceType string

const (
	Challenger ServiceType = "challenger"
	Solver     ServiceType = "solver"
)

// Init initializes the global logger with the specified log level.
// Sets up console output with pretty formatting for development use.
// Defaults to info level if an invalid level is provided.
func Init(level string) {
	// Set global log level
	switch strings.ToLower(level) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Configure pretty printing for development
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	})
}

// InitWithFileLogging initializes the logger with both console and file output.
// Creates timestamped log files in the logs/ directory with service information.
func InitWithFileLogging(level string, service ServiceType) {
	// Set global log level
	switch strings.ToLower(level) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	logFileMutex.Lock()
	defer logFileMutex.Unlock()

	// Check if we already have a multi-writer for this service
	if multiWriter, exists := serviceMultiWriters[service]; exists {
		// Configure global logger with existing multi-writer
		log.Logger = zerolog.New(multiWriter).With().Timestamp().Logger()
		return
	}

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		fmt.Printf("Failed to create logs directory: %v\n", err)
		return
	}

	// Generate log file name
	logFileName := generateLogFileName(service)
	logFilePath := filepath.Join("logs", logFileName)

	// Open log file
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file %s: %v\n", logFilePath, err)
		return
	}

	// Store the file handle for this service
	serviceLoggers[service] = logFile

	// Create console writer for pretty output
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	// Create multi-writer: console gets pretty format, file gets JSON
	multiWriter := zerolog.MultiLevelWriter(
		consoleWriter,
		logFile,
	)

	// Store the multi-writer for this service
	serviceMultiWriters[service] = multiWriter

	// Configure logger with multi-writer
	log.Logger = zerolog.New(multiWriter).With().Timestamp().Logger()

	fmt.Printf("Logging to file: %s\n", logFilePath)
}

// generateLogFileName creates a timestamped log file name with sequence number.
// Format: YYYYMMDD_HHMMSS_{service}_{sequence}.log
// Note: This function assumes the logFileMutex is already locked by the caller
func generateLogFileName(service ServiceType) string {
	now := time.Now()
	dateStr := now.Format("20060102")
	timeStr := now.Format("150405")

	// Create a key for sequence tracking
	key := fmt.Sprintf("%s_%s_%s", dateStr, timeStr, service)

	// Increment sequence counter
	sequenceCounter[key]++
	sequence := sequenceCounter[key]

	// Format: YYYYMMDD_HHMMSS_{service}_{sequence}.log
	return fmt.Sprintf("%s_%s_%s_%s.log",
		dateStr, timeStr, service, fmt.Sprintf("%03d", sequence))
}

// NewCategoryLogger creates a new logger instance with file output for a specific category.
// All categories for the same service write to the same file, with category information in the log entry.
func NewCategoryLogger(level string, service ServiceType, category LogCategory) zerolog.Logger {
	logFileMutex.Lock()
	defer logFileMutex.Unlock()

	// Check if we already have a multi-writer for this service
	if multiWriter, exists := serviceMultiWriters[service]; exists {
		// Return new logger instance using existing multi-writer
		return zerolog.New(multiWriter).With().Timestamp().Str("service", string(service)).Str("category", string(category)).Logger()
	}

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		fmt.Printf("Failed to create logs directory: %v\n", err)
		return log.Logger
	}

	// Generate log file name for this service
	logFileName := generateLogFileName(service)
	logFilePath := filepath.Join("logs", logFileName)

	// Open log file
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file %s: %v\n", logFilePath, err)
		return log.Logger
	}

	// Store the file handle for this service
	serviceLoggers[service] = logFile

	// Create console writer for pretty output
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	// Create multi-writer: console gets pretty format, file gets JSON
	multiWriter := zerolog.MultiLevelWriter(
		consoleWriter,
		logFile,
	)

	// Store the multi-writer for this service
	serviceMultiWriters[service] = multiWriter

	fmt.Printf("Logging for service %s to file: %s\n", service, logFilePath)

	// Return new logger instance
	return zerolog.New(multiWriter).With().Timestamp().Str("service", string(service)).Str("category", string(category)).Logger()
}

// WithRequestID creates a logger with a request ID field.
// Used for tracing requests across service boundaries and operations.
func WithRequestID(requestID string) zerolog.Logger {
	return log.With().Str("request_id", requestID).Logger()
}

// WithChallengeID creates a logger with a challenge ID field.
// Used for tracking operations related to specific challenges.
func WithChallengeID(challengeID string) zerolog.Logger {
	return log.With().Str("challenge_id", challengeID).Logger()
}

// WithKeyID creates a logger with a key ID field.
// Used for authentication and HMAC-related logging operations.
func WithKeyID(keyID string) zerolog.Logger {
	return log.With().Str("key_id", keyID).Logger()
}

// WithFields creates a logger with multiple custom fields.
// Allows adding arbitrary structured data to log entries.
func WithFields(fields map[string]interface{}) zerolog.Logger {
	logger := log.Logger
	for k, v := range fields {
		logger = logger.With().Interface(k, v).Logger()
	}
	return logger
}

// CleanupOldLogs removes log files older than the specified number of days.
// Helps prevent logs directory from growing indefinitely.
func CleanupOldLogs(daysToKeep int) error {
	logsDir := "logs"
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		return nil // No logs directory, nothing to clean
	}

	return filepath.Walk(logsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .log files
		if !strings.HasSuffix(info.Name(), ".log") {
			return nil
		}

		// Check if file is older than specified days
		age := time.Since(info.ModTime())
		if age > time.Duration(daysToKeep)*24*time.Hour {
			fmt.Printf("Removing old log file: %s\n", path)
			return os.Remove(path)
		}

		return nil
	})
}

// GetLogStats returns statistics about log files in the logs directory.
func GetLogStats() (map[string]int, error) {
	stats := make(map[string]int)
	logsDir := "logs"

	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		return stats, nil
	}

	err := filepath.Walk(logsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".log") {
			// Extract service and category from filename
			parts := strings.Split(info.Name(), "_")
			if len(parts) >= 4 {
				service := parts[2]
				category := parts[3]
				key := fmt.Sprintf("%s_%s", service, category)
				stats[key]++
			}
		}

		return nil
	})

	return stats, err
}
