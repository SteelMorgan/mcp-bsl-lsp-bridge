package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// symbolKindToString converts a SymbolKind to a human-readable string
func symbolKindToString(kind protocol.SymbolKind) string {
	switch kind {
	case protocol.SymbolKindFile:
		return "file"
	case protocol.SymbolKindModule:
		return "module"
	case protocol.SymbolKindNamespace:
		return "namespace"
	case protocol.SymbolKindPackage:
		return "package"
	case protocol.SymbolKindClass:
		return "class"
	case protocol.SymbolKindMethod:
		return "method"
	case protocol.SymbolKindProperty:
		return "property"
	case protocol.SymbolKindField:
		return "field"
	case protocol.SymbolKindConstructor:
		return "constructor"
	case protocol.SymbolKindEnum:
		return "enum"
	case protocol.SymbolKindInterface:
		return "interface"
	case protocol.SymbolKindFunction:
		return "function"
	case protocol.SymbolKindVariable:
		return "variable"
	case protocol.SymbolKindConstant:
		return "constant"
	case protocol.SymbolKindString:
		return "string"
	case protocol.SymbolKindNumber:
		return "number"
	case protocol.SymbolKindBoolean:
		return "boolean"
	case protocol.SymbolKindArray:
		return "array"
	case protocol.SymbolKindObject:
		return "object"
	case protocol.SymbolKindKey:
		return "key"
	case protocol.SymbolKindNull:
		return "null"
	case protocol.SymbolKindEnumMember:
		return "enum member"
	case protocol.SymbolKindStruct:
		return "struct"
	case protocol.SymbolKindEvent:
		return "event"
	case protocol.SymbolKindOperator:
		return "operator"
	case protocol.SymbolKindTypeParameter:
		return "type parameter"
	default:
		return fmt.Sprintf("unknown(%d)", kind)
	}
}

// Helper function to format hover content
func formatHoverContent(contents any) string {
	switch v := contents.(type) {
	case protocol.MarkupContent:
		return fmt.Sprintf("=== HOVER INFORMATION ===\n%s", v.Value)
	case string:
		return fmt.Sprintf("=== HOVER INFORMATION ===\n%s", v)
	case []any:
		var result strings.Builder
		result.WriteString("=== HOVER INFORMATION ===\n")
		for i, item := range v {
			if i > 0 {
				result.WriteString("\n---\n")
			}
			// Try to handle different content types
			if str, ok := item.(string); ok {
				result.WriteString(str)
			} else if markup, ok := item.(protocol.MarkupContent); ok {
				result.WriteString(markup.Value)
			} else {
				result.WriteString(fmt.Sprintf("%v", item))
			}
		}
		return result.String()
	default:
		return fmt.Sprintf("=== HOVER INFORMATION ===\nContent: %v", contents)
	}
}

// Helper function to format signature help
func formatSignatureHelp(sigHelp protocol.SignatureHelpResponse) string {
	var result strings.Builder
	result.WriteString("=== SIGNATURE HELP ===\n")
	
	// For now, just return a basic representation until we can inspect the actual structure
	result.WriteString(fmt.Sprintf("Signature help data: %+v", sigHelp))
	
	return result.String()
}

// Helper function to format diagnostics
func formatDiagnostics(diagnostics []any) string {
	var result strings.Builder
	result.WriteString("=== DIAGNOSTICS ===\n")
	
	if len(diagnostics) == 0 {
		result.WriteString("No diagnostics found")
		return result.String()
	}
	
	// Group diagnostics by severity
	errors := []protocol.Diagnostic{}
	warnings := []protocol.Diagnostic{}
	info := []protocol.Diagnostic{}
	hints := []protocol.Diagnostic{}
	
	for _, diag := range diagnostics {
		if diagnostic, ok := diag.(protocol.Diagnostic); ok {
			// For simplicity, put all diagnostics in warnings for now
			warnings = append(warnings, diagnostic)
		}
	}
	
	formatDiagnosticGroup := func(title string, diags []protocol.Diagnostic) {
		if len(diags) > 0 {
			result.WriteString(fmt.Sprintf("\n%s (%d):\n", title, len(diags)))
			for i, diag := range diags {
				result.WriteString(fmt.Sprintf("%d. %s", 
					i+1, 
					diag.Message))
				if diag.Source != "" {
					result.WriteString(fmt.Sprintf(" [%s]", diag.Source))
				}
				result.WriteString("\n")
			}
		}
	}
	
	formatDiagnosticGroup("ERRORS", errors)
	formatDiagnosticGroup("WARNINGS", warnings)
	formatDiagnosticGroup("INFORMATION", info)
	formatDiagnosticGroup("HINTS", hints)
	
	return result.String()
}

// Helper function to format code actions
func formatCodeActions(actions []any) string {
	var result strings.Builder
	result.WriteString("=== CODE ACTIONS ===\n")
	
	if len(actions) == 0 {
		result.WriteString("No code actions available")
		return result.String()
	}
	
	result.WriteString(fmt.Sprintf("Found %d code actions:\n\n", len(actions)))
	
	for i, action := range actions {
		if codeAction, ok := action.(protocol.CodeAction); ok {
			result.WriteString(fmt.Sprintf("%d. %s", i+1, codeAction.Title))
			if codeAction.Kind != nil {
				result.WriteString(fmt.Sprintf(" (%s)", string(*codeAction.Kind)))
			}
			result.WriteString("\n")
			
			if len(codeAction.Diagnostics) > 0 {
				result.WriteString("   Addresses diagnostics:\n")
				for _, diag := range codeAction.Diagnostics {
					result.WriteString(fmt.Sprintf("   - %s\n", diag.Message))
				}
			}
		} else {
			result.WriteString(fmt.Sprintf("%d. %v\n", i+1, action))
		}
	}
	
	return result.String()
}

