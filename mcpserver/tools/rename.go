package tools

import (
	"context"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterRenameTool registers the rename tool
func RegisterRenameTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("rename",
		mcp.WithDescription("Rename a symbol with preview of changes"),
		mcp.WithString("uri", mcp.Description("URI to the file")),
		mcp.WithNumber("line", mcp.Description("Line number (0-based)")),
		mcp.WithNumber("character", mcp.Description("Character position (0-based)")),
		mcp.WithString("new_name", mcp.Description("New name for the symbol")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse and validate parameters
		uri, err := request.RequireString("uri")
		if err != nil {
			logger.Error("rename: URI parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		line, err := request.RequireInt("line")
		if err != nil {
			logger.Error("rename: Line parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		character, err := request.RequireInt("character")
		if err != nil {
			logger.Error("rename: Character parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		newName, err := request.RequireString("new_name")
		if err != nil {
			logger.Error("rename: New name parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Execute bridge method (always preview for now)
		result, err := bridge.RenameSymbol(uri, int32(line), int32(character), newName, true)
		if err != nil {
			logger.Error("rename: Request failed", err)
			return mcp.NewToolResultError("Failed to rename symbol"), nil
		}

		// Format and return result
		content := formatWorkspaceEdit(result)
		return mcp.NewToolResultText(content), nil
	})
}