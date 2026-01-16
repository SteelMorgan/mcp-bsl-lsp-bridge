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

func FoldingRangeTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("folding_range",
			mcp.WithDescription("Get folding ranges for a document (textDocument/foldingRange)."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("folding_range: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			ranges, err := bridge.FoldingRange(uri)
			if err != nil {
				logger.Error("folding_range: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Folding range failed: %v", err)), nil
			}

			raw, err := json.Marshal(ranges)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize result: %v", err)), nil
			}

			return mcp.NewToolResultText(string(raw)), nil
		}
}

func RegisterFoldingRangeTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(FoldingRangeTool(bridge))
}
