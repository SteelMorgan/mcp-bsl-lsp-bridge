package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// bslDiagnosticMetaJSON is the offline classification table for BSL LS diagnostics,
// generated from the BSL LS sources (v1.0.0-rc.1). The LSP wire protocol carries only
// the rule name in Diagnostic.code and never the BSL DiagnosticType / DiagnosticTag, so
// security/performance/SQL targeting must be done client-side against this table.
//
//go:embed bsl_diagnostic_meta.json
var bslDiagnosticMetaJSON []byte

// bslDiagnosticMeta describes one BSL LS diagnostic rule.
type bslDiagnosticMeta struct {
	Type     string   `json:"type"`     // CODE_SMELL | ERROR | SECURITY_HOTSPOT | VULNERABILITY
	Severity string   `json:"severity"` // BLOCKER | CRITICAL | MAJOR | MINOR | INFO
	Tags     []string `json:"tags"`     // PERFORMANCE | SQL | SUSPICIOUS | ...
	Quickfix bool     `json:"quickfix"`
	Scopes   []string `json:"scopes"`
}

var bslDiagnosticTable = func() map[string]bslDiagnosticMeta {
	m := map[string]bslDiagnosticMeta{}
	if err := json.Unmarshal(bslDiagnosticMetaJSON, &m); err != nil {
		logger.Error("quality_diagnostics: failed to parse embedded diagnostic metadata", err)
	}
	return m
}()

// qualityCategory classifies a rule. A rule may belong to several categories.
func qualityCategories(meta bslDiagnosticMeta) []string {
	var cats []string
	if meta.Type == "SECURITY_HOTSPOT" || meta.Type == "VULNERABILITY" {
		cats = append(cats, "security")
	}
	for _, t := range meta.Tags {
		switch t {
		case "PERFORMANCE":
			cats = append(cats, "performance")
		case "SQL":
			cats = append(cats, "sql")
		}
	}
	return cats
}

// QualityDiagnosticsTool reports only security / performance / SQL diagnostics for a file.
func QualityDiagnosticsTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("quality_diagnostics",
			mcp.WithDescription(`Targeted security, performance and SQL diagnostics for a BSL/1C file.

Runs BSL LS document diagnostics and keeps ONLY the rules that matter for code quality risk:
- security:    SECURITY_HOTSPOT + VULNERABILITY rules (DisableSafeMode, ExecuteExternalCode, hardcoded secrets, ...)
- performance: rules tagged PERFORMANCE (CreateQueryInCycle / query-in-loop, ...)
- sql:         rules tagged SQL (missing aliases, virtual-table call without parameters, ...)

USE WHEN: security/perf review of a module, or a self-check before finishing a change. For the full diagnostic list use document_diagnostics instead.`),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file to analyze"), mcp.Required()),
			mcp.WithString("categories", mcp.Description("Comma-separated subset of: security, performance, sql. Default: all three.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("quality_diagnostics: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}
			wanted := parseCategories(request.GetString("categories", ""))

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			provider, ok := bridge.(interface {
				GetDocumentDiagnostics(uri string, identifier string, previousResultId string) (*protocol.DocumentDiagnosticReport, error)
			})
			if !ok {
				return mcp.NewToolResultError("document diagnostics not supported by this bridge implementation"), nil
			}

			report, err := provider.GetDocumentDiagnostics(uri, "", "")
			if err != nil {
				logger.Error("quality_diagnostics: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("quality diagnostics request failed: %v", err)), nil
			}

			items := extractDiagnosticItems(report)
			return mcp.NewToolResultText(formatQualityDiagnostics(items, uri, wanted)), nil
		}
}

func parseCategories(raw string) map[string]bool {
	all := map[string]bool{"security": true, "performance": true, "sql": true}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return all
	}
	wanted := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		key := strings.ToLower(strings.TrimSpace(part))
		if all[key] {
			wanted[key] = true
		}
	}
	if len(wanted) == 0 {
		return all
	}
	return wanted
}

