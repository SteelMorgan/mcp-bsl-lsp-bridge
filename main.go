package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"rockerboo/mcp-lsp-bridge/bridge"
	"rockerboo/mcp-lsp-bridge/directories"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mcpserver"
	"rockerboo/mcp-lsp-bridge/security"

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
		"example.lsp_config.json",               // Example config in current dir
	}

	for _, fallbackPath := range fallbackPaths {
		if fallbackPath != primaryPath {
			if config, err := lsp.LoadLSPConfig(fallbackPath, configAllowedDirectories); err == nil {
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
			LanguageServers:      make(map[lsp.Language]lsp.LanguageServerConfig),
			ExtensionLanguageMap: make(map[string]lsp.Language),
			LanguageExtensionMap: make(map[lsp.Language][]string),
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

	cwd, err := os.Getwd()
	if err != nil {
		panic("Failed to get current working directory: " + err.Error())
	}

	// Create and initialize the bridge
	bridgeInstance := bridge.NewMCPLSPBridge(config, []string{cwd})

	// Setup MCP server with bridge
	mcpServer := mcpserver.SetupMCPServer(bridgeInstance)

	// Store the server reference in the bridge
	bridgeInstance.SetServer(mcpServer)

	// Start MCP server
	logger.Info("Starting MCP server...")

	if err := server.ServeStdio(mcpServer); err != nil {
		logger.Error("MCP server error: " + err.Error())
	}
}
