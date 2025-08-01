package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterLSPConnectTool registers the lsp_connect tool
func RegisterLSPConnectTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(LSPConnectTool(bridge))
}

func LSPConnectTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("lsp_connect",
			mcp.WithDescription("Connect to a language server for a specific language. REQUIRED for advanced analysis - establishes LSP connection to enable intelligent code operations like hover, definitions, and diagnostics."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("language", mcp.Description("Programming language to connect")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			language, err := request.RequireString("language")
			if err != nil {
				logger.Error("lsp_connect: Language parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			_, err = bridge.GetClientForLanguage(language)

			if err != nil {
				logger.Error("lsp_connect: Failed to get LSP client",
					fmt.Sprintf("Language: %s, Error: %v", language, err),
				)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get LSP client for %s: %v", language, err)), nil
			}

			return mcp.NewToolResultText("Connected to LSP for " + language), nil
		}
}
