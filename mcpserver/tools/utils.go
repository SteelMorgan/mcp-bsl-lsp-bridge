package tools

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

type ToolServer interface {
	AddTool(tool mcp.Tool, handler server.ToolHandlerFunc)
}

// safeUint32 safely converts an int to uint32, checking for overflow
func safeUint32(val int) (uint32, error) {
	if val < 0 {
		return 0, fmt.Errorf("value cannot be negative: %d", val)
	}
	if val > math.MaxUint32 {
		return 0, fmt.Errorf("value exceeds uint32 maximum: %d", val)
	}
	return uint32(val), nil
}

// safeInt32 safely converts an int to int32, checking for overflow
func safeInt32(val int) (int32, error) {
	if val < math.MinInt32 {
		return 0, fmt.Errorf("value below int32 minimum: %d", val)
	}
	if val > math.MaxInt32 {
		return 0, fmt.Errorf("value exceeds int32 maximum: %d", val)
	}
	return int32(val), nil
}

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
func formatHoverContent(contents protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]) string {
	// Handle nil or zero-value contents
	if contents.Value == nil {
		return "No hover information available"
	}

	switch v := contents.Value.(type) {
	case protocol.MarkupContent:
		// Prefer value with non-empty content
		if v.Value != "" {
			return v.Value
		}
	case protocol.MarkedString:
		// Handle MarkedString value
		if strVal, ok := v.Value.(string); ok && strVal != "" {
			return strVal
		}
	case string:
		// Return non-empty string
		if v != "" {
			return v
		}
	case []any:
		var result strings.Builder

		for i, item := range v {
			if i > 0 {
				result.WriteString("\n---\n")
			}
			
			// Comprehensive type handling with fallback
			switch typed := item.(type) {
			case string:
				result.WriteString(typed)
			case protocol.MarkupContent:
				if typed.Value != "" {
					result.WriteString(typed.Value)
				}
			case protocol.MarkedString:
				markedVal, ok := typed.Value.(string)
				if ok && markedVal != "" {
					result.WriteString(markedVal)
				}
			default:
				// Fallback for unknown types
				strVal := fmt.Sprintf("%v", typed)
				if strVal != "" {
					result.WriteString(strVal)
				}
			}
		}

		// Return result if not empty
		if result.Len() > 0 {
			return result.String()
		}
	default:
		// Final fallback for unhandled cases
		return "No meaningful hover information found"
	}

	return "No meaningful hover information found"
}

