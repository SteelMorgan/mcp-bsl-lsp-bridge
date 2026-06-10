package tools

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// SymbolImpactTool is a deterministic combo for change-impact analysis: from a
// single symbol position it gathers incoming callers (call hierarchy) AND all
// reference sites, then classifies each caller by 1C module type (derived from
// the URI) so you can see where the pressure comes from (UI form vs manager vs
// background). It replaces a prepare+incoming call-hierarchy chain plus a
// separate references call plus the manual classification.
//
// Scope note: call hierarchy shows only direct CALL edges. A method can also be
// reached by event subscriptions, form handlers, scheduled jobs and extension
// &Вместо/&Перед/&После — those are declarative and NOT visible here. References
// resolve real symbols, so they exclude comment/string matches, but for the same
// reason they do NOT catch string-literal type refs ("ДокументСсылка.X") or
// metadata paths inside query text — complement those with grep.
func SymbolImpactTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("symbol_impact",
			mcp.WithDescription(`Change-impact analysis for a symbol: incoming callers (call hierarchy) + all reference sites + per-caller 1C module-type classification, in one call.

OUTPUT:
- INCOMING CALLERS grouped by module type (CommonModule / FormModule / ManagerModule / ObjectModule / ...), so you see whether a change is pulled by UI, server or background code;
- REFERENCES: every site where the symbol is used (semantic — no comment/string false positives);
- an IMPACT SUMMARY.

USE WHEN: estimating the blast radius of renaming/changing a procedure or function. Caveats: call hierarchy misses non-call triggers (event subscriptions, form handlers, scheduled jobs, extension overrides); references miss string-literal/query-text usages — cross-check with grep. For one-directional traversal or depth>1 use call_graph; for raw references use symbol_explore.`),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithNumber("line", mcp.Description("Line number (0-based) of the symbol"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("character", mcp.Description("Character position (0-based) of the symbol"), mcp.Required(), mcp.Min(0)),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("symbol_impact: URI parsing failed", err)
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

			normalizedURI := bridge.NormalizeURIForLSP(uri)
			language, err := bridge.InferLanguage(uri)
			if err != nil {
				logger.Error("symbol_impact: language inference failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to infer language from URI: %v", err)), nil
			}

			items, err := bridge.PrepareCallHierarchy(normalizedURI, lineU, charU)
			if err != nil {
				logger.Error("symbol_impact: prepare call hierarchy failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to prepare call hierarchy: %v", err)), nil
			}

			var callers []protocol.CallHierarchyIncomingCall
			var callErrs []error
			for _, item := range items {
				in, err := bridge.IncomingCalls(item)
				if err != nil {
					callErrs = append(callErrs, fmt.Errorf("incoming for %s: %w", item.Name, err))
					continue
				}
				callers = append(callers, in...)
			}

			refs, refErr := bridge.FindSymbolReferences(string(*language), normalizedURI, lineU, charU, false)
			if refErr != nil {
				// References are best-effort; report but still show callers.
				callErrs = append(callErrs, fmt.Errorf("references: %w", refErr))
			}

			symbolName := ""
			if len(items) > 0 {
				symbolName = items[0].Name
			}

			return mcp.NewToolResultText(formatSymbolImpact(symbolName, uri, line, character, items, callers, refs, callErrs)), nil
		}
}

// classifyModuleType maps a BSL module URI to its 1C module type, derived from
// the file name and the designer/EDT dump path layout. Returns a concise label
// used to show where call pressure originates.
func classifyModuleType(uri string) string {
	p := strings.ToLower(uri)
	base := path.Base(strings.TrimSuffix(p, "/"))

	switch {
	case strings.Contains(p, "/forms/") && (base == "module.bsl" || strings.HasSuffix(base, "form/module.bsl")):
		return "FormModule"
	case base == "managermodule.bsl":
		return "ManagerModule"
	case base == "objectmodule.bsl":
		return "ObjectModule"
	case base == "recordsetmodule.bsl":
		return "RecordSetModule"
	case base == "valuemanagermodule.bsl":
		return "ValueManagerModule"
	case base == "commandmodule.bsl":
		return "CommandModule"
	case strings.Contains(p, "/commonmodules/"):
		return "CommonModule"
	case strings.Contains(p, "/forms/"):
		return "FormModule"
	}

	// Fall back to the metadata class from the first meaningful path segment.
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "documents", "catalogs", "informationregisters", "accumulationregisters",
			"accountingregisters", "businessprocesses", "tasks",
			"dataprocessors", "reports", "enums", "exchangeplans", "chartsofcharacteristictypes":
			return capitalize(seg) + "Module"
		}
	}
	return "Module"
}

