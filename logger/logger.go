package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type LoggerConfig struct {
	LogPath     string
	LogLevel    string // "info", "debug", "error"
	MaxLogFiles int    // Maximum number of log files to keep
}

var (
	config      LoggerConfig
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
	logFile     *os.File
	logMutex    sync.Mutex
)

// DefaultConfig provides a default logging configuration
func DefaultConfig() LoggerConfig {
	return LoggerConfig{
		LogPath:     filepath.Join(os.TempDir(), "mcp-lsp-bridge.log"),
		LogLevel:    "info",
		MaxLogFiles: 5,
	}
}

// InitLogger sets up file-based logging with configuration
func InitLogger(cfg LoggerConfig) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Use default config if not provided
	if cfg.LogPath == "" {
		cfg = DefaultConfig()
	}

	// Ensure log directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.LogPath), 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Rotate logs if max log files exceeded
	rotateLogFiles(cfg)

	// Open log file with append mode and create if not exists
	file, err := os.OpenFile(cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	logFile = file

	// Store configuration
	config = cfg

	// Create loggers with timestamps
	infoLogger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger = log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	debugLogger = log.New(file, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)

	return nil
}

// rotateLogFiles manages log file rotation
func rotateLogFiles(cfg LoggerConfig) {
	if cfg.MaxLogFiles <= 0 {
		return
	}

	// Find existing log files
	baseDir := filepath.Dir(cfg.LogPath)
	baseFileName := filepath.Base(cfg.LogPath)
	files, _ := filepath.Glob(filepath.Join(baseDir, baseFileName+".*"))

	// If max log files exceeded, remove oldest logs
	if len(files) >= cfg.MaxLogFiles {
		sort.Slice(files, func(i, j int) bool {
			fiA, _ := os.Stat(files[i])
			fiB, _ := os.Stat(files[j])

			return fiA.ModTime().Before(fiB.ModTime())
		})

		// Remove oldest log files
		for _, oldFile := range files[:len(files)-cfg.MaxLogFiles+1] {
			err := os.Remove(oldFile)
			if err != nil {
				Error(fmt.Errorf("failed to remove old log file: %v", err))
			}
		}
	}
}

// Info logs an informational message with caller context
func Info(v ...any) {
	if config.LogLevel == "info" || config.LogLevel == "debug" {
		if infoLogger != nil {
			_ = infoLogger.Output(2, fmt.Sprintln(v...))
		}
	}
}

// Warn logs a warning message with caller context
func Warn(v ...any) {
	if config.LogLevel == "info" || config.LogLevel == "warn" {
		if infoLogger != nil {
			_ = infoLogger.Output(2, fmt.Sprintln(v...))
		}
	}
}

// Error logs an error message with caller context
func Error(v ...any) {
	if errorLogger != nil {
		_ = errorLogger.Output(2, fmt.Sprintln(v...))
	}
}

// Debug logs a debug message with caller context
func Debug(v ...any) {
	if config.LogLevel == "debug" {
		if debugLogger != nil {
			_ = debugLogger.Output(2, fmt.Sprintln(v...))
		}
	}
}

// Close closes the log file
func Close() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile != nil {
		err := logFile.Close()
		if err != nil {
			log.Printf("failed to close log file: %v", err)
		}
	}
}
