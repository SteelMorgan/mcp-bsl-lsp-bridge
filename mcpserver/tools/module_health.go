package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Risk weights for the combined module_health score. Security issues dominate,
// then SQL/performance, then complexity overage above the BSL LS thresholds.
const (
	weightSecurity    = 10
	weightSQL         = 4
	weightPerformance = 4
)

// healthRow aggregates per-method complexity and quality issue counts.
type healthRow struct {
	method      string
	line        uint32
	cyclomatic  int
	cognitive   int
	hasCyclo    bool
	hasCogn     bool
	overCyclo   bool
	overCogn    bool
	security    int
	performance int
	sql         int
}

// score is the combined refactor-priority score; 0 means clean and within budget.
func (h *healthRow) score() int {
	s := weightSecurity*h.security + weightSQL*h.sql + weightPerformance*h.performance
	if h.overCyclo {
		s += h.cyclomatic
	}
	if h.overCogn {
		s += h.cognitive
	}
	return s
}

// ModuleHealthTool is a deterministic combo: it runs complexity (CodeLens) and
// quality diagnostics over one module in a single call, attributes each quality
// issue to its enclosing method, and returns a ranked "what to refactor first"
// view. It replaces a complexity + quality_diagnostics call pair plus the manual
// merge. For a single method's metrics use `complexity`; for one risk category
// use `quality_diagnostics`.
func ModuleHealthTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("module_health",
			mcp.WithDescription(`Combined per-module health report for a BSL/1C module: cyclomatic + cognitive complexity merged with security/performance/SQL diagnostics, attributed per method and ranked by refactor priority.

Runs two BSL LS analyses in one call (complexity CodeLens + document diagnostics) and folds them into:
- TOP REFACTOR TARGETS: methods scored by security(×10) + sql/perf(×4) + complexity over threshold, highest first;
- a per-method complexity table (cyclomatic > 20 / cognitive > 15 flagged);
- security/performance/SQL issue totals.

USE WHEN: triaging a whole module ("what to fix first"), risk-reviewing a change, or a pre-finish health gate. For a single method's metrics use complexity; for one risk category use quality_diagnostics.`),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the BSL module file"), mcp.Required()),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("module_health: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			cProvider, ok := bridge.(interface {
				MethodComplexity(uri string) ([]protocol.CodeLens, error)
			})
			if !ok {
				return mcp.NewToolResultError("module_health: complexity not supported by this bridge implementation"), nil
			}
			dProvider, ok := bridge.(interface {
				GetDocumentDiagnostics(uri string, identifier string, previousResultId string) (*protocol.DocumentDiagnosticReport, error)
			})
			if !ok {
				return mcp.NewToolResultError("module_health: document diagnostics not supported by this bridge implementation"), nil
			}

			lenses, err := cProvider.MethodComplexity(uri)
			if err != nil {
				logger.Error("module_health: complexity request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("complexity request failed: %v", err)), nil
			}
			report, err := dProvider.GetDocumentDiagnostics(uri, "", "")
			if err != nil {
				logger.Error("module_health: diagnostics request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("diagnostics request failed: %v", err)), nil
			}

			return mcp.NewToolResultText(formatModuleHealth(lenses, report, uri)), nil
		}
}

// qualityCounts holds attributed-issue counters (per method or module-level).
type qualityCounts struct{ security, performance, sql int }

func formatModuleHealth(lenses []protocol.CodeLens, report *protocol.DocumentDiagnosticReport, uri string) string {
	complexity := foldComplexityRows(lenses) // sorted by line asc

	rows := make([]*healthRow, 0, len(complexity))
	for _, c := range complexity {
		rows = append(rows, &healthRow{
			method:     c.method,
			line:       c.line,
			cyclomatic: c.cyclomatic,
			cognitive:  c.cognitive,
			hasCyclo:   c.hasCyclo,
			hasCogn:    c.hasCogn,
			overCyclo:  c.hasCyclo && c.cyclomatic > cyclomaticThreshold,
			overCogn:   c.hasCogn && c.cognitive > cognitiveThreshold,
		})
	}

	// Attribute each quality issue to its enclosing method (greatest start line
	// <= issue line). Issues above the first method fall to module-level.
	var moduleLevel qualityCounts
	totals := qualityCounts{}
	for _, d := range extractDiagnosticItems(report) {
		code := diagnosticCode(d)
		if code == "" {
			continue
		}
		meta, found := bslDiagnosticTable[code]
		if !found {
			continue
		}
		target := enclosingMethod(rows, d.Range.Start.Line)
		for _, cat := range qualityCategories(meta) {
			bump(&totals, cat)
			if target == nil {
				bump(&moduleLevel, cat)
				continue
			}
			switch cat {
			case "security":
				target.security++
			case "performance":
				target.performance++
			case "sql":
				target.sql++
			}
		}
	}

	if len(rows) == 0 && totals == (qualityCounts{}) {
		return "No health signals available. Ensure this is a BSL module and that complexity CodeLens is " +
			"enabled in the BSL LS config (codeLens.parameters.cyclomaticComplexity / cognitiveComplexity)."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "MODULE HEALTH: %s\n", uri)
	fmt.Fprintf(&b, "Thresholds: cyclomatic > %d, cognitive > %d. Risk weights: security ×%d, sql ×%d, performance ×%d + complexity overage.\n",
		cyclomaticThreshold, cognitiveThreshold, weightSecurity, weightSQL, weightPerformance)
	flagged := 0
	for _, r := range rows {
		if r.overCyclo || r.overCogn {
			flagged++
		}
	}
	fmt.Fprintf(&b, "Methods: %d (over threshold: %d). Issues — security: %d, performance: %d, sql: %d.\n",
		len(rows), flagged, totals.security, totals.performance, totals.sql)

	// TOP REFACTOR TARGETS: scored rows, highest first.
	ranked := make([]*healthRow, 0, len(rows))
	for _, r := range rows {
		if r.score() > 0 {
			ranked = append(ranked, r)
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score() != ranked[j].score() {
			return ranked[i].score() > ranked[j].score()
		}
		return ranked[i].line < ranked[j].line
	})

	b.WriteString("\nTOP REFACTOR TARGETS (ranked):\n")
	if len(ranked) == 0 {
		b.WriteString("  ✅ none — no method is over threshold or carries security/perf/sql issues.\n")
	}
	for i, r := range ranked {
		fmt.Fprintf(&b, "  %d. %s (line %d)  score %d  [cyc %s, cog %s%s]\n",
			i+1, r.method, r.line+1, r.score(),
			metricStr(r.cyclomatic, r.hasCyclo, r.overCyclo),
			metricStr(r.cognitive, r.hasCogn, r.overCogn),
			issueStr(r.security, r.performance, r.sql))
	}
	if moduleLevel != (qualityCounts{}) {
		fmt.Fprintf(&b, "  module-level (outside any method): %s\n",
			strings.TrimPrefix(issueStr(moduleLevel.security, moduleLevel.performance, moduleLevel.sql), ", "))
	}

	// Full complexity table.
	b.WriteString("\nCOMPLEXITY (per method):\n")
	fmt.Fprintf(&b, "  %-40s %5s %5s\n", "Method (line)", "Cyc", "Cog")
	fmt.Fprintf(&b, "  %s\n", strings.Repeat("-", 54))
	for _, r := range rows {
		label := fmt.Sprintf("%s (%d)", r.method, r.line+1)
		mark := ""
		if r.overCyclo || r.overCogn {
			mark = " ⚠"
		}
		fmt.Fprintf(&b, "  %-40s %5s %5s%s\n", label,
			metricRaw(r.cyclomatic, r.hasCyclo), metricRaw(r.cognitive, r.hasCogn), mark)
	}

	return b.String()
}

// enclosingMethod returns the method whose declaration line is the greatest one
// <= the given line (rows are sorted by line asc), or nil if the line precedes
// the first method.
func enclosingMethod(rows []*healthRow, line uint32) *healthRow {
	var found *healthRow
	for _, r := range rows {
		if r.line <= line {
			found = r
		} else {
			break
		}
	}
	return found
}

func bump(q *qualityCounts, cat string) {
	switch cat {
	case "security":
		q.security++
	case "performance":
		q.performance++
	case "sql":
		q.sql++
	}
}

func metricStr(v int, has, over bool) string {
	if !has {
		return "-"
	}
	if over {
		return strconv.Itoa(v) + "⚠"
	}
	return strconv.Itoa(v)
}

func metricRaw(v int, has bool) string {
	if !has {
		return "-"
	}
	return strconv.Itoa(v)
}

// issueStr renders the non-zero issue counts as ", security:1, sql:2" (leading
// separator so it appends cleanly; trimmed by callers when standalone).
func issueStr(security, performance, sql int) string {
	var parts []string
	if security > 0 {
		parts = append(parts, fmt.Sprintf("security:%d", security))
	}
	if performance > 0 {
		parts = append(parts, fmt.Sprintf("performance:%d", performance))
	}
	if sql > 0 {
		parts = append(parts, fmt.Sprintf("sql:%d", sql))
	}
	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

// RegisterModuleHealthTool registers the module_health combo tool.
func RegisterModuleHealthTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ModuleHealthTool(bridge))
}