// capitalize upper-cases the first ASCII byte of s (metadata class segments are ASCII).
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func formatSymbolImpact(
	name, uri string, line, character int,
	items []protocol.CallHierarchyItem,
	callers []protocol.CallHierarchyIncomingCall,
	refs []protocol.Location,
	errs []error,
) string {
	var b strings.Builder
	label := name
	if label == "" {
		label = "(symbol)"
	}
	fmt.Fprintf(&b, "SYMBOL IMPACT: %s\n", label)
	fmt.Fprintf(&b, "Position: %s:%d:%d   resolved items: %d\n", uri, line, character, len(items))

	if len(items) == 0 {
		b.WriteString("\nNo call-hierarchy symbol at this position. Point at the procedure/function name (check the character offset).\n")
		if len(refs) > 0 {
			writeReferences(&b, refs)
		}
		return b.String()
	}

	// Group callers by module type.
	byType := map[string]int{}
	type callerRow struct {
		name, uri, mtype string
		ranges           int
	}
	rows := make([]callerRow, 0, len(callers))
	for _, c := range callers {
		mt := classifyModuleType(string(c.From.Uri))
		byType[mt]++
		rows = append(rows, callerRow{
			name:   c.From.Name,
			uri:    string(c.From.Uri),
			mtype:  mt,
			ranges: len(c.FromRanges),
		})
	}

	fmt.Fprintf(&b, "\nINCOMING CALLERS (%d):\n", len(callers))
	if len(callers) == 0 {
		b.WriteString("  none (direct calls). NOTE: triggers — event subscriptions, form handlers, scheduled jobs, extension &Вместо/&Перед/&После — are not shown here.\n")
	} else {
		b.WriteString("  by module type: ")
		b.WriteString(formatTypeHistogram(byType))
		b.WriteString("\n")
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].mtype != rows[j].mtype {
				return rows[i].mtype < rows[j].mtype
			}
			return rows[i].name < rows[j].name
		})
		for _, r := range rows {
			fmt.Fprintf(&b, "  - %-30s [%s]  %s  (call sites: %d)\n", r.name, r.mtype, r.uri, r.ranges)
		}
	}

	writeReferences(&b, refs)

	// Impact summary.
	fmt.Fprintf(&b, "\nIMPACT SUMMARY: %d direct caller(s) across %d module type(s), %d reference site(s).\n",
		len(callers), len(byType), len(refs))
	b.WriteString("Blast-radius caveats: add non-call triggers (subscriptions/form handlers/jobs/extension overrides — not in call hierarchy) and string-literal/query-text usages (not in references) via grep.\n")

	if len(errs) > 0 {
		fmt.Fprintf(&b, "\nPartial results — %d error(s):\n", len(errs))
		for i, e := range errs {
			fmt.Fprintf(&b, "  %d. %v\n", i+1, e)
		}
	}
	return b.String()
}

func writeReferences(b *strings.Builder, refs []protocol.Location) {
	fmt.Fprintf(b, "\nREFERENCES (%d, semantic — excludes comments/strings):\n", len(refs))
	if len(refs) == 0 {
		b.WriteString("  none.\n")
		return
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Uri != refs[j].Uri {
			return refs[i].Uri < refs[j].Uri
		}
		return refs[i].Range.Start.Line < refs[j].Range.Start.Line
	})
	for _, r := range refs {
		fmt.Fprintf(b, "  - %s:%d:%d\n", r.Uri, r.Range.Start.Line+1, r.Range.Start.Character+1)
	}
}

func formatTypeHistogram(byType map[string]int) string {
	keys := make([]string, 0, len(byType))
	for k := range byType {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", k, byType[k]))
	}
	return strings.Join(parts, ", ")
}

// RegisterSymbolImpactTool registers the symbol_impact combo tool.
func RegisterSymbolImpactTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(SymbolImpactTool(bridge))
}
