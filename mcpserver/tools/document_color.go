package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func DocumentColorTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("document_color",
			mcp.WithDescription("Get color information in a document (textDocument/documentColor)."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("document_color: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			colors, err := bridge.DocumentColor(uri)
			if err != nil {
				logger.Error("document_color: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Document color failed: %v", err)), nil
			}

			raw, err := json.Marshal(colors)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize result: %v", err)), nil
			}

			return mcp.NewToolResultText(string(raw)), nil
		}
}

func RegisterDocumentColorTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(DocumentColorTool(bridge))
}
