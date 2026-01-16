package tools

import (
	"context"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// LSPStatusTool reports current LSP client status, including server-sent $/progress streams.
func LSPStatusTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("lsp_status",
			mcp.WithDescription("Show current LSP connection status and server progress ($/progress). Useful for detecting whether a language server is still indexing or ready."),
			mcp.WithDestructiveHintAnnotation(false),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			status, err := BuildLSPStatus(bridge)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			payload, err := FormatLSPStatus(status)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			logger.Debug("lsp_status: reported status for clients")
			return mcp.NewToolResultText(payload), nil
		}
}

func RegisterLSPStatusTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	tool, handler := LSPStatusTool(bridge)
	mcpServer.AddTool(tool, handler)
}
