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
func RangeContentTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_range_content",
			mcp.WithDescription("Get text content from file range. Efficient for specific code blocks. Range parameters (uri, start/end line/char) should be precise, typically from 'lsp__project_analysis' ('definitions' or 'document_symbols' modes). Use 'strict' parameter to control bounds checking behavior."),
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

			logger.Info(fmt.Sprintf("GetRangeContent Tool: Parsed URI: %s, Range: %d:%d - %d:%d, Strict: %t", uri, startLine, startCharacter, endLine, endCharacter, strict))

			filePath := normalizeURI(uri)
			filePath = strings.TrimPrefix(filePath, "file://")

			absPath, err := bridge.IsAllowedDirectory(filePath)
			if err != nil {
				logger.Error("get_range_content: File path parsing failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Invalid file path: %v", err)), nil
			}

			content, err := os.ReadFile(absPath) // #nosec G304
			if err != nil {
				logger.Error("get_range_content: Failed to read file", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to read file %s: %v", filePath, err)), nil
			}

			// Split content into lines. Use "\n" directly in the string
			lines := strings.Split(string(content), "\n")

			// Validate basic line range - these should be hard errors
			linesLen, err := safeUint32(len(lines))
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("File too large: %v", err)), nil
			}
			if startLine >= linesLen || endLine >= linesLen {
				logger.Error("get_range_content: Line range out of file bounds", fmt.Errorf("line range %d-%d out of bounds (file has %d lines)", startLine, endLine, len(lines)))
				return mcp.NewToolResultError(fmt.Sprintf("Line range %d-%d out of bounds (file has %d lines)", startLine, endLine, len(lines))), nil
			}

			// Validate logical range order
			if startLine > endLine || (startLine == endLine && startCharacter > endCharacter) {
				logger.Error("get_range_content: Invalid range order", fmt.Errorf("invalid range order %d:%d - %d:%d", startLine, startCharacter, endLine, endCharacter))
				return mcp.NewToolResultError(fmt.Sprintf("Invalid range order: %d:%d - %d:%d", startLine, startCharacter, endLine, endCharacter)), nil
			}

			// Helper function to handle character bounds based on strictness
			handleCharacterBounds := func(line string, pos uint32, lineNum uint32, posType string) (uint32, error) {
				lineLen, err := safeUint32(len(line))
				if err != nil {
					return 0, fmt.Errorf("line too long: %v", err)
				}
				if pos > lineLen {
					if strict {
						return 0, fmt.Errorf("invalid %s character on line %d: %d (line length: %d)", posType, lineNum, pos, lineLen)
					}
					logger.Info(fmt.Sprintf("get_range_content: Adjusted %s character on line %d from %d to %d (line end)",
						posType, lineNum, pos, lineLen))
					return lineLen, nil
				}
				return pos, nil
			}

			// Extract content based on line and character indices
			var resultLines []string

			if startLine == endLine {
				// Single line range
				line := lines[startLine]

				// Handle character bounds based on strictness
				adjustedStartChar, err := handleCharacterBounds(line, startCharacter, startLine, "start")
				if err != nil {
					logger.Error("get_range_content: Invalid start character on single line", err)
					return mcp.NewToolResultError(err.Error()), nil
				}

				adjustedEndChar, err := handleCharacterBounds(line, endCharacter, startLine, "end")
				if err != nil {
					logger.Error("get_range_content: Invalid end character on single line", err)
					return mcp.NewToolResultError(err.Error()), nil
				}

				// Additional check for character order on same line
				if adjustedStartChar > adjustedEndChar {
					err := fmt.Errorf("invalid character range on line %d: start %d > end %d", startLine, adjustedStartChar, adjustedEndChar)
					logger.Error("get_range_content: Invalid character order on single line", err)
					return mcp.NewToolResultError(err.Error()), nil
				}

				// Handle edge case where start character is at end of line
				lineLen, err := safeUint32(len(line))
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Line too long: %v", err)), nil
				}
				if adjustedStartChar >= lineLen {
					resultLines = append(resultLines, "")
				} else {
					resultLines = append(resultLines, line[adjustedStartChar:adjustedEndChar])
				}
			} else {
				// Multi-line range
				// First line (from start_character to end of line)
				firstLine := lines[startLine]
				adjustedStartChar, err := handleCharacterBounds(firstLine, startCharacter, startLine, "start")
				if err != nil {
					logger.Error("get_range_content: Invalid start character on first line", err)
					return mcp.NewToolResultError(err.Error()), nil
				}

				firstLineLen, err := safeUint32(len(firstLine))
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("First line too long: %v", err)), nil
				}
				if adjustedStartChar >= firstLineLen {
					resultLines = append(resultLines, "")
				} else {
					resultLines = append(resultLines, firstLine[adjustedStartChar:])
				}

				// Middle lines (full lines)
				for i := startLine + 1; i < endLine; i++ {
					if i < linesLen {
						resultLines = append(resultLines, lines[i])
					} else {
						// Should not happen with previous range validation, but as a safeguard
						logger.Warn(fmt.Sprintf("get_range_content: Unexpected line index %d during multi-line extraction", i))
						break
					}
				}

				// Last line (from start of line to end_character)
				lastLine := lines[endLine]
				adjustedEndChar, err := handleCharacterBounds(lastLine, endCharacter-1, endLine, "end")
				if err != nil {
					logger.Error("get_range_content: Invalid end character on last line", err)
					return mcp.NewToolResultError(err.Error()), nil
				}

				resultLines = append(resultLines, lastLine[:adjustedEndChar])
			}

			// Join the extracted lines back into a single string
			contentInRange := strings.Join(resultLines, "\n")

			return mcp.NewToolResultText(contentInRange), nil
		}
}

// RegisterRangeTools registers the range-based tools with the MCP server.
func RegisterRangeTools(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(RangeContentTool(bridge))
}
