package tools

import (
	"context"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterCodeActionsTool registers the code actions tool
func RegisterCodeActionsTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("code_actions",
		mcp.WithDescription("Get intelligent code actions including quick fixes, refactoring suggestions, and automated improvements for a code range. Returns language server suggested actions like import fixes, error corrections, extract method, add missing imports, implement interfaces, and other context-aware improvements. Use at error locations for fixes or at any code location for refactoring suggestions."),
		mcp.WithString("uri", mcp.Description("URI to the file (file:// scheme required, e.g., 'file:///path/to/file.go')")),
		mcp.WithNumber("line", mcp.Description("Start line number (0-based) - target specific code location or error")),
		mcp.WithNumber("character", mcp.Description("Start character position (0-based) - target specific code location or error")),
		mcp.WithNumber("end_line", mcp.Description("End line number (0-based, optional) - for range-based actions, defaults to start line")),
		mcp.WithNumber("end_character", mcp.Description("End character position (0-based, optional) - for range-based actions, defaults to start character")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse and validate parameters
		uri, err := request.RequireString("uri")
		if err != nil {
			logger.Error("code_actions: URI parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		line, err := request.RequireInt("line")
		if err != nil {
			logger.Error("code_actions: Line parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		character, err := request.RequireInt("character")
		if err != nil {
			logger.Error("code_actions: Character parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Optional end parameters, default to same as start
		endLine := line
		if val, err := request.RequireInt("end_line"); err == nil {
			endLine = val
		}

		endCharacter := character
		if val, err := request.RequireInt("end_character"); err == nil {
			endCharacter = val
		}

		// Execute bridge method
		actions, err := bridge.GetCodeActions(uri, int32(line), int32(character), int32(endLine), int32(endCharacter))
		if err != nil {
			logger.Error("code_actions: Request failed", err)
			return mcp.NewToolResultError("Failed to get code actions"), nil
		}

		// Format and return result
		content := formatCodeActions(actions)
		return mcp.NewToolResultText(content), nil
	})
}