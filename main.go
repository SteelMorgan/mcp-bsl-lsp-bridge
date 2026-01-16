// Copyright 2025 Dave Lage (rockerBOO)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"rockerboo/mcp-lsp-bridge/bridge"
	"rockerboo/mcp-lsp-bridge/directories"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mcpserver"
	"rockerboo/mcp-lsp-bridge/security"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/mark3labs/mcp-go/server"
)

// tryLoadConfig attempts to load configuration from multiple locations with security validation
func tryLoadConfig(primaryPath, configDir string, allowedDirectories ...[]string) (*lsp.LSPServerConfig, error) {
	var configAllowedDirectories []string

	// If allowed directories are not provided, use default
	if len(allowedDirectories) > 0 {
		configAllowedDirectories = allowedDirectories[0]
	} else {
		// Get current working directory for validation
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}

		// Get allowed directories for config files
		configAllowedDirectories = security.GetConfigAllowedDirectories(configDir, cwd)
	}

	// Try primary path first (from command line or default)
	if config, err := lsp.LoadLSPConfig(primaryPath, configAllowedDirectories); err == nil {
		return config, nil
	}

	// If primary path fails and it's not the same as the fallback, try fallback locations
	fallbackPaths := []string{
		"lsp_config.json",                       // Current directory
		filepath.Join(configDir, "config.json"), // Alternative name in config dir
		"lsp_config.example.json",               // Example config in current dir
	}

	for _, fallbackPath := range fallbackPaths {
		if fallbackPath != primaryPath {
			if config, err := lsp.LoadLSPConfig(fallbackPath, configAllowedDirectories); err == nil {
				logger.Warn(fmt.Sprintf("INFO: Loaded configuration from fallback location: %s\n", fallbackPath))
				fmt.Fprintf(os.Stderr, "INFO: Loaded configuration from fallback location: %s\n", fallbackPath)
				return config, nil
			}
		}
	}

	return nil, errors.New("no valid configuration found")
}

// validateCommandLineArgs validates command line arguments for security
func validateCommandLineArgs(confPath, logPath, configDir, logDir string) error {
	// Get current working directory for validation
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Validate config path if provided
	if confPath != "" {
		configAllowedDirs := security.GetConfigAllowedDirectories(configDir, cwd)
		if _, err := security.ValidateConfigPath(confPath, configAllowedDirs); err != nil {
			return fmt.Errorf("invalid config path: %w", err)
		}
	}

	// Validate log path if provided
	if logPath != "" {
		logAllowedDirs := []string{logDir, cwd, "."}
		if _, err := security.ValidateConfigPath(logPath, logAllowedDirs); err != nil {
			return fmt.Errorf("invalid log path: %w", err)
		}
	}

	return nil
}

