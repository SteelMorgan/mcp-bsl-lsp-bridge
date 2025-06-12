package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mcp_lsp_bridge"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// MCPLSPBridge combines MCP server capabilities with LSP client functionality
type MCPLSPBridge struct {
	server *server.MCPServer
	client *lsp.LanguageClient
}

// NewMCPLSPBridge creates a new bridge instance
func NewMCPLSPBridge() *MCPLSPBridge {
	bridge := &MCPLSPBridge{}

	return bridge
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
}

// setupLSPHandler configures the LSP message handlers
func (b *MCPLSPBridge) setupLSPClient() {

	lc, err := lsp.NewLanguageClient("gopls")

	if err != nil {
		log.Fatal(err)
	}

	defer lc.Close()

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	root_uri := protocol.DocumentUri(fmt.Sprintf("file://%s", dir))
	process_id := int32(os.Getpid())

	params := protocol.InitializeRequest{
		JsonRPC: "2.0",
		ID:      protocol.Or2[string, int32]{Value: "init-1"},
		Method:  "initialize",
		Params: protocol.InitializeParams{
			ProcessId: &process_id,
			ClientInfo: &protocol.ClientInfo{
				Name:    "MCP-LSP Bridge",
				Version: "1.0.0",
			},
			RootUri:      &root_uri,
			Capabilities: lc.ClientCapabilities(),
		},
	}

	// Send initialize request
	var result protocol.InitializeResult
	err = lc.SendRequest("initialize", params, &result, 10*time.Second)
	if err != nil {
		fmt.Printf("Initialize failed: %v\n", err)
		return
	}

	lc.SetServerCapabilities(result.Capabilities)

	log.Println(fmt.Sprintf("Initialize result - Server Info: %+v\n", mcp_lsp_bridge.SafePrettyPrint(result.ServerInfo)))
	log.Println(fmt.Sprintf("Initialize result - Capabilities: %+v\n", mcp_lsp_bridge.SafePrettyPrint(result.Capabilities)))

	// Send initialized notification
	err = lc.SendNotification("initialized", map[string]any{})
	if err != nil {
		fmt.Printf("Failed to send initialized notification: %v\n", err)
		return
	}

	b.client = lc
}

func main() {
	// Configure logging
	log.Println("Starting MCP-LSP Bridge...")

	// Create and start the bridge
	bridge := NewMCPLSPBridge()

	// Initialize MCP server
	bridge.setupMCPServer()

	// Initialize LSP client
	bridge.setupLSPClient()

	// Start MCP server (if needed as separate service)
	log.Println("Starting MCP server ... ")
	if err := server.ServeStdio(bridge.server); err != nil {
		log.Printf("MCP server error: %v", err)
	}

}
