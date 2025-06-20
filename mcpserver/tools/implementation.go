package tools

import (
	"context"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterImplementationTool registers the implementation tool
func RegisterImplementationTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("implementation",
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
		implementations, err := bridge.FindImplementations(uri, int32(line), int32(character))
		if err != nil {
			logger.Error("implementation: Request failed", err)
			return mcp.NewToolResultError("Failed to find implementations"), nil
		}

		// Format and return result
		content := formatImplementations(implementations)
		return mcp.NewToolResultText(content), nil
	})
}