func main() {
	// Initialize directory resolver
	dirResolver := directories.NewDirectoryResolver("mcp-lsp-bridge", directories.DefaultUserProvider{}, directories.DefaultEnvProvider{}, true)

	// Get default directories
	configDir, err := dirResolver.GetConfigDirectory()
	if err != nil {
		log.Fatalf("Failed to get config directory: %v", err)
	}

	logDir, err := dirResolver.GetLogDirectory()
	if err != nil {
		log.Fatalf("Failed to get log directory: %v", err)
	}

	// Set up default paths
	defaultConfigPath := filepath.Join(configDir, "lsp_config.json")
	defaultLogPath := filepath.Join(logDir, "mcp-lsp-bridge.log")

	// Parse command line flags
	var confPath string

	var logPath string

	var logLevel string

	flag.StringVar(&confPath, "config", defaultConfigPath, "Path to LSP configuration file")
	flag.StringVar(&confPath, "c", defaultConfigPath, "Path to LSP configuration file (short)")
	flag.StringVar(&logPath, "log-path", "", "Path to log file (overrides config and default)")
	flag.StringVar(&logPath, "l", "", "Path to log file (short)")
	flag.StringVar(&logLevel, "log-level", "", "Log level: debug, info, warn, error (overrides config)")
	flag.Parse()

	// Validate command line arguments for security
	if err := validateCommandLineArgs(confPath, logPath, configDir, logDir); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Invalid command line arguments: %v\n", err)
		os.Exit(1)
	}

	// Load LSP configuration
	// Attempt to load config from multiple locations
	config, err := tryLoadConfig(confPath, configDir)
	logConfig := logger.LoggerConfig{}

	if err != nil {
		// Detailed error logging
		fullErrMsg := fmt.Sprintf("CRITICAL: Failed to load LSP config from '%s': %v", confPath, err)
		fmt.Fprintln(os.Stderr, fullErrMsg)
		log.Println(fullErrMsg)

		// Set default config when config load fails
		logConfig = logger.LoggerConfig{
			LogPath:     defaultLogPath,
			LogLevel:    "debug",
			MaxLogFiles: 10,
		}

		// Create minimal default LSP config so bridge can initialize
		config = &lsp.LSPServerConfig{
			LanguageServers:      make(map[types.LanguageServer]lsp.LanguageServerConfig),
			LanguageServerMap:    make(map[types.LanguageServer][]types.Language),
			ExtensionLanguageMap: make(map[string]types.Language),
			Global: struct {
				LogPath            string `json:"log_file_path"`
				LogLevel           string `json:"log_level"`
				MaxLogFiles        int    `json:"max_log_files"`
				MaxRestartAttempts int    `json:"max_restart_attempts"`
				RestartDelayMs     int    `json:"restart_delay_ms"`
			}{
				LogPath:     defaultLogPath,
				LogLevel:    "debug",
				MaxLogFiles: 10,
			},
		}

		// Ensure user is aware of configuration failure
		fmt.Fprintln(os.Stderr, "NOTICE: Using minimal default configuration. LSP functionality will be limited.")
	} else {
		logConfig = logger.LoggerConfig{
			LogPath:     config.Global.LogPath,
			LogLevel:    config.Global.LogLevel,
			MaxLogFiles: config.Global.MaxLogFiles,
		}
	}

	// Allow runtime tuning from outside (e.g. via Cursor MCP env vars)
	// without editing config files inside the container.
	lsp.ApplyEnvOverrides(config)

	// Override with command-line flags if provided
	if logPath != "" {
		logConfig.LogPath = logPath
	}

	if logLevel != "" {
		logConfig.LogLevel = logLevel
	}

	// Ensure we have a log path (use default if not specified)
	if logConfig.LogPath == "" {
		logConfig.LogPath = defaultLogPath
	}

	if err := logger.InitLogger(logConfig); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Close()

	logger.Info("Starting MCP-LSP Bridge...")
	
	// Debug: log to a file that persists between calls
	debugFile, _ := os.OpenFile("/tmp/mcp-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if debugFile != nil {
		fmt.Fprintf(debugFile, "=== MCP-LSP Bridge started at %s ===\n", time.Now().Format(time.RFC3339))
		defer debugFile.Close()
	}

	cwd, err := os.Getwd()
	if err != nil {
		panic("Failed to get current working directory: " + err.Error())
	}

	// In container mode we must anchor workspace operations to the mounted workspace root,
	// not to the process CWD (often /home/user).
	workspaceRoot := os.Getenv("WORKSPACE_ROOT")
	allowedDirs := []string{cwd}
	if workspaceRoot != "" {
		allowedDirs = []string{workspaceRoot}
	}

	// Create and initialize the bridge
	bridgeInstance := bridge.NewMCPLSPBridge(config, allowedDirs)

	// Setup MCP server with bridge
	mcpServer := mcpserver.SetupMCPServer(bridgeInstance)

	// Store the server reference in the bridge
	bridgeInstance.SetServer(mcpServer)

	// Start auto-connect + warm-up SYNCHRONOUSLY before MCP server starts.
	// This ensures LSP connections are fully established before stdin processing begins.
	// Critical for docker exec scenarios where stdin closes immediately after sending a request.
	logger.Info("Connecting to language servers...")
	if err := bridgeInstance.SyncAutoConnect(); err != nil {
		logger.Warn("Some language servers failed to connect: " + err.Error())
	}
	logger.Info("Language server connections ready.")

	// Start MCP server
	logger.Info("Starting MCP server...")
	fmt.Fprintln(os.Stderr, "DEBUG MAIN: About to call ServeStdio...")

	if err := server.ServeStdio(mcpServer); err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG MAIN: ServeStdio returned error: %v\n", err)
		logger.Error("MCP server error: " + err.Error())
	}
	fmt.Fprintln(os.Stderr, "DEBUG MAIN: ServeStdio returned, main() exiting...")
}
