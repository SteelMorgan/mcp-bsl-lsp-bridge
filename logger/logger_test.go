package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoggerInitialization tests basic logger initialization
func TestLoggerInitialization(t *testing.T) {
	// Create a temporary log path
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "test.log")

	// Initialize logger with test configuration
	cfg := LoggerConfig{
		LogPath:     logPath,
		LogLevel:    "debug",
		MaxLogFiles: 3,
	}

	err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	defer Close()

	// Check if log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file was not created at %s", logPath)
	}
}

// TestLogLevels tests logging at different log levels
func TestLogLevels(t *testing.T) {
	// Create a temporary log directory
	logDir := t.TempDir()

	// Test different log levels
	testCases := []struct {
		name       string
		logLevel   string
		logFunc    func(...any)
		logMessage string
		shouldLog  bool
	}{
		{
			name:       "Info Log at Info Level",
			logLevel:   "info",
			logFunc:    Info,
			logMessage: "Test info message",
			shouldLog:  true,
		},
		{
			name:       "Debug Log at Info Level",
			logLevel:   "info",
			logFunc:    Debug,
			logMessage: "Test debug message",
			shouldLog:  false,
		},
		{
			name:       "Debug Log at Debug Level",
			logLevel:   "debug",
			logFunc:    Debug,
			logMessage: "Test debug message",
			shouldLog:  true,
		},
		{
			name:       "Error Log Always Logs",
			logLevel:   "info",
			logFunc:    Error,
			logMessage: "Test error message",
			shouldLog:  true,
		},
		{
			name:       "Warn Log at Info Level",
			logLevel:   "info",
			logFunc:    Warn,
			logMessage: "Test warn message",
			shouldLog:  true,
		},
		{
			name:       "Warn Log at Warn Level",
			logLevel:   "warn",
			logFunc:    Warn,
			logMessage: "Test warn message",
			shouldLog:  true,
		},
		{
			name:       "Warn Log at Error Level", 
			logLevel:   "error",
			logFunc:    Warn,
			logMessage: "Test warn message",
			shouldLog:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset logger
			logFile = nil
			infoLogger = nil
			errorLogger = nil
			debugLogger = nil

			// Create unique log path for this test case
			testLogPath := filepath.Join(logDir, fmt.Sprintf("test_%s.log", strings.ReplaceAll(tc.name, " ", "_")))

			// Initialize logger with test configuration
			cfg := LoggerConfig{
				LogPath:     testLogPath,
				LogLevel:    tc.logLevel,
				MaxLogFiles: 3,
			}

			err := InitLogger(cfg)
			if err != nil {
				t.Fatalf("Failed to initialize logger: %v", err)
			}

			defer Close()

			// Log the message
			tc.logFunc(tc.logMessage)

			// Read log file and check contents
			content, err := os.ReadFile(testLogPath) // #nosec G304
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			logged := strings.Contains(string(content), tc.logMessage)
			if logged != tc.shouldLog {
				t.Errorf("Unexpected logging behavior. Expected log: %v, Actual log: %v", tc.shouldLog, logged)
			}
		})
	}
}

// TestLogRotation tests log file rotation
func TestLogRotation(t *testing.T) {
	// Create a temporary log directory
	logDir := t.TempDir()

	// Prepare log path pattern
	baseLogPath := filepath.Join(logDir, "rotation.log")

	// Test log rotation
	testCases := []struct {
		name          string
		maxLogFiles   int
		expectedFiles int
	}{
		{
			name:          "Rotate with 3 max log files",
			maxLogFiles:   3,
			expectedFiles: 3,
		},
		{
			name:          "Rotate with 5 max log files",
			maxLogFiles:   5,
			expectedFiles: 5,
		},
		{
			name:          "No rotation with 0 max log files",
			maxLogFiles:   0,
			expectedFiles: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate multiple log file creations
			for i := 0; i < 10; i++ {
				cfg := LoggerConfig{
					LogPath:     baseLogPath,
					LogLevel:    "info",
					MaxLogFiles: tc.maxLogFiles,
				}

				err := InitLogger(cfg)
				if err != nil {
					t.Fatalf("Failed to initialize logger: %v", err)
				}

				defer Close()

				// Log some content
				Info(fmt.Sprintf("Log entry %d", i))
			}

			// Count log files
			files, err := filepath.Glob(baseLogPath + "*")
			if err != nil {
				t.Fatalf("Failed to list log files: %v", err)
			}

			// Check number of files based on max log files setting
			if tc.maxLogFiles > 0 {
				if len(files) > tc.maxLogFiles {
					t.Errorf("Too many log files. Expected max %d, got %d", tc.maxLogFiles, len(files))
				}
			} else {
				if len(files) != 1 {
					t.Errorf("Expected 1 log file when MaxLogFiles is 0, got %d", len(files))
				}
			}
		})
	}
}

