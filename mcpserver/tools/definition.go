package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// DefinitionTool exposes LSP textDocument/definition for a specific (uri,line,character).
// This is lower-level than project_analysis(definitions) and is intended for fast, coordinate-based navigation.
func DefinitionTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("definition",
			mcp.WithDescription(`Get definition location(s) for the symbol at a specific cursor position using LSP textDocument/definition.

USAGE:
- definition: uri="file://path", line=15, character=10
- override language inference: language="bsl"

PARAMETERS: uri (required), line/character (required, 0-based), language (optional)
OUTPUT: One or more target locations (file + range) suitable for get_range_content/navigation`),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithNumber("line", mcp.Description("Line number (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("character", mcp.Description("Character position (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithString("language", mcp.Description("Optional language override (e.g., bsl)")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("definition: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				logger.Error("definition: line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				logger.Error("definition: character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			lineU, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			charU, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}

			// Determine language
			var lang types.Language
			if override := request.GetString("language", ""); override != "" {
				lang = types.Language(strings.ToLower(override))
			} else {
				inferred, langErr := bridge.InferLanguage(uri)
				if langErr != nil || inferred == nil {
					logger.Error("definition: language inference failed", langErr)
					return mcp.NewToolResultError(fmt.Sprintf("failed to infer language for %s: %v (pass language=\"bsl\" to override)", uri, langErr)), nil
				}
				lang = *inferred
			}

			// Normalize URI (important for Docker/session mode path mapping)
			normalizedURI := bridge.NormalizeURIForLSP(uri)

			defs, err := bridge.FindSymbolDefinitions(string(lang), normalizedURI, lineU, charU)
			if err != nil {
				logger.Error("definition: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("definition request failed: %v", err)), nil
			}

			return mcp.NewToolResultText(formatDefinitions(defs)), nil
		}
}

func RegisterDefinitionTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(DefinitionTool(bridge))
}

func formatDefinitions(defs []protocol.Or2[protocol.LocationLink, protocol.Location]) string {
	if len(defs) == 0 {
		return "DEFINITION:\nNo definitions found."
	}

	var b strings.Builder
	b.WriteString("DEFINITION:\n")
	fmt.Fprintf(&b, "Count: %d\n\n", len(defs))

	limit := len(defs)
	if limit > 20 {
		limit = 20
	}

	for i := 0; i < limit; i++ {
		def := defs[i]
		switch v := def.Value.(type) {
		case protocol.Location:
			u := string(v.Uri)
			filename := filepath.Base(strings.TrimPrefix(u, "file://"))
			fmt.Fprintf(&b, "%d. %s:%d:%d-%d:%d\n", i+1,
				filename,
				v.Range.Start.Line, v.Range.Start.Character,
				v.Range.End.Line, v.Range.End.Character,
			)
			fmt.Fprintf(&b, "   URI: %s\n", u)
		case protocol.LocationLink:
			u := string(v.TargetUri)
			filename := filepath.Base(strings.TrimPrefix(u, "file://"))
			fmt.Fprintf(&b, "%d. %s:%d:%d-%d:%d\n", i+1,
				filename,
				v.TargetRange.Start.Line, v.TargetRange.Start.Character,
				v.TargetRange.End.Line, v.TargetRange.End.Character,
			)
			fmt.Fprintf(&b, "   URI: %s\n", u)
		default:
			fmt.Fprintf(&b, "%d. Unsupported definition type: %T\n", i+1, def.Value)
		}
	}

	if len(defs) > limit {
		fmt.Fprintf(&b, "\n(Showing first %d of %d)\n", limit, len(defs))
	}

	return b.String()
}
