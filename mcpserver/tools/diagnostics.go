package tools

import (
	"context"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterDiagnosticsTool registers the diagnostics tool
func RegisterDiagnosticsTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("diagnostics",
		mcp.WithDescription("Get diagnostics (errors, warnings) for a document"),
		mcp.WithString("uri", mcp.Description("URI to the file")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse and validate parameters
		uri, err := request.RequireString("uri")
		if err != nil {
			logger.Error("diagnostics: URI parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Execute bridge method
		diagnostics, err := bridge.GetDiagnostics(uri)
		if err != nil {
			logger.Error("diagnostics: Request failed", err)
			return mcp.NewToolResultError("Failed to get diagnostics"), nil
		}

		// Format and return result
		content := formatDiagnostics(diagnostics)
		return mcp.NewToolResultText(content), nil
	})
}