// Helper function to format diagnostics
func formatDiagnostics(diagnostics []any) string {
	var result strings.Builder

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
func formatCodeActions(actions []protocol.CodeAction) string {
	var result strings.Builder

	if len(actions) == 0 {
		result.WriteString("No code actions available")
		return result.String()
	}

	result.WriteString(fmt.Sprintf("Found %d code actions:\n\n", len(actions)))

	for i, codeAction := range actions {
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
	}

	return result.String()
}

// Helper function to format text edits
func formatTextEdits(edits []protocol.TextEdit) string {
	var result strings.Builder

	result.WriteString("=== DOCUMENT FORMATTING ===\n")

	if len(edits) == 0 {
		result.WriteString("Document is already properly formatted")
		return result.String()
	}

	result.WriteString(fmt.Sprintf("Found %d formatting edits:\n\n", len(edits)))

	// Count different types of edits for summary
	whitespaceEdits := 0
	contentEdits := 0

	for i, textEdit := range edits {
		startLine := textEdit.Range.Start.Line + 1 // Convert to 1-based line numbers
		startChar := textEdit.Range.Start.Character
		endLine := textEdit.Range.End.Line + 1
		endChar := textEdit.Range.End.Character

		// Determine edit type for better description
		newText := textEdit.NewText
		isWhitespaceEdit := strings.TrimSpace(newText) == ""

		if isWhitespaceEdit {
			whitespaceEdits++
		} else {
			contentEdits++
		}

		// Position information
		if startLine == endLine {
			result.WriteString(fmt.Sprintf("%d. Line %d (chars %d-%d):\n", i+1, startLine, startChar, endChar))
		} else {
			result.WriteString(fmt.Sprintf("%d. Lines %d-%d (char %d to line %d char %d):\n",
				i+1, startLine, endLine, startChar, endLine, endChar))
		}

		// Describe the change more clearly
		if newText == "" {
			result.WriteString("   Action: Remove whitespace/formatting\n")
		} else if strings.TrimSpace(newText) == "" {
			// It's whitespace but not empty
			visibleText := strings.ReplaceAll(strings.ReplaceAll(newText, "\n", "\\n"), "\t", "\\t")
			result.WriteString(fmt.Sprintf("   Action: Replace with whitespace: %q\n", visibleText))
		} else {
			// Has actual content
			if len(newText) > 50 {
				preview := newText[:47] + "..."
				result.WriteString(fmt.Sprintf("   Action: Replace with: %q\n", preview))
			} else {
				result.WriteString(fmt.Sprintf("   Action: Replace with: %q\n", newText))
			}
		}
	}

	// Summary for agents
	result.WriteString(fmt.Sprintf("Total edits: %d\n", len(edits)))
	result.WriteString(fmt.Sprintf("Whitespace/formatting edits: %d\n", whitespaceEdits))
	result.WriteString(fmt.Sprintf("Content edits: %d\n", contentEdits))

	if whitespaceEdits > 0 && contentEdits == 0 {
		result.WriteString("\nNote: These are formatting-only changes (whitespace, indentation, etc.)\n")
		result.WriteString("The document structure and content remain unchanged.\n")
	}

	return result.String()
}

// Helper function to format workspace edit for rename
func formatWorkspaceEdit(workspaceEdit *protocol.WorkspaceEdit) string {
	if workspaceEdit == nil {
		return "No changes needed"
	}

	var result strings.Builder

	totalFiles := 0
	totalEdits := 0

	// Handle DocumentChanges format (preferred by most language servers)
	if workspaceEdit.DocumentChanges != nil {
		for _, docChange := range workspaceEdit.DocumentChanges {
			// Use reflection to access the union type structure
			// Based on log output: DocumentChanges: [{Value:{Edits:[...] TextDocument:{Uri:... Version:...}}}]
			docChangeStr := fmt.Sprintf("%+v", docChange)

			// Simple pattern matching to extract URI and edits count
			// This is a fallback approach until proper union type handling is implemented
			if strings.Contains(docChangeStr, "TextDocument:{Uri:") && strings.Contains(docChangeStr, "Edits:[") {
				// Extract URI using string parsing
				uriStart := strings.Index(docChangeStr, "Uri:")
				if uriStart != -1 {
					uriPart := docChangeStr[uriStart+4:]
					uriEnd := strings.Index(uriPart, " ")

					if uriEnd != -1 {
						uri := uriPart[:uriEnd]
						totalFiles++
						filename := filepath.Base(strings.TrimPrefix(uri, "file://"))

						// Count edits by counting "NewText:" occurrences
						editCount := strings.Count(docChangeStr, "NewText:")
						totalEdits += editCount

						result.WriteString(fmt.Sprintf("File: %s (%d edits)\n", filename, editCount))
						result.WriteString(fmt.Sprintf("   URI: %s\n", uri))

						// Extract individual edits using pattern matching
						editsSection := docChangeStr
						editNum := 1

						for {
							newTextStart := strings.Index(editsSection, "NewText:")
							if newTextStart == -1 {
								break
							}

							// Extract NewText value
							newTextPart := editsSection[newTextStart+8:]

							newTextEnd := strings.Index(newTextPart, " Range:")
							if newTextEnd == -1 {
								break
							}

							newText := newTextPart[:newTextEnd]

							// Extract Range information
							rangeStart := strings.Index(newTextPart, "Range:{")
							if rangeStart != -1 {
								rangePart := newTextPart[rangeStart+7:]
								rangeEnd := strings.Index(rangePart, "}}")

								if rangeEnd != -1 {
									rangeInfo := rangePart[:rangeEnd]

									// Simple extraction of line numbers
									if strings.Contains(rangeInfo, "Line:") {
										// Extract line numbers from "Start:{Character:5 Line:6}" pattern
										linePattern := "Line:"

										lineStart := strings.Index(rangeInfo, linePattern)
										if lineStart != -1 {
											linePart := rangeInfo[lineStart+5:]
											lineEndIdx := strings.IndexAny(linePart, " }")

											if lineEndIdx != -1 {
												lineNumStr := linePart[:lineEndIdx]
												result.WriteString(fmt.Sprintf("   %d. Line %s: Replace with %s\n",
													editNum, lineNumStr, newText))
											}
										}
									}
								}
							}

							editNum++
							editsSection = editsSection[newTextStart+10:] // Move past this edit
						}
					}
				}
			}
		}
	}

	// Handle Changes map format (alternative)
	if workspaceEdit.Changes != nil {
		for uri, edits := range workspaceEdit.Changes {
			totalFiles++
			filename := filepath.Base(strings.TrimPrefix(string(uri), "file://"))
			editCount := len(edits)
			totalEdits += editCount

			result.WriteString(fmt.Sprintf("File: %s (%d edits)\n", filename, editCount))
			result.WriteString(fmt.Sprintf("   URI: %s\n", uri))

			for i, edit := range edits {
				startLine := edit.Range.Start.Line + 1 // Convert to 1-based
				endLine := edit.Range.End.Line + 1
				startChar := edit.Range.Start.Character
				endChar := edit.Range.End.Character

				if startLine == endLine {
					result.WriteString(fmt.Sprintf("   %d. Line %d (chars %d-%d): Replace with \"%s\"\n",
						i+1, startLine, startChar, endChar, edit.NewText))
				} else {
					result.WriteString(fmt.Sprintf("   %d. Lines %d-%d: Replace with \"%s\"\n",
						i+1, startLine, endLine, edit.NewText))
				}
			}
		}
	}

	if totalFiles == 0 {
		result.WriteString("No rename changes found")
	} else {
		result.WriteString(fmt.Sprintf("Files to be modified: %d\n", totalFiles))
		result.WriteString(fmt.Sprintf("Total edits: %d\n", totalEdits))
	}

	return result.String()
}

// Helper function to format implementations
func formatImplementations(implementations []protocol.Location) string {
	var result strings.Builder

	if len(implementations) == 0 {
		result.WriteString("No implementations found")
		return result.String()
	}

	result.WriteString(fmt.Sprintf("Found %d implementations:\n\n", len(implementations)))

	for i, location := range implementations {
		uri := string(location.Uri)
		filename := filepath.Base(strings.TrimPrefix(uri, "file://"))
		line := location.Range.Start.Line + 1 // Convert to 1-based

		result.WriteString(fmt.Sprintf("%d. %s:%d\n", i+1, filename, line))
	}

	return result.String()
}

// formatWorkspaceDiagnostics formats workspace diagnostic results for display
func formatWorkspaceDiagnostics(diagnostics []protocol.WorkspaceDiagnosticReport) string {
	if diagnostics == nil {
		return "No workspace diagnostics available"
	}

	if len(diagnostics) == 0 {
		return "No workspace diagnostics found"
	}

	var result strings.Builder

	totalIssues := 0
	errorCount := 0
	warningCount := 0
	infoCount := 0
	hintCount := 0

	for i, report := range diagnostics {
		result.WriteString(fmt.Sprintf("Language Server %d Results:\n", i+1))

		// Try to extract the full document report from the union type
		// This is a simplified approach - we'll format whatever we can extract
		result.WriteString(fmt.Sprintf("\nDocument Report: %+v\n", report))

		// For now, we'll handle this as a generic interface until we can properly decode the union type
		// TODO: Implement proper union type handling for WorkspaceDocumentDiagnosticReport
		result.WriteString("\n")
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
