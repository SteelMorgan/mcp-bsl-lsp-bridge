package tools

import (
	"context"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterLSPDisconnectTool registers the lsp_disconnect tool
func RegisterLSPDisconnectTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("lsp_disconnect",
		mcp.WithDescription("Disconnect all active language server clients"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Close all active clients
		bridge.CloseAllClients()

		logger.Info("lsp_disconnect: Disconnected all language server clients")

		return mcp.NewToolResultText("All language server clients disconnected"), nil
	})
}