// diagnosticCode returns the rule name from Diagnostic.code (BSL uses the string side of Or2).
func diagnosticCode(d protocol.Diagnostic) string {
	if d.Code == nil {
		return ""
	}
	if s, ok := d.Code.Value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", d.Code.Value)
}

// extractDiagnosticItems pulls []Diagnostic out of the Or2 document diagnostic report,
// tolerating both the typed value and the JSON-roundtrip fallback (mirrors document_diagnostics).
func extractDiagnosticItems(report *protocol.DocumentDiagnosticReport) []protocol.Diagnostic {
	if report == nil {
		return nil
	}
	if report.Value != nil {
		switch v := report.Value.(type) {
		case protocol.RelatedFullDocumentDiagnosticReport:
			return v.Items
		case *protocol.RelatedFullDocumentDiagnosticReport:
			return v.Items
		}
	}
	b, err := json.Marshal(report)
	if err != nil {
		return nil
	}
	var full protocol.RelatedFullDocumentDiagnosticReport
	if err := json.Unmarshal(b, &full); err == nil {
		return full.Items
	}
	return nil
}

type qualityRow struct {
	category string
	code     string
	dtype    string
	tags     []string
	quickfix bool
	line     uint32
	col      uint32
	message  string
}

func formatQualityDiagnostics(items []protocol.Diagnostic, uri string, wanted map[string]bool) string {
	byCat := map[string][]qualityRow{}
	unknown := 0

	for _, d := range items {
		code := diagnosticCode(d)
		if code == "" {
			continue
		}
		meta, found := bslDiagnosticTable[code]
		if !found {
			unknown++
			continue
		}
		for _, cat := range qualityCategories(meta) {
			if !wanted[cat] {
				continue
			}
			byCat[cat] = append(byCat[cat], qualityRow{
				category: cat,
				code:     code,
				dtype:    meta.Type,
				tags:     meta.Tags,
				quickfix: meta.Quickfix,
				line:     d.Range.Start.Line,
				col:      d.Range.Start.Character,
				message:  d.Message,
			})
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "QUALITY DIAGNOSTICS: %s\n", uri)
	fmt.Fprintf(&b, "Categories: %s\n", strings.Join(sortedKeys(wanted), ", "))

	total := 0
	for _, cat := range []string{"security", "performance", "sql"} {
		rows := byCat[cat]
		if !wanted[cat] {
			continue
		}
		total += len(rows)
	}

	if total == 0 {
		fmt.Fprintf(&b, "\n✅ No %s issues found.\n", strings.Join(sortedKeys(wanted), "/"))
		return b.String()
	}

	for _, cat := range []string{"security", "performance", "sql"} {
		rows := byCat[cat]
		if len(rows) == 0 {
			continue
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].line < rows[j].line })
		fmt.Fprintf(&b, "\n%s (%d):\n", strings.ToUpper(cat), len(rows))
		fmt.Fprintf(&b, "%s\n", strings.Repeat("=", 50))
		for _, r := range rows {
			qf := ""
			if r.quickfix {
				qf = "  [quick-fix available]"
			}
			tagStr := ""
			if len(r.tags) > 0 {
				tagStr = " {" + strings.Join(r.tags, ",") + "}"
			}
			fmt.Fprintf(&b, "  Line %d:%d  %s [%s]%s%s\n", r.line+1, r.col+1, r.code, r.dtype, tagStr, qf)
			fmt.Fprintf(&b, "      %s\n", r.message)
		}
	}

	fmt.Fprintf(&b, "\nTotal quality issues: %d", total)
	if unknown > 0 {
		fmt.Fprintf(&b, " (%d diagnostics with unrecognized codes ignored)", unknown)
	}
	b.WriteString("\n")
	return b.String()
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// RegisterQualityDiagnosticsTool registers the quality_diagnostics tool with the MCP server.
func RegisterQualityDiagnosticsTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(QualityDiagnosticsTool(bridge))
}
