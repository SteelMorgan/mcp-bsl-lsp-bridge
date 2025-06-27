package tools

import (
	"context"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterRenameTool registers the rename tool
func RegisterRenameTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(RenameTool(bridge))
}

func RenameTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("rename",
			mcp.WithDescription("ACTIONABLE: Rename a symbol across the entire codebase with LSP precision and cross-file support. PREVIEW MODE (apply='false', default): Shows all files and exact locations that would be modified, with line numbers and replacement text. APPLY MODE (apply='true'): Actually performs the rename across all affected files in the codebase. Requires precise coordinates from definitions or hover for accurate targeting. Supports functions, variables, types, and other symbols. Always preview first to verify scope."),
			mcp.WithString("uri", mcp.Description("URI to the file containing the symbol (file:// scheme required, e.g., 'file:///path/to/file.go')")),
			mcp.WithNumber("line", mcp.Description("Line number (0-based) where the symbol is located - use coordinates from project_analysis definitions for precision")),
			mcp.WithNumber("character", mcp.Description("Character position (0-based) within the line - use coordinates from project_analysis definitions for precision")),
			mcp.WithString("new_name", mcp.Description("New name for the symbol - must be a valid identifier for the programming language")),
			mcp.WithString("apply", mcp.Description("CRITICAL: Whether to apply rename changes. 'false' (default) = preview only, 'true' = actually rename across codebase. ALWAYS preview first!")),
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

			// Parse apply parameter
			applyChanges := false
			if val, err := request.RequireString("apply"); err == nil {
				applyChanges = (val == "true" || val == "True" || val == "TRUE")
			}

			// Execute bridge method to get rename edits
			result, err := bridge.RenameSymbol(uri, uint32(line), uint32(character), newName, false) // Always get actual edits
			if err != nil {
				logger.Error("rename: Request failed", err)
				return mcp.NewToolResultError("Failed to rename symbol"), nil
			}

			if applyChanges {
				// Apply the rename changes
				err := bridge.ApplyWorkspaceEdit(result)
				if err != nil {
					logger.Error("rename: Failed to apply workspace edit", err)
					return mcp.NewToolResultError("Failed to apply rename changes"), nil
				}

				// Return success message with applied changes
				content := formatWorkspaceEdit(result)
				content += "\n✅ RENAME APPLIED ✅\nAll rename changes have been applied across the codebase."

				return mcp.NewToolResultText(content), nil
			} else {
				// Just preview the changes
				content := formatWorkspaceEdit(result)
				if content != "=== RENAME PREVIEW ===\nWorkspace edit: <nil>" {
					content += "\n💡 To apply these changes, use: rename with apply='true'"
				}

				return mcp.NewToolResultText(content), nil
			}
		}
}
