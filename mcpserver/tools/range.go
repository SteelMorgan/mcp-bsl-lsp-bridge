package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GetRangeContentTool defines the lsp__get_range_content tool.
func GetRangeContentTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_range_content",
			mcp.WithDescription("Get the text content within a specified file range."),
			mcp.WithString("uri", mcp.Description("URI to the file (file:// scheme required).")),
			mcp.WithNumber("start_line", mcp.Description("Start line number (0-based).")),
			mcp.WithNumber("start_character", mcp.Description("Start character position (0-based).")),
			mcp.WithNumber("end_line", mcp.Description("End line number (0-based).")),
			mcp.WithNumber("end_character", mcp.Description("End character position (0-based).")),
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

			startLine := uint32(startLineInt)

			startCharInt, err := request.RequireInt("start_character")
			if err != nil {
				logger.Error("get_range_content: Start character parsing failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_character parameter: %v", err)), nil
			}

			startCharacter := uint32(startCharInt)

			endLineInt, err := request.RequireInt("end_line")
			if err != nil {
				logger.Error("get_range_content: End line parsing failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_line parameter: %v", err)), nil
			}

			endLine := uint32(endLineInt)

			endCharInt, err := request.RequireInt("end_character")
			if err != nil {
				logger.Error("get_range_content: End character parsing failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_character parameter: %v", err)), nil
			}

			endCharacter := uint32(endCharInt)

			logger.Info(fmt.Sprintf("GetRangeContent Tool: Parsed URI: %s, Range: %d:%d - %d:%d", uri, startLine, startCharacter, endLine, endCharacter))

			filePath := normalizeURI(uri)
			filePath = strings.TrimPrefix(filePath, "file://")

			content, err := os.ReadFile(filePath)
			if err != nil {
				logger.Error("get_range_content: Failed to read file", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to read file %s: %v", filePath, err)), nil
			}

			// Split content into lines. Use "\n" directly in the string
			lines := strings.Split(string(content), "\n")

			// Validate line and character ranges
			if startLine > endLine || (startLine == endLine && startCharacter > endCharacter) ||
				startLine >= uint32(len(lines)) || endLine >= uint32(len(lines)) {
				logger.Error("get_range_content: Range out of file bounds or invalid", fmt.Errorf("invalid range %d:%d - %d:%d", startLine, startCharacter, endLine, endCharacter))
				return mcp.NewToolResultError(fmt.Sprintf("Range out of file bounds or invalid: %d:%d - %d:%d", startLine, startCharacter, endLine, endCharacter)), nil
			}

			// Extract content based on line and character indices
			var resultLines []string

			if startLine == endLine {
				// Single line range
				line := lines[startLine]
				// Added startCharacter > endCharacter check for completeness
				if startCharacter > uint32(len(line)) || endCharacter > uint32(len(line)) || startCharacter > endCharacter {
					// Corrected logger.Error format string and arguments
					logger.Error("get_range_content: Invalid character range on single line", fmt.Errorf("invalid character range on line %d: %d to %d", startLine, startCharacter, endCharacter))
					// Corrected mcp.NewToolResultError format string and arguments
					return mcp.NewToolResultError(fmt.Sprintf("Invalid character range on line %d: %d to %d", startLine, startCharacter, endCharacter)), nil
				}

				resultLines = append(resultLines, line[startCharacter:endCharacter])
			} else {
				// Multi-line range
				// First line (from start_character to end of line)
				firstLine := lines[startLine]
				if startCharacter > uint32(len(firstLine)) {
					logger.Error("get_range_content: Invalid start character on first line", fmt.Errorf("invalid start character on line %d: %d", startLine, startCharacter))
					return mcp.NewToolResultError(fmt.Sprintf("Invalid start character on line %d: %d", startLine, startCharacter)), nil
				}

				resultLines = append(resultLines, firstLine[startCharacter:])

				// Middle lines (full lines)
				for i := startLine + 1; i < endLine; i++ {
					if i < uint32(len(lines)) {
						resultLines = append(resultLines, lines[i])
					} else {
						// Should not happen with previous range validation, but as a safeguard
						logger.Warn(fmt.Sprintf("get_range_content: Unexpected line index %d during multi-line extraction", i))
						break
					}
				}

				// Last line (from start of line to end_character)
				lastLine := lines[endLine]
				if endCharacter > uint32(len(lastLine)) {
					logger.Error("get_range_content: Invalid end character on last line", fmt.Errorf("invalid end character on line %d: %d", endLine, endCharacter))
					return mcp.NewToolResultError(fmt.Sprintf("Invalid end character on line %d: %d", endLine, endCharacter)), nil
				}

				resultLines = append(resultLines, lastLine[:endCharacter])
			}

			// Join the extracted lines back into a single string
			contentInRange := strings.Join(resultLines, "\n")

			return mcp.NewToolResultText(contentInRange), nil
		}
}

// RegisterRangeTools registers the range-based tools with the MCP server.
func RegisterRangeTools(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(GetRangeContentTool(bridge))
}