// TestDefaultConfiguration tests the default logger configuration
func TestDefaultConfiguration(t *testing.T) {
	// Get default configuration
	defaultCfg := DefaultConfig()

	// Check default values
	if defaultCfg.LogLevel != "info" {
		t.Errorf("Unexpected default log level. Expected 'info', got %s", defaultCfg.LogLevel)
	}

	if defaultCfg.MaxLogFiles != 5 {
		t.Errorf("Unexpected default max log files. Expected 5, got %d", defaultCfg.MaxLogFiles)
	}

	if !strings.Contains(defaultCfg.LogPath, "mcp-lsp-bridge.log") {
		t.Errorf("Unexpected default log path. Got %s", defaultCfg.LogPath)
	}
}

// TestEmptyLogPath tests behavior when an empty log path is provided
func TestEmptyLogPath(t *testing.T) {
	// Create a base directory for testing absolute paths
	baseLogDir := t.TempDir()

	testCases := []struct {
		name          string
		inputLogPath  string
		expectDefault bool
	}{
		{
			name:          "Empty Path",
			inputLogPath:  "",
			expectDefault: true,
		},
		{
			name:          "Relative Path",
			inputLogPath:  "bridge.log",
			expectDefault: false,
		},
		{
			name:          "Absolute Path",
			inputLogPath:  filepath.Join(baseLogDir, "bridge.log"),
			expectDefault: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a configuration with provided log path
			cfg := LoggerConfig{
				LogPath:     tc.inputLogPath,
				LogLevel:    "debug",
				MaxLogFiles: 3,
			}

			// Initialize logger
			err := InitLogger(cfg)
			if err != nil {
				t.Fatalf("Failed to initialize logger: %v", err)
			}

			defer Close()

			// Log some messages
			Info("Test info message")
			Debug("Test debug message")
			Error("Test error message")

			// Verify log file path
			if tc.expectDefault {
				defaultCfg := DefaultConfig()
				if config.LogPath != defaultCfg.LogPath {
					t.Errorf("Expected default log path %s, got %s", defaultCfg.LogPath, config.LogPath)
				}
			} else {
				if config.LogPath != tc.inputLogPath {
					t.Errorf("Expected log path %s, got %s", tc.inputLogPath, config.LogPath)
				}
			}
		})
	}
}

// TestWarnFunction tests the Warn logging function specifically
func TestWarnFunction(t *testing.T) {
	logDir := t.TempDir()

	// Test Warn function at different log levels
	testCases := []struct {
		name       string
		logLevel   string
		shouldLog  bool
	}{
		{"Warn at info level", "info", true},
		{"Warn at warn level", "warn", true},
		{"Warn at debug level", "debug", false}, // warn not included in debug
		{"Warn at error level", "error", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset logger state
			logFile = nil
			infoLogger = nil
			errorLogger = nil
			debugLogger = nil

			// Use unique log file path for each test case
			logPath := filepath.Join(logDir, fmt.Sprintf("warn_test_%s.log", tc.name))
			
			cfg := LoggerConfig{
				LogPath:     logPath,
				LogLevel:    tc.logLevel,
				MaxLogFiles: 3,
			}

			err := InitLogger(cfg)
			if err != nil {
				t.Fatalf("Failed to initialize logger: %v", err)
			}
			defer Close()

			// Log a warning message
			testMessage := "Test warning message"
			Warn(testMessage)

			// Read log file and check contents
			content, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			logged := strings.Contains(string(content), testMessage)
			if logged != tc.shouldLog {
				t.Errorf("Unexpected logging behavior for level %s. Expected log: %v, Actual log: %v", 
					tc.logLevel, tc.shouldLog, logged)
			}
		})
	}
}

// TestRotateLogFilesCoverage tests additional paths in rotateLogFiles for better coverage
func TestRotateLogFilesCoverage(t *testing.T) {
	logDir := t.TempDir()
	baseLogPath := filepath.Join(logDir, "rotate_coverage.log")

	// Test case where MaxLogFiles is 0 (no rotation)
	t.Run("no rotation when MaxLogFiles is 0", func(t *testing.T) {
		cfg := LoggerConfig{
			LogPath:     baseLogPath,
			LogLevel:    "info",
			MaxLogFiles: 0,
		}

		// Create some files first
		for i := 0; i < 3; i++ {
			filename := fmt.Sprintf("%s.%d", baseLogPath, i)
			file, err := os.Create(filename)
			if err != nil {
				t.Fatalf("Failed to create test log file: %v", err)
			}
			file.Close()
		}

		filesBefore, _ := filepath.Glob(baseLogPath + "*")
		
		// Call rotate - should do nothing
		rotateLogFiles(cfg)
		
		filesAfter, _ := filepath.Glob(baseLogPath + "*")
		
		if len(filesAfter) != len(filesBefore) {
			t.Errorf("Files should not be rotated when MaxLogFiles is 0. Before: %d, After: %d", 
				len(filesBefore), len(filesAfter))
		}
	})
}

