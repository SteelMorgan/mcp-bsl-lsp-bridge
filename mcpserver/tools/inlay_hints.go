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

func InlayHintsTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("inlay_hints",
			mcp.WithDescription("Get inlay hints for a range (textDocument/inlayHint). Reveals parameter names at call sites (e.g. Новый Массив(ВместимостьИлиРазмерности: 10)) and inferred types - useful for understanding dense code without opening each definition."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithNumber("start_line", mcp.Description("Start line (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("end_line", mcp.Description("End line (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("start_character", mcp.Description("Start character (0-based), default 0"), mcp.Min(0)),
			mcp.WithNumber("end_character", mcp.Description("End character (0-based), default 0 (start of end_line)"), mcp.Min(0)),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("inlay_hints: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			startLine, err := request.RequireInt("start_line")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("start_line is required: %v", err)), nil
			}
			endLine, err := request.RequireInt("end_line")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("end_line is required: %v", err)), nil
			}
			startCharacter := request.GetInt("start_character", 0)
			endCharacter := request.GetInt("end_character", 0)

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			startLineU, err := safeUint32(startLine)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_line: %v", err)), nil
			}
			startCharU, err := safeUint32(startCharacter)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_character: %v", err)), nil
			}
			endLineU, err := safeUint32(endLine)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_line: %v", err)), nil
			}
			endCharU, err := safeUint32(endCharacter)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_character: %v", err)), nil
			}

			hints, err := bridge.InlayHint(uri, startLineU, startCharU, endLineU, endCharU)
			if err != nil {
				logger.Error("inlay_hints: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Inlay hint request failed: %v", err)), nil
			}

			if len(hints) == 0 {
				return mcp.NewToolResultText("No inlay hints in the specified range."), nil
			}

			return mcp.NewToolResultText(formatInlayHints(hints)), nil
		}
}

func formatInlayHints(hints []protocol.InlayHint) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d inlay hint(s):\n", len(hints))
	for _, h := range hints {
		label := inlayLabelString(h.Label)
		kind := inlayKindString(h.Kind)
		fmt.Fprintf(&b, "  - %d:%d [%s] %s\n", h.Position.Line, h.Position.Character, kind, label)
	}
	return b.String()
}

func inlayLabelString(label protocol.Or2[string, []protocol.InlayHintLabelPart]) string {
	switch v := label.Value.(type) {
	case string:
		return v
	case []protocol.InlayHintLabelPart:
		parts := make([]string, 0, len(v))
		for _, p := range v {
			parts = append(parts, p.Value)
		}
		return strings.Join(parts, "")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func inlayKindString(kind *protocol.InlayHintKind) string {
	if kind == nil {
		return "hint"
	}
	switch *kind {
	case protocol.InlayHintKindType:
		return "type"
	case protocol.InlayHintKindParameter:
		return "parameter"
	default:
		return "hint"
	}
}

// RegisterInlayHintsTool registers the inlay_hints tool with the MCP server.
func RegisterInlayHintsTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(InlayHintsTool(bridge))
}
