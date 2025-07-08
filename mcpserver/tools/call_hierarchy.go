package tools

import (
	"context"
	"fmt"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterCallHierarchyTool registers the call hierarchy tool
func RegisterCallHierarchyTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(CallHierarchyTool(bridge))
}

func CallHierarchyTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("call_hierarchy",
			mcp.WithDescription("Show call hierarchy (callers and callees) for a symbol"),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file")),
			mcp.WithNumber("line", mcp.Description("Line number (0-based)")),
			mcp.WithNumber("character", mcp.Description("Character position (0-based)")),
			mcp.WithString("direction", mcp.Description("Direction: 'incoming', 'outgoing', or 'both' (default: 'both')")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse and validate parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("call_hierarchy: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				logger.Error("call_hierarchy: Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				logger.Error("call_hierarchy: Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Optional direction parameter - for future use
			_ = "both" // Default direction (unused for now)
			if direction, err := request.RequireString("direction"); err == nil {
				// Direction parameter exists but not used in current implementation
				_ = direction // TODO: Use direction when implementing call hierarchy filtering
			}

			// First, prepare call hierarchy
			lineUint32, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			characterUint32, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}

			items, err := bridge.PrepareCallHierarchy(uri, lineUint32, characterUint32)
			if err != nil {
				logger.Error("call_hierarchy: Prepare failed", err)
				return mcp.NewToolResultError("Failed to prepare call hierarchy"), nil
			}

			if len(items) == 0 {
				return mcp.NewToolResultText(fmt.Sprintf("No call hierarchy items found on line %d, character %d", line, character)), nil
			}

			var result strings.Builder

			result.WriteString(fmt.Sprintf("Prepared %d call hierarchy items\n\n", len(items)))

			// For now, just show the prepared items since incoming/outgoing calls need proper implementation
			for i, item := range items {
				result.WriteString(fmt.Sprintf("%d. %v\n", i+1, item))
			}

			result.WriteString("\nNote: Full incoming/outgoing call analysis is not yet implemented")

			return mcp.NewToolResultText(result.String()), nil
		}
}
