package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
)

// RegisterFormatDocumentTool registers the document formatting tool
func RegisterFormatDocumentTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("format_document",
		mcp.WithDescription("ACTIONABLE: Format a document according to language conventions with dual-mode operation. PREVIEW MODE (apply='false', default): Shows detailed formatting changes without modifying files - displays line-by-line changes, whitespace adjustments, and content modifications. APPLY MODE (apply='true'): Actually applies all formatting changes to the file. Supports customizable indentation and language-specific formatting rules. Always preview first for safety."),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("uri", mcp.Description("URI to the file to format (file:// scheme required, e.g., 'file:///path/to/file.go')")),
		mcp.WithNumber("tab_size", mcp.Description("Tab size for formatting (default: 4, affects indentation width)")),
		mcp.WithString("apply", mcp.Description("CRITICAL: Whether to apply formatting changes. 'false' (default) = preview only, 'true' = actually format file. ALWAYS preview first!")),
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

		// Parse apply parameter
		applyChanges := false
		if val, err := request.RequireString("apply"); err == nil {
			applyChanges = (val == "true" || val == "True" || val == "TRUE")
		}

		insertSpaces := true

		// Execute bridge method to get formatting edits
		tabSizeUint32, err := safeUint32(tabSize)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid tab size: %v", err)), nil
		}

		edits, err := bridge.FormatDocument(uri, tabSizeUint32, insertSpaces)
		if err != nil {
			logger.Error("format_document: Request failed", err)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format document: %+v", err)), nil
		}

		if applyChanges && len(edits) > 0 {
			// Apply the formatting changes to the file
			err := bridge.ApplyTextEdits(uri, edits)
			if err != nil {
				logger.Error("format_document: Failed to apply edits", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to apply formatting changes: %+v", err)), nil
			}

			// Return success message with applied changes
			content := formatTextEdits(edits)
			content += "\n✅ FORMATTING APPLIED ✅\nAll formatting changes have been applied to the file."

			return mcp.NewToolResultText(content), nil
		} else {
			// Just preview the changes
			content := formatTextEdits(edits)
			if len(edits) > 0 {
				content += "\n💡 To apply these changes, use: format_document with apply='true'"
			}

			return mcp.NewToolResultText(content), nil
		}
	})
}
