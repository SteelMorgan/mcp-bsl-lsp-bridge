package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mcp_lsp_bridge"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// MCPLSPBridge combines MCP server capabilities with multiple LSP clients
type MCPLSPBridge struct {
	server        *server.MCPServer
	clients       map[string]*lsp.LanguageClient
	config        *lsp.LSPServerConfig
	currentClient *lsp.LanguageClient
}

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

	log.Println(fmt.Sprintf("Initialize result - Server Info: %+v\n", mcp_lsp_bridge.SafePrettyPrint(result.ServerInfo)))
	log.Println(fmt.Sprintf("Initialize result - Capabilities: %+v\n", mcp_lsp_bridge.SafePrettyPrint(result.Capabilities)))

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

// extractSemanticInfo retrieves semantic information for a given file
func (b *MCPLSPBridge) extractSemanticInfo(language, fileUri string) (*SemanticAnalysisResult, error) {
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, err
	}

	uri := protocol.DocumentUri(fileUri)

	var symbols []protocol.DocumentSymbol
	// Collect various semantic insights
	sym_err := client.SendRequest("textDocument/documentSymbol",
		protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		},
		&symbols,
		1*time.Second,
	)

	if sym_err != nil {
		return nil, sym_err
	}

	var references []protocol.Location

	ref_err := client.SendRequest("textDocument/references",
		protocol.ReferenceParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
		&references,
		1*time.Second,
	)

	if ref_err != nil {
		return nil, ref_err
	}

	return &SemanticAnalysisResult{
		Symbols:    symbols,
		References: references,
		// other semantic information
	}, nil
}

type SemanticAnalysisResult struct {
	Symbols    []protocol.DocumentSymbol
	References []protocol.Location
}

// setupMCPServer configures the MCP server with AI-powered tools
func (b *MCPLSPBridge) setupMCPServer() {
	hooks := &server.Hooks{}

	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		fmt.Printf("beforeAny: %s, %v, %v\n", method, id, message)
	})
	hooks.AddOnSuccess(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		fmt.Printf("onSuccess: %s, %v, %v, %v\n", method, id, message, result)
	})
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		fmt.Printf("onError: %s, %v, %v, %v\n", method, id, message, err)
	})
	hooks.AddBeforeInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest) {
		fmt.Printf("beforeInitialize: %v, %v\n", id, message)
	})
	hooks.AddOnRequestInitialization(func(ctx context.Context, id any, message any) error {
		fmt.Printf("AddOnRequestInitialization: %v, %v\n", id, message)
		// authorization verification and other preprocessing tasks are performed.
		return nil
	})
	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
		fmt.Printf("afterInitialize: %v, %v, %v\n", id, message, result)
	})
	hooks.AddAfterCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest, result *mcp.CallToolResult) {
		fmt.Printf("afterCallTool: %v, %v, %v\n", id, message, result)
	})
	hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest) {
		fmt.Printf("beforeCallTool: %v, %v\n", id, message)
	})

	b.server = server.NewMCPServer(
		"lsp-bridge-mcp",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithLogging(),
		server.WithHooks(hooks),
	)

	// Register MCP tools for code analysis
	b.server.AddTool(mcp.NewTool("analyze_code",
		mcp.WithDescription("Analyze code for completion suggestions and insights"),
		mcp.WithString("uri", mcp.Description("URI to the file location to analyze")),
		mcp.WithNumber("line", mcp.Description("Line of the file to analyze")),
		mcp.WithNumber("character", mcp.Description("Character of the line to analyze")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		uri, err := request.RequireString("uri")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		line, err := request.RequireInt("line")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		character, err := request.RequireInt("character")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("%s %d %d", uri, line, character)), nil
	})

	// Infer Language Tool
	b.server.AddTool(mcp.NewTool("infer_language",
		mcp.WithDescription("Infer the programming language for a file"),
		mcp.WithString("file_path", mcp.Description("Path to the file to infer language")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := request.RequireString("file_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Infer language from file extension
		ext := filepath.Ext(filePath)
		language, exists := b.config.ExtensionLanguageMap[ext]
		if !exists {
			return mcp.NewToolResultError(fmt.Sprintf("No language found for extension %s", ext)), nil
		}

		return mcp.NewToolResultText(language), nil
	})

	// LSP Connection Management Tool
	b.server.AddTool(mcp.NewTool("lsp_connect",
		mcp.WithDescription("Connect to a language server for a specific language"),
		mcp.WithString("language", mcp.Description("Programming language to connect")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		language, err := request.RequireString("language")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Find the language server configuration
		_, exists := b.config.LanguageServers[language]
		if !exists {
			return mcp.NewToolResultError(fmt.Sprintf("No language server configured for %s", language)), nil
		}

		// Attempt to get or create the LSP client
		_, err = b.GetClientForLanguage(language)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to set up LSP client: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Connected to LSP for %s", language)), nil
	})

	// LSP Disconnect Tool
	b.server.AddTool(mcp.NewTool("lsp_disconnect",
		mcp.WithDescription("Disconnect all active language server clients"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Close all active clients
		b.CloseAllClients()

		return mcp.NewToolResultText("All language server clients disconnected"), nil
	})
}

func main() {
	// Configure logging
	log.Println("Starting MCP-LSP Bridge...")

	// Create and start the bridge
	bridge := NewMCPLSPBridge()

	// Initialize MCP server
	bridge.setupMCPServer()

	// Start MCP server (if needed as separate service)
	log.Println("Starting MCP server ... ")
	if err := server.ServeStdio(bridge.server); err != nil {
		log.Printf("MCP server error: %v", err)
	}
}