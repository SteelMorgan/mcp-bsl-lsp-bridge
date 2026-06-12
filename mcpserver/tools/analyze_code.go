package tools

import (
	"context"
	"fmt"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

const maxCompletionItems = 100

// RegisterAnalyzeCodeTool registers the analyze_code tool
func RegisterAnalyzeCodeTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(AnalyzeCode(bridge))
}

func AnalyzeCode(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("analyze_code",
			mcp.WithDescription("Get completion suggestions at a position (textDocument/completion). Useful for API discovery: which platform/module methods, properties and predefined values are available at the cursor. Returns the candidate list with type details."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file location to analyze"), mcp.Required()),
			mcp.WithNumber("line", mcp.Description("Line of the file to analyze (0-based)"), mcp.Required()),
			mcp.WithNumber("character", mcp.Description("Character of the line to analyze (0-based)"), mcp.Required()),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("analyze_code: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				logger.Error("analyze_code: Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				logger.Error("analyze_code: Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			lineU, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			characterU, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}

			completion, err := bridge.GetCompletion(uri, lineU, characterU)
			if err != nil {
				logger.Error("analyze_code: completion request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Completion request failed: %v", err)), nil
			}

			logger.Info("analyze_code: completion analyzed",
				fmt.Sprintf("URI: %s, Line: %d, Character: %d", uri, line, character),
			)

			return mcp.NewToolResultText(formatCompletion(completion)), nil
		}
}

func formatCompletion(list *protocol.CompletionList) string {
	if list == nil || len(list.Items) == 0 {
		return "No completion suggestions at this position."
	}

	var b strings.Builder
	total := len(list.Items)
	shown := total
	if shown > maxCompletionItems {
		shown = maxCompletionItems
	}

	fmt.Fprintf(&b, "Completion suggestions: %d (showing %d):\n", total, shown)
	for _, item := range list.Items[:shown] {
		fmt.Fprintf(&b, "  - %s", item.Label)
		if item.Detail != "" {
			fmt.Fprintf(&b, "  : %s", item.Detail)
		}
		b.WriteString("\n")
	}
	if total > shown {
		fmt.Fprintf(&b, "  ... and %d more (refine position to narrow results)\n", total-shown)
	}
	return b.String()
}
