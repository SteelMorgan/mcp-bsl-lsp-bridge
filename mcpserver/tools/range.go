package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GetRangeContentTool defines the lsp__get_range_content tool.
func RangeContentTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_range_content",
			mcp.WithDescription("Get text content from file range. HIGHLY EFFICIENT for extracting specific code blocks - much faster than reading entire files when you need targeted content from precise locations."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to file (file:// scheme required)."), mcp.Required()),
			mcp.WithNumber("start_line", mcp.Description("Start line (0-based)."), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("start_character", mcp.Description("Start character (0-based)."), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("end_line", mcp.Description("End line (0-based)."), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("end_character", mcp.Description("End character (0-based)."), mcp.Required(), mcp.Min(0)),
			mcp.WithBoolean("strict", mcp.Description("Strict bounds checking. If true, fails on any out-of-bounds characters. If false (default), clamps character positions to line boundaries."), mcp.DefaultBool(false)),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			logger.Info("GetRangeContent Tool: Received request")

			// Parse parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("get_range_content: URI parsing failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Invalid URI parameter: %v", err)), nil
			}

			startLineInt, err := request.RequireInt("start_line")
			if err != nil {
				logger.Error("get_range_content: Start line parsing failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_line parameter: %v", err)), nil
			}

			startLine, err := safeUint32(startLineInt)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start line number: %v", err)), nil
			}

			startCharInt, err := request.RequireInt("start_character")
			if err != nil {
				logger.Error("get_range_content: Start character parsing failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_character parameter: %v", err)), nil
			}

			startCharacter, err := safeUint32(startCharInt)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start character position: %v", err)), nil
			}

			endLineInt, err := request.RequireInt("end_line")
			if err != nil {
				logger.Error("get_range_content: End line parsing failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_line parameter: %v", err)), nil
			}

			endLine, err := safeUint32(endLineInt)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end line number: %v", err)), nil
			}

			endCharInt, err := request.RequireInt("end_character")
			if err != nil {
				logger.Error("get_range_content: End character parsing failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_character parameter: %v", err)), nil
			}

			endCharacter, err := safeUint32(endCharInt)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end character position: %v", err)), nil
			}

			// Parse strict parameter (defaults to false)
			strict := request.GetBool("strict", false)

			logger.Info(fmt.Sprintf("GetRangeContent Tool: Parsed URI: %s, Range: %d:%d - %d:%d, Strict: %t",
				uri, startLine, startCharacter, endLine, endCharacter, strict))

			contentInRange, err := getRangeContent(bridge, uri, startLine, startCharacter, endLine, endCharacter, strict)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultText(contentInRange), nil
		}
}

// RegisterRangeTools registers the range-based tools with the MCP server.
func RegisterRangeTools(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(RangeContentTool(bridge))
}
