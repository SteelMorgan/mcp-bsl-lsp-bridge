package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

const defaultCompletionLimit = 100

// CompletionTool exposes textDocument/completion. With BSL LS Type System v2 the
// completion response is metadata- and type-aware: after a dot it lists the members
// of the receiver's inferred type (platform types AND configuration objects —
// document attributes, tabular sections, their columns), with each item's signature
// and RETURN TYPE in `detail`. This makes it a discovery channel for "what does this
// object expose here, and what type does each member return" while writing or
// verifying a member-access chain.
func CompletionTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("completion",
			mcp.WithDescription(`Code completion at a position (textDocument/completion), type- and metadata-aware via BSL LS Type System v2.

Most useful right after a dot ('Объект.') — it returns the members available on the receiver's inferred type:
- platform types: methods/properties with signature + return type (e.g. ТаблицаЗначений.Добавить(): СтрокаТаблицыЗначений);
- configuration objects: attributes, TABULAR SECTIONS (табличные части) and their columns of a document/catalog object;
- deprecated members are flagged.

USE WHEN: discovering what a variable/object exposes, finding a document's tabular sections/attributes, or checking the return type of each link in a member chain (a.b.c). For the type of the variable itself use hover; for call parameters use signature_help.`),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithNumber("line", mcp.Description("Line (0-based). For member discovery, the line containing 'Объект.'"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("character", mcp.Description("Character (0-based), the position right after the dot"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("limit", mcp.Description("Max items to return (default 100)"), mcp.Min(1)),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("completion: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}
			line, err := request.RequireInt("line")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("line is required: %v", err)), nil
			}
			character, err := request.RequireInt("character")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("character is required: %v", err)), nil
			}
			limit := request.GetInt("limit", defaultCompletionLimit)

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			lineU, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line: %v", err)), nil
			}
			charU, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character: %v", err)), nil
			}

			list, err := bridge.GetCompletion(uri, lineU, charU)
			if err != nil {
				logger.Error("completion: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("completion request failed: %v", err)), nil
			}

			return mcp.NewToolResultText(formatCompletionItems(list, uri, lineU, charU, limit)), nil
		}
}

// completionKindName maps LSP CompletionItemKind to a short label.
func completionKindName(kind *protocol.CompletionItemKind) string {
	if kind == nil {
		return "item"
	}
	switch *kind {
	case protocol.CompletionItemKindMethod:
		return "method"
	case protocol.CompletionItemKindFunction:
		return "function"
	case protocol.CompletionItemKindConstructor:
		return "constructor"
	case protocol.CompletionItemKindField:
		return "field"
	case protocol.CompletionItemKindVariable:
		return "variable"
	case protocol.CompletionItemKindClass:
		return "class"
	case protocol.CompletionItemKindModule:
		return "module"
	case protocol.CompletionItemKindProperty:
		return "property"
	case protocol.CompletionItemKindEnum:
		return "enum"
	case protocol.CompletionItemKindEnumMember:
		return "enum-member"
	case protocol.CompletionItemKindKeyword:
		return "keyword"
	case protocol.CompletionItemKindValue:
		return "value"
	default:
		return "item"
	}
}

func completionDeprecated(item protocol.CompletionItem) bool {
	if item.Deprecated {
		return true
	}
	for _, t := range item.Tags {
		if t == protocol.CompletionItemTagDeprecated {
			return true
		}
	}
	return false
}

func formatCompletionItems(list *protocol.CompletionList, uri string, line, character uint32, limit int) string {
	if list == nil || len(list.Items) == 0 {
		return fmt.Sprintf("No completions at %s:%d:%d. For member discovery, position right after a dot ('Объект.').", uri, line, character)
	}

	items := list.Items
	total := len(items)

	// Stable order: by kind, then label — keeps members of a type grouped.
	sort.SliceStable(items, func(i, j int) bool {
		ki, kj := completionKindName(items[i].Kind), completionKindName(items[j].Kind)
		if ki != kj {
			return ki < kj
		}
		return items[i].Label < items[j].Label
	})

	truncated := false
	if limit > 0 && len(items) > limit {
		items = items[:limit]
		truncated = true
	}

	var b strings.Builder
	fmt.Fprintf(&b, "COMPLETIONS at %s:%d:%d — %d item(s)%s\n", uri, line, character,
		total, map[bool]string{true: fmt.Sprintf(" (showing %d)", limit)}[truncated])
	if list.IsIncomplete {
		b.WriteString("(list is incomplete — refine the prefix for fewer, sharper results)\n")
	}
	b.WriteString("\n")

	for _, it := range items {
		dep := ""
		if completionDeprecated(it) {
			dep = "  [DEPRECATED]"
		}
		detail := ""
		if it.Detail != "" {
			detail = "  " + it.Detail
		}
		fmt.Fprintf(&b, "  [%-9s] %s%s%s\n", completionKindName(it.Kind), it.Label, detail, dep)
	}

	if truncated {
		fmt.Fprintf(&b, "\n… %d more not shown (raise `limit` or narrow the prefix).\n", total-limit)
	}
	return b.String()
}

// RegisterCompletionTool registers the completion tool with the MCP server.
func RegisterCompletionTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(CompletionTool(bridge))
}
