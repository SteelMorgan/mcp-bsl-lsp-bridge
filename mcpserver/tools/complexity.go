package tools

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Default BSL LS thresholds above which a method is flagged. These match the
// BSL LS defaults for CyclomaticComplexity (20) and CognitiveComplexity (15).
const (
	cyclomaticThreshold = 20
	cognitiveThreshold  = 15
)

var trailingIntRe = regexp.MustCompile(`(\d+)\s*$`)

// complexityRow aggregates both metrics for a single method.
type complexityRow struct {
	method     string
	line       uint32
	cyclomatic int
	cognitive  int
	hasCyclo   bool
	hasCogn    bool
}

// ComplexityTool reports cyclomatic and cognitive complexity per method.
// BSL LS exposes these only through complexity CodeLens (two-step codeLens +
// codeLens/resolve); the bridge does the protocol dance and returns resolved
// lenses, which we fold into a per-method table.
func ComplexityTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("complexity",
			mcp.WithDescription(`Report cyclomatic and cognitive complexity for every method in a BSL/1C module (via BSL LS complexity CodeLens).

USE WHEN: deciding what to refactor, reviewing a module's risk, or checking that a new/changed method stays within complexity budget before finishing work.

OUTPUT: per-method table with cyclomatic + cognitive complexity, sorted by line, flagging methods above BSL LS default thresholds (cyclomatic > 20, cognitive > 15).`),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the BSL module file"), mcp.Required()),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("complexity: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			provider, ok := bridge.(interface {
				MethodComplexity(uri string) ([]protocol.CodeLens, error)
			})
			if !ok {
				return mcp.NewToolResultError("complexity not supported by this bridge implementation"), nil
			}

			lenses, err := provider.MethodComplexity(uri)
			if err != nil {
				logger.Error("complexity: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("complexity request failed: %v", err)), nil
			}

			return mcp.NewToolResultText(formatComplexity(lenses, uri)), nil
		}
}

// parseComplexityValue extracts the trailing integer from a resolved CodeLens
// title (e.g. "Цикломатическая сложность: 8" -> 8).
func parseComplexityValue(title string) (int, bool) {
	m := trailingIntRe.FindStringSubmatch(title)
	if m == nil {
		return 0, false
	}
	v, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return v, true
}

// lensMeta pulls the method name and metric id out of CodeLens.Data
// ({"methodName": ..., "id": "cyclomaticComplexity"|"cognitiveComplexity", "uri": ...}).
func lensMeta(data any) (methodName, id string) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return "", ""
	}
	if s, ok := m["methodName"].(string); ok {
		methodName = s
	}
	if s, ok := m["id"].(string); ok {
		id = s
	}
	return methodName, id
}

// foldComplexityRows folds the resolved complexity lenses (two per method:
// cyclomatic + cognitive) into per-method rows sorted by line. Shared by the
// complexity tool and module_health.
func foldComplexityRows(lenses []protocol.CodeLens) []*complexityRow {
	rows := map[string]*complexityRow{}
	order := []string{}

	for _, lens := range lenses {
		if lens.Command == nil {
			continue
		}
		methodName, id := lensMeta(lens.Data)
		value, ok := parseComplexityValue(lens.Command.Title)
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d:%s", lens.Range.Start.Line, methodName)
		row := rows[key]
		if row == nil {
			row = &complexityRow{method: methodName, line: lens.Range.Start.Line}
			rows[key] = row
			order = append(order, key)
		}
		switch id {
		case "cyclomaticComplexity":
			row.cyclomatic, row.hasCyclo = value, true
		case "cognitiveComplexity":
			row.cognitive, row.hasCogn = value, true
		}
	}

	list := make([]*complexityRow, 0, len(order))
	for _, k := range order {
		list = append(list, rows[k])
	}
	sort.Slice(list, func(i, j int) bool { return list[i].line < list[j].line })
	return list
}

// overThreshold reports whether a method exceeds either complexity budget.
func (r *complexityRow) overThreshold() bool {
	return (r.hasCyclo && r.cyclomatic > cyclomaticThreshold) || (r.hasCogn && r.cognitive > cognitiveThreshold)
}

func formatComplexity(lenses []protocol.CodeLens, uri string) string {
	list := foldComplexityRows(lenses)

	if len(list) == 0 {
		return "No complexity metrics available. Ensure this is a BSL module with methods and that " +
			"complexity CodeLens is enabled in the BSL LS config " +
			"(codeLens.parameters.cyclomaticComplexity / cognitiveComplexity)."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "COMPLEXITY METRICS: %s\n", uri)
	fmt.Fprintf(&b, "Thresholds: cyclomatic > %d, cognitive > %d (flagged with ⚠)\n\n", cyclomaticThreshold, cognitiveThreshold)
	fmt.Fprintf(&b, "%-40s %5s %5s %5s\n", "Method (line)", "Cyc", "Cog", "")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 60))

	flagged := 0
	maxCyc, maxCog := 0, 0
	for _, r := range list {
		cyc, cog := "-", "-"
		if r.hasCyclo {
			cyc = strconv.Itoa(r.cyclomatic)
			if r.cyclomatic > maxCyc {
				maxCyc = r.cyclomatic
			}
		}
		if r.hasCogn {
			cog = strconv.Itoa(r.cognitive)
			if r.cognitive > maxCog {
				maxCog = r.cognitive
			}
		}
		mark := ""
		if r.overThreshold() {
			mark = "⚠"
			flagged++
		}
		label := fmt.Sprintf("%s (%d)", r.method, r.line+1)
		fmt.Fprintf(&b, "%-40s %5s %5s %5s\n", label, cyc, cog, mark)
	}

	fmt.Fprintf(&b, "\nMethods: %d   Over threshold: %d   Max cyclomatic: %d   Max cognitive: %d\n",
		len(list), flagged, maxCyc, maxCog)
	return b.String()
}

// RegisterComplexityTool registers the complexity tool with the MCP server.
func RegisterComplexityTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ComplexityTool(bridge))
}
