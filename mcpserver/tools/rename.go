package tools

import (
	"context"
	"fmt"

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
			mcp.WithDescription(`Rename symbols across entire codebase with cross-file precision and preview mode. SAFEST approach for refactoring - tracks all dependencies to prevent breaking changes that manual renaming causes.

USAGE:
- Preview: uri="file://path", line=10, character=5, new_name="newFunc", apply="false"
- Apply: Same parameters with apply="true"

PARAMETERS: uri (required), line/character (required), new_name (required), apply (default: false)
OUTPUT: All affected files with exact change locations`),
			mcp.WithDestructiveHintAnnotation(true),
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

			// Safe conversions for line and character
			lineUint32, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			characterUint32, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}

			// Execute bridge method to get rename edits
			result, err := bridge.RenameSymbol(uri, lineUint32, characterUint32, newName, false) // Always get actual edits
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
				content += "\nRENAME APPLIED\nAll rename changes have been applied across the codebase."

				return mcp.NewToolResultText(content), nil
			} else {
				// Just preview the changes
				content := formatWorkspaceEdit(result)
				if content != "=== RENAME PREVIEW ===\nWorkspace edit: <nil>" {
					content += "\nTo apply these changes, use: rename with apply='true'"
				}

				return mcp.NewToolResultText(content), nil
			}
		}
}