// Helper function to format text edits
func formatTextEdits(edits []any) string {
	var result strings.Builder
	result.WriteString("=== DOCUMENT FORMATTING ===\n")
	
	if len(edits) == 0 {
		result.WriteString("Document is already properly formatted")
		return result.String()
	}
	
	result.WriteString(fmt.Sprintf("Found %d formatting edits:\n\n", len(edits)))
	
	for i, edit := range edits {
		if textEdit, ok := edit.(protocol.TextEdit); ok {
			startLine := textEdit.Range.Start.Line + 1  // Convert to 1-based line numbers
			endLine := textEdit.Range.End.Line + 1
			
			if startLine == endLine {
				result.WriteString(fmt.Sprintf("%d. Line %d: Replace text\n", i+1, startLine))
			} else {
				result.WriteString(fmt.Sprintf("%d. Lines %d-%d: Replace text\n", i+1, startLine, endLine))
			}
			
			// Show a preview of the new text (truncated if too long)
			newText := textEdit.NewText
			if len(newText) > 50 {
				newText = newText[:47] + "..."
			}
			if newText != "" {
				result.WriteString(fmt.Sprintf("   New text: %q\n", newText))
			} else {
				result.WriteString("   Action: Delete text\n")
			}
		} else {
			result.WriteString(fmt.Sprintf("%d. %v\n", i+1, edit))
		}
	}
	
	return result.String()
}

// Helper function to format workspace edit for rename
func formatWorkspaceEdit(edit any) string {
	var result strings.Builder
	result.WriteString("=== RENAME PREVIEW ===\n")
	
	// For now, just return a basic representation
	result.WriteString(fmt.Sprintf("Workspace edit: %+v", edit))
	
	return result.String()
}

// Helper function to format implementations
func formatImplementations(implementations []any) string {
	var result strings.Builder
	result.WriteString("=== IMPLEMENTATIONS ===\n")
	
	if len(implementations) == 0 {
		result.WriteString("No implementations found")
		return result.String()
	}
	
	result.WriteString(fmt.Sprintf("Found %d implementations:\n\n", len(implementations)))
	
	for i, impl := range implementations {
		if location, ok := impl.(protocol.Location); ok {
			uri := string(location.Uri)
			filename := filepath.Base(strings.TrimPrefix(uri, "file://"))
			line := location.Range.Start.Line + 1  // Convert to 1-based
			
			result.WriteString(fmt.Sprintf("%d. %s:%d\n", i+1, filename, line))
		} else {
			result.WriteString(fmt.Sprintf("%d. %v\n", i+1, impl))
		}
	}
	
	return result.String()
}

// formatWorkspaceDiagnostics formats workspace diagnostic results for display
func formatWorkspaceDiagnostics(diagnostics any) string {
	if diagnostics == nil {
		return "No workspace diagnostics available"
	}

	reports, ok := diagnostics.([]any)
	if !ok {
		return fmt.Sprintf("Unexpected diagnostics format: %v", diagnostics)
	}

	if len(reports) == 0 {
		return "No workspace diagnostics found"
	}

	var result strings.Builder
	result.WriteString("=== Workspace Diagnostics ===\n\n")

	totalIssues := 0
	errorCount := 0
	warningCount := 0
	infoCount := 0
	hintCount := 0

	for i, report := range reports {
		if workspaceReport, ok := report.(protocol.WorkspaceDiagnosticReport); ok {
			result.WriteString(fmt.Sprintf("Language Server %d Results:\n", i+1))
			
			for _, docReport := range workspaceReport.Items {
				// Try to extract the full document report from the union type
				// This is a simplified approach - we'll format whatever we can extract
				result.WriteString(fmt.Sprintf("\nDocument Report: %+v\n", docReport))
				
				// For now, we'll handle this as a generic interface until we can properly decode the union type
				// TODO: Implement proper union type handling for WorkspaceDocumentDiagnosticReport
			}
			result.WriteString("\n")
		}
	}

	// Summary
	result.WriteString("=== Summary ===\n")
	result.WriteString(fmt.Sprintf("Total Issues: %d\n", totalIssues))
	result.WriteString(fmt.Sprintf("🔴 Errors: %d\n", errorCount))
	result.WriteString(fmt.Sprintf("🟡 Warnings: %d\n", warningCount))
	result.WriteString(fmt.Sprintf("🔵 Info: %d\n", infoCount))
	result.WriteString(fmt.Sprintf("💡 Hints: %d\n", hintCount))

	if totalIssues == 0 {
		result.WriteString("\n✅ No issues found in workspace")
	}

	return result.String()
}

// getDiagnosticSeverityString converts diagnostic severity to string
func getDiagnosticSeverityString(severity *protocol.DiagnosticSeverity) string {
	if severity == nil {
		return "Unknown"
	}
	switch *severity {
	case protocol.DiagnosticSeverityError:
		return "Error"
	case protocol.DiagnosticSeverityWarning:
		return "Warning"
	case protocol.DiagnosticSeverityInformation:
		return "Information"
	case protocol.DiagnosticSeverityHint:
		return "Hint"
	default:
		return "Unknown"
	}
}

// getSeverityIcon returns appropriate icon for diagnostic severity
func getSeverityIcon(severity string) string {
	switch severity {
	case "Error":
		return "🔴"
	case "Warning":
		return "🟡"
	case "Information":
		return "🔵"
	case "Hint":
		return "💡"
	default:
		return "❓"
	}
}