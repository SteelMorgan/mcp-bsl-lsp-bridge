package tools

import (
	"context"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterFormatDocumentTool registers the document formatting tool
func RegisterFormatDocumentTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("format_document",
		mcp.WithDescription("Format a document according to language conventions"),
		mcp.WithString("uri", mcp.Description("URI to the file")),
		mcp.WithNumber("tab_size", mcp.Description("Tab size for formatting (default: 4)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse and validate parameters
		uri, err := request.RequireString("uri")
		if err != nil {
			logger.Error("format_document: URI parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Optional parameters with defaults
		tabSize := 4
		if val, err := request.RequireInt("tab_size"); err == nil {
			tabSize = val
		}

		insertSpaces := true
		// For now, we'll use a default value since RequireBoolean might not be available
		// TODO: Handle boolean parameter when MCP framework supports it

		// Execute bridge method
		edits, err := bridge.FormatDocument(uri, int32(tabSize), insertSpaces)
		if err != nil {
			logger.Error("format_document: Request failed", err)
			return mcp.NewToolResultError("Failed to format document"), nil
		}

		// Format and return result
		content := formatTextEdits(edits)
		return mcp.NewToolResultText(content), nil
	})
}