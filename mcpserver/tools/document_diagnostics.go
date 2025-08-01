package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

func DocumentDiagnosticsTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("document_diagnostics",
			mcp.WithDescription(`Get diagnostics for a specific document using LSP 3.17+ textDocument/diagnostic method. EXCELLENT for identifying errors, warnings, and issues in individual files - provides more targeted results than workspace diagnostics.

USAGE:
- Basic diagnostics: uri="file://path/to/file.tsx"
- With identifier: uri="file://path", identifier="my-id"
- With result caching: uri="file://path", previous_result_id="abc123"

OUTPUT: Full document diagnostic report with items, related documents, and result caching info`),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file to diagnose"), mcp.Required()),
			mcp.WithString("identifier", mcp.Description("Optional identifier for the diagnostic request")),
			mcp.WithString("previous_result_id", mcp.Description("Optional result ID from previous diagnostic request for caching")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse and validate parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("document_diagnostics: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Optional parameters
			identifier := request.GetString("identifier", "")
			previousResultId := request.GetString("previous_result_id", "")

			// Infer language for debugging
			language, langErr := bridge.InferLanguage(uri)
			if langErr != nil {
				logger.Error("document_diagnostics: Language inference failed", langErr)
				return mcp.NewToolResultError(fmt.Sprintf("failed to infer language for %s: %v", uri, langErr)), nil
			}

			logger.Info(fmt.Sprintf("document_diagnostics: Processing request for %s (language: %s)", uri, string(*language)))

			// Get document diagnostics from bridge
			bridgeWithDiagnostics, ok := bridge.(interface {
				GetDocumentDiagnostics(uri string, identifier string, previousResultId string) (*protocol.DocumentDiagnosticReport, error)
			})
			if !ok {
				return mcp.NewToolResultError("document diagnostics not supported by this bridge implementation"), nil
			}

			report, err := bridgeWithDiagnostics.GetDocumentDiagnostics(uri, identifier, previousResultId)
			if err != nil {
				logger.Error("document_diagnostics: Request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("document diagnostics request failed: %v", err)), nil
			}

			// Format the response
			result := formatDocumentDiagnostics(report, uri)

			logger.Info("document_diagnostics: Successfully processed " + uri)
			return mcp.NewToolResultText(result), nil
		}
}

func formatDocumentDiagnostics(report *protocol.DocumentDiagnosticReport, uri string) string {
	if report == nil {
		return "No diagnostic report available"
	}

	var result strings.Builder
	result.WriteString("DOCUMENT DIAGNOSTICS:\n")
	result.WriteString(fmt.Sprintf("File: %s\n", uri))

	// DocumentDiagnosticReport is Or2[RelatedFullDocumentDiagnosticReport, RelatedUnchangedDocumentDiagnosticReport]
	// Try to extract the actual diagnostics by accessing the underlying data
	// Since Or2 is a union type, we need to handle it carefully

	// Convert to JSON and back to extract the actual diagnostic data
	reportBytes, err := json.Marshal(report)
	if err != nil {
		return fmt.Sprintf("Error parsing diagnostic report: %v", err)
	}

	// Try to parse as RelatedFullDocumentDiagnosticReport first
	var fullReport protocol.RelatedFullDocumentDiagnosticReport
	if err := json.Unmarshal(reportBytes, &fullReport); err == nil && len(fullReport.Items) > 0 {
		return formatFullDiagnosticReport(&fullReport, uri)
	}

	// Try to parse as RelatedUnchangedDocumentDiagnosticReport
	var unchangedReport protocol.RelatedUnchangedDocumentDiagnosticReport
	if err := json.Unmarshal(reportBytes, &unchangedReport); err == nil {
		return formatUnchangedDiagnosticReport(&unchangedReport, uri)
	}

	// If we can't parse it, show basic info
	result.WriteString("Report Type: Document Diagnostic Report\n")
	result.WriteString("LSP 3.17+ document diagnostics received successfully.\n")
	result.WriteString("Raw report data available but type parsing needs refinement.\n")

	return result.String()
}

func formatFullDiagnosticReport(report *protocol.RelatedFullDocumentDiagnosticReport, uri string) string {
	var result strings.Builder

	result.WriteString("Report Type: Full Document Diagnostic Report\n")
	if report.ResultId != "" {
		result.WriteString(fmt.Sprintf("Result ID: %s\n", report.ResultId))
	}

	result.WriteString(fmt.Sprintf("Diagnostics: %d\n\n", len(report.Items)))

	if len(report.Items) == 0 {
		result.WriteString("No issues found in this document.\n")
	} else {
		result.WriteString("ISSUES FOUND:\n")
		result.WriteString(strings.Repeat("=", 50) + "\n\n")

		// Group diagnostics by severity
		errors := make([]protocol.Diagnostic, 0)
		warnings := make([]protocol.Diagnostic, 0)
		infos := make([]protocol.Diagnostic, 0)
		hints := make([]protocol.Diagnostic, 0)

		for _, diagnostic := range report.Items {
			if diagnostic.Severity == nil {
				errors = append(errors, diagnostic) // Default to error if no severity
			} else {
				switch *diagnostic.Severity {
				case 1: // Error
					errors = append(errors, diagnostic)
				case 2: // Warning
					warnings = append(warnings, diagnostic)
				case 3: // Information
					infos = append(infos, diagnostic)
				case 4: // Hint
					hints = append(hints, diagnostic)
				}
			}
		}

		// Format errors
		if len(errors) > 0 {
			result.WriteString(fmt.Sprintf("ERRORS (%d):\n", len(errors)))
			for i, diagnostic := range errors {
				result.WriteString(formatEnhancedDiagnostic(i+1, diagnostic, "ERROR"))
			}
			result.WriteString("\n")
		}

		// Format warnings
		if len(warnings) > 0 {
			result.WriteString(fmt.Sprintf("WARNINGS (%d):\n", len(warnings)))
			for i, diagnostic := range warnings {
				result.WriteString(formatEnhancedDiagnostic(i+1, diagnostic, "WARNING"))
			}
			result.WriteString("\n")
		}

		// Format info messages
		if len(infos) > 0 {
			result.WriteString(fmt.Sprintf("INFORMATION (%d):\n", len(infos)))
			for i, diagnostic := range infos {
				result.WriteString(formatEnhancedDiagnostic(i+1, diagnostic, "INFO"))
			}
			result.WriteString("\n")
		}

		// Format hints
		if len(hints) > 0 {
			result.WriteString(fmt.Sprintf("HINTS (%d):\n", len(hints)))
			for i, diagnostic := range hints {
				result.WriteString(formatEnhancedDiagnostic(i+1, diagnostic, "HINT"))
			}
			result.WriteString("\n")
		}
	}

	// Handle related documents
	if len(report.RelatedDocuments) > 0 {
		result.WriteString("RELATED DOCUMENTS:\n")
		result.WriteString(strings.Repeat("-", 30) + "\n")
		for relatedUri := range report.RelatedDocuments {
			result.WriteString(fmt.Sprintf("- %s\n", relatedUri))
		}
		result.WriteString("\n")
	}

	return result.String()
}

func formatUnchangedDiagnosticReport(report *protocol.RelatedUnchangedDocumentDiagnosticReport, uri string) string {
	var result strings.Builder

	result.WriteString("Report Type: Unchanged Document Diagnostic Report\n")
	result.WriteString(fmt.Sprintf("Result ID: %s\n", report.ResultId))
	result.WriteString("No changes since last diagnostic request - results are unchanged.\n\n")

	// Handle related documents
	if len(report.RelatedDocuments) > 0 {
		result.WriteString("RELATED DOCUMENTS:\n")
		result.WriteString(strings.Repeat("-", 30) + "\n")
		for relatedUri := range report.RelatedDocuments {
			result.WriteString(fmt.Sprintf("- %s\n", relatedUri))
		}
	}

	return result.String()
}

func formatEnhancedDiagnostic(index int, diagnostic protocol.Diagnostic, severityStr string) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("%d. %s\n", index, diagnostic.Message))
	result.WriteString(fmt.Sprintf("   Location: Line %d, Column %d-%d\n",
		diagnostic.Range.Start.Line+1,
		diagnostic.Range.Start.Character+1,
		diagnostic.Range.End.Character+1))

	// Show diagnostic code if available
	if diagnostic.Code != nil {
		result.WriteString(fmt.Sprintf("   Code: %v\n", *diagnostic.Code))
	}

	// Show source (e.g., 'typescript', 'biome', etc.)
	if diagnostic.Source != "" {
		result.WriteString(fmt.Sprintf("   Source: %s\n", diagnostic.Source))
	}

	// Show diagnostic tags
	if len(diagnostic.Tags) > 0 {
		var tags []string
		for _, tag := range diagnostic.Tags {
			switch tag {
			case 1:
				tags = append(tags, "Unnecessary")
			case 2:
				tags = append(tags, "Deprecated")
			default:
				tags = append(tags, fmt.Sprintf("Tag-%d", tag))
			}
		}
		result.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(tags, ", ")))
	}

	// Show code description link if available
	if diagnostic.CodeDescription != nil && diagnostic.CodeDescription.Href != "" {
		result.WriteString(fmt.Sprintf("   Reference: %s\n", diagnostic.CodeDescription.Href))
	}

	// Show related information if available
	if len(diagnostic.RelatedInformation) > 0 {
		result.WriteString("   Related Information:\n")
		for _, info := range diagnostic.RelatedInformation {
			result.WriteString(fmt.Sprintf("      - %s (Line %d)\n",
				info.Message,
				info.Location.Range.Start.Line+1))
		}
	}

	result.WriteString("\n")
	return result.String()
}

func RegisterDocumentDiagnosticsTool(s ToolServer, bridge interfaces.BridgeInterface) {
	tool, handler := DocumentDiagnosticsTool(bridge)
	s.AddTool(tool, handler)
}
