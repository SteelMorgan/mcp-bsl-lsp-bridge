package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterImplementationTool registers the implementation tool
func RegisterImplementationTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ImplementationTool(bridge))
}

func ImplementationTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("implementation",
			mcp.WithDescription("Find implementations of a symbol (interfaces, abstract methods)"),
			mcp.WithString("uri", mcp.Description("URI to the file")),
			mcp.WithNumber("line", mcp.Description("Line number (0-based)")),
			mcp.WithNumber("character", mcp.Description("Character position (0-based)")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse and validate parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("implementation: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				logger.Error("implementation: Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				logger.Error("implementation: Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Execute bridge method
			implementations, err := bridge.FindImplementations(uri, uint32(line), uint32(character))
			if err != nil {
				logger.Error("implementation: Request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to find implementations: %v", err)), nil
			}

			// Format and return result
			content := formatImplementations(implementations)

			return mcp.NewToolResultText(content), nil
		}
}
