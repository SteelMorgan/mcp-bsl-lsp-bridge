package bridge

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// NewMCPLSPBridge creates a new bridge instance with optional configuration
func NewMCPLSPBridge() *MCPLSPBridge {
	bridge := &MCPLSPBridge{
		clients: make(map[string]*lsp.LanguageClient),
	}

	// Try to load configuration
	confPath := "lsp_config.json"
	config, err := lsp.LoadLSPConfig(confPath)
	if err != nil {
		log.Fatal(fmt.Sprintf("Could not load configuration from %s: %v", confPath, err))
	}

	bridge.config = config
	return bridge
}

// GetClientForLanguage retrieves or creates a language server client for a specific language
func (b *MCPLSPBridge) GetClientForLanguage(language string) (*lsp.LanguageClient, error) {
	// Check if client already exists
	if existingClient, exists := b.clients[language]; exists {
		return existingClient, nil
	}

	// Find the server configuration
	serverConfig, exists := b.config.LanguageServers[language]
	if !exists {
		return nil, fmt.Errorf("no server configuration found for language %s", language)
	}

	// For testing or mock connections, use a special mock client
	if serverConfig.Command == "echo" {
		lc, err := lsp.NewLanguageClient("echo", serverConfig.Args...)
		if err != nil {
			return nil, fmt.Errorf("failed to create mock language client: %w", err)
		}

		// Store the new client
		b.clients[language] = lc

		return lc, nil
	}

	// Create new language client
	lc, err := lsp.NewLanguageClient(serverConfig.Command, serverConfig.Args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create language client: %w", err)
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	root_uri := protocol.DocumentUri(fmt.Sprintf("file://%s", dir))
	process_id := int32(os.Getpid())

	// Prepare initialization parameters
	params := protocol.InitializeParams{
		ProcessId: &process_id,
		ClientInfo: &protocol.ClientInfo{
			Name:    "MCP-LSP Bridge",
			Version: "1.0.0",
		},
		RootUri:      &root_uri,
		Capabilities: lc.ClientCapabilities(),
	}

	// Apply any initialization options from the configuration
	initOptions := serverConfig.InitializationOptions
	if initOptions != nil {
		params.InitializationOptions = initOptions
	}

	// Send initialize request
	var result protocol.InitializeResult
	err = lc.SendRequest("initialize", params, &result, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("initialize request failed: %w", err)
	}

	lc.SetServerCapabilities(result.Capabilities)

	if result.ServerInfo != nil {
		log.Println(fmt.Sprintf("Initialize result - Server Info: %+v\n", *result.ServerInfo))
	}
	log.Println(fmt.Sprintf("Initialize result - Capabilities: %+v\n", result.Capabilities))

	// Send initialized notification
	err = lc.SendNotification("initialized", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	// Store the new client
	b.clients[language] = lc

	return lc, nil
}

// CloseAllClients closes all active language server clients
func (b *MCPLSPBridge) CloseAllClients() {
	for _, client := range b.clients {
		client.Close()
	}
	b.clients = make(map[string]*lsp.LanguageClient)
	b.currentClient = nil
}

// InferLanguage infers the programming language from a file path
func (b *MCPLSPBridge) InferLanguage(filePath string) (string, error) {
	ext := filepath.Ext(filePath)
	language, exists := b.config.ExtensionLanguageMap[ext]
	if !exists {
		return "", fmt.Errorf("no language found for extension %s", ext)
	}
	return language, nil
}

// GetConfig returns the bridge's configuration
func (b *MCPLSPBridge) GetConfig() *lsp.LSPServerConfig {
	return b.config
}

// GetServer returns the bridge's MCP server
func (b *MCPLSPBridge) GetServer() *server.MCPServer {
	return b.server
}

// SetServer sets the bridge's MCP server
func (b *MCPLSPBridge) SetServer(mcpServer *server.MCPServer) {
	b.server = mcpServer
}

// GetClientForLanguageInterface returns a client as interface{} for tool compatibility
func (b *MCPLSPBridge) GetClientForLanguageInterface(language string) (any, error) {
	return b.GetClientForLanguage(language)
}
