package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterLSPConnectTool registers the lsp_connect tool
func RegisterLSPConnectTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("lsp_connect",
		mcp.WithDescription("Connect to a language server for a specific language"),
		mcp.WithString("language", mcp.Description("Programming language to connect")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		language, err := request.RequireString("language")
		if err != nil {
			logger.Error("lsp_connect: Language parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Check if language server is configured
		config := bridge.GetConfig()
		if config == nil {
			logger.Error("lsp_connect: No configuration available")
			return mcp.NewToolResultError("No configuration available"), nil
		}

		if _, exists := config.LanguageServers[language]; !exists {
			logger.Error("lsp_connect: No language server configured",
				fmt.Sprintf("Language: %s", language),
			)
			return mcp.NewToolResultError(fmt.Sprintf("No language server configured for %s", language)), nil
		}

		// Attempt to get or create the LSP client
		_, err = bridge.GetClientForLanguageInterface(language)
		if err != nil {
			logger.Error("lsp_connect: Failed to set up LSP client",
				fmt.Sprintf("Language: %s, Error: %v", language, err),
			)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to set up LSP client: %v", err)), nil
		}

		logger.Info("lsp_connect: Successfully connected to LSP",
			fmt.Sprintf("Language: %s", language),
		)

		return mcp.NewToolResultText(fmt.Sprintf("Connected to LSP for %s", language)), nil
	})
}