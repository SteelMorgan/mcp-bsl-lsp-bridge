package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerAnalyzeCodeTool registers the analyze_code tool
func registerAnalyzeCodeTool(mcpServer *server.MCPServer) {
	mcpServer.AddTool(mcp.NewTool("analyze_code",
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

// registerInferLanguageTool registers the infer_language tool
func registerInferLanguageTool(mcpServer *server.MCPServer, bridge BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("infer_language",
		mcp.WithDescription("Infer the programming language for a file"),
		mcp.WithString("file_path", mcp.Description("Path to the file to infer language")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := request.RequireString("file_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Infer language from file extension
		language, err := bridge.InferLanguage(filePath)
		if err != nil {
			ext := filepath.Ext(filePath)
			return mcp.NewToolResultError(fmt.Sprintf("No language found for extension %s", ext)), nil
		}

		return mcp.NewToolResultText(language), nil
	})
}

// registerLSPConnectTool registers the lsp_connect tool
func registerLSPConnectTool(mcpServer *server.MCPServer, bridge BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("lsp_connect",
		mcp.WithDescription("Connect to a language server for a specific language"),
		mcp.WithString("language", mcp.Description("Programming language to connect")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		language, err := request.RequireString("language")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Check if language server is configured
		config := bridge.GetConfig()
		if config == nil {
			return mcp.NewToolResultError("No configuration available"), nil
		}

		if _, exists := config.LanguageServers[language]; !exists {
			return mcp.NewToolResultError(fmt.Sprintf("No language server configured for %s", language)), nil
		}

		// Attempt to get or create the LSP client
		_, err = bridge.GetClientForLanguageInterface(language)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to set up LSP client: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Connected to LSP for %s", language)), nil
	})
}

// registerLSPDisconnectTool registers the lsp_disconnect tool
func registerLSPDisconnectTool(mcpServer *server.MCPServer, bridge BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("lsp_disconnect",
		mcp.WithDescription("Disconnect all active language server clients"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Close all active clients
		bridge.CloseAllClients()

		return mcp.NewToolResultText("All language server clients disconnected"), nil
	})
}