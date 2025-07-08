package tools

import (
	"context"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterLSPDisconnectTool registers the lsp_disconnect tool
func RegisterLSPDisconnectTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(LSPDisconnectTool(bridge))
}

func LSPDisconnectTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("lsp_disconnect",
			mcp.WithDescription("Disconnect all active language server clients"),
			mcp.WithDestructiveHintAnnotation(false),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Close all active clients
			bridge.CloseAllClients()

			logger.Info("lsp_disconnect: Disconnected all language server clients")

			return mcp.NewToolResultText("All language server clients disconnected"), nil
		}
}
