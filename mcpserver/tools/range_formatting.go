package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RangeFormattingTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("range_formatting",
			mcp.WithDescription("Format a specific range within a document (textDocument/rangeFormatting). Use for targeted formatting in large files."),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithNumber("start_line", mcp.Description("Start line (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("start_character", mcp.Description("Start character (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("end_line", mcp.Description("End line (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("end_character", mcp.Description("End character (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("tab_size", mcp.Description("Tab size for formatting (default: 4)")),
			mcp.WithBoolean("insert_spaces", mcp.Description("Use spaces for indentation (default: true)"), mcp.DefaultBool(true)),
			mcp.WithString("apply", mcp.Description("Whether to apply formatting changes. 'false' (default) = preview only, 'true' = apply edits.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("range_formatting: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			startLine, err := request.RequireInt("start_line")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_line: %v", err)), nil
			}
			startCharacter, err := request.RequireInt("start_character")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_character: %v", err)), nil
			}
			endLine, err := request.RequireInt("end_line")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_line: %v", err)), nil
			}
			endCharacter, err := request.RequireInt("end_character")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_character: %v", err)), nil
			}

			tabSize := 4
			if val, err := request.RequireInt("tab_size"); err == nil {
				tabSize = val
			}
			insertSpaces := request.GetBool("insert_spaces", true)

			applyChanges := false
			if val, err := request.RequireString("apply"); err == nil {
				applyChanges = (val == "true" || val == "True" || val == "TRUE")
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			startLineUint32, err := safeUint32(startLine)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_line: %v", err)), nil
			}
			startCharacterUint32, err := safeUint32(startCharacter)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_character: %v", err)), nil
			}
			endLineUint32, err := safeUint32(endLine)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_line: %v", err)), nil
			}
			endCharacterUint32, err := safeUint32(endCharacter)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_character: %v", err)), nil
			}
			tabSizeUint32, err := safeUint32(tabSize)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid tab_size: %v", err)), nil
			}

			edits, err := bridge.RangeFormatting(uri, startLineUint32, startCharacterUint32, endLineUint32, endCharacterUint32, tabSizeUint32, insertSpaces)
			if err != nil {
				logger.Error("range_formatting: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Range formatting failed: %v", err)), nil
			}

			if applyChanges && len(edits) > 0 {
				if err := bridge.ApplyTextEdits(uri, edits); err != nil {
					logger.Error("range_formatting: apply edits failed", err)
					return mcp.NewToolResultError(fmt.Sprintf("Failed to apply edits: %v", err)), nil
				}
				content := formatTextEdits(edits)
				content += "\nFORMATTING APPLIED\nAll range formatting changes have been applied."
				return mcp.NewToolResultText(content), nil
			}

			content := formatTextEdits(edits)
			if len(edits) > 0 {
				content += "\nTo apply these changes, use: range_formatting with apply='true'"
			}
			return mcp.NewToolResultText(content), nil
		}
}

func RegisterRangeFormattingTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(RangeFormattingTool(bridge))
}
