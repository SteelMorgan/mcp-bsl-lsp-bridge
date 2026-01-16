package tools

import (
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/utils"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

type ToolServer interface {
	AddTool(tool mcp.Tool, handler server.ToolHandlerFunc)
}

// findPreciseCharacterPosition uses semantic tokens to find the exact character position of a symbol
func FindPreciseCharacterPosition(bridge interfaces.BridgeInterface, uri string, line, approxCharacter uint32, symbolName string) uint32 {
	// Get semantic tokens for the line containing the symbol
	tokens, err := bridge.SemanticTokens(uri, []string{"function", "variable", "type", "method"}, line, 0, line, 1000)
	if err != nil {
		logger.Warn(fmt.Sprintf("Failed to get semantic tokens for %s:%d: %v", uri, line, err))
		return approxCharacter // Fall back to original position
	}

	// Look for our symbol name in the semantic tokens on this line
	for _, token := range tokens {
		if token.Range.Start.Line == line && strings.Contains(token.Text, symbolName) {
			// Found a token containing our symbol name on the right line
			// Use the start character of this token for more precise positioning
			logger.Debug(fmt.Sprintf("Found precise position for %s: char %d -> %d", symbolName, approxCharacter, token.Range.Start.Character))
			return token.Range.Start.Character
		}
	}

	// If no semantic token found, try to find the symbol name in nearby positions
	logger.Debug(fmt.Sprintf("No semantic token found for %s, using approximate position %d", symbolName, approxCharacter))
	return approxCharacter
}

// safeUint32 safely converts an int to uint32, checking for overflow
func safeUint32(val int) (uint32, error) {
	if val < 0 {
		return 0, fmt.Errorf("value cannot be negative: %d", val)
	}
	// On 32-bit systems, int and uint32 have the same max value
	// On 64-bit systems, we need to check for overflow
	if uint64(val) > math.MaxUint32 {
		return 0, fmt.Errorf("value exceeds uint32 maximum: %d", val)
	}
	return uint32(val), nil // #nosec G115 -- overflow already checked above
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

	result.WriteString("DOCUMENT FORMATTING:\n")

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
	result.WriteString("SUMMARY:\n")
	result.WriteString(fmt.Sprintf("Total Issues: %d\n", totalIssues))
	result.WriteString(fmt.Sprintf("Errors: %d\n", errorCount))
	result.WriteString(fmt.Sprintf("Warnings: %d\n", warningCount))
	result.WriteString(fmt.Sprintf("Info: %d\n", infoCount))
	result.WriteString(fmt.Sprintf("Hints: %d\n", hintCount))

	if totalIssues == 0 {
		result.WriteString("\nNo issues found in workspace")
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

// getSeverityIcon returns appropriate prefix for diagnostic severity
func getSeverityIcon(severity string) string {
	switch severity {
	case "Error":
		return "[ERROR]"
	case "Warning":
		return "[WARN]"
	case "Information":
		return "[INFO]"
	case "Hint":
		return "[HINT]"
	default:
		return "[UNKNOWN]"
	}
}

// PaginationResult represents the result of applying pagination to a dataset
type PaginationResult struct {
	Offset      int  // Starting index of returned items
	Limit       int  // Maximum number of items requested
	Total       int  // Total number of items available
	Start       int  // Actual starting index (1-based for display)
	End         int  // Actual ending index (1-based for display)
	Count       int  // Actual number of items returned
	HasMore     bool // Whether there are more items after this page
	HasPrevious bool // Whether there are items before this page
}

// ApplyPagination applies offset and limit to a dataset and returns pagination info
// Generic function that works with any slice type
func ApplyPagination[T any](items []T, offset, limit int) ([]T, PaginationResult) {
	total := len(items)

	// Validate offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []T{}, PaginationResult{
			Offset:      offset,
			Limit:       limit,
			Total:       total,
			Start:       0,
			End:         0,
			Count:       0,
			HasMore:     false,
			HasPrevious: offset > 0,
		}
	}

	// Calculate end index
	end := offset + limit
	if end > total {
		end = total
	}

	// Get paginated slice
	paginatedItems := items[offset:end]

	// Create pagination result
	result := PaginationResult{
		Offset:      offset,
		Limit:       limit,
		Total:       total,
		Start:       offset + 1, // 1-based for display
		End:         end,        // 1-based for display
		Count:       len(paginatedItems),
		HasMore:     end < total,
		HasPrevious: offset > 0,
	}

	return paginatedItems, result
}

// FormatPaginationInfo formats pagination information for display
func FormatPaginationInfo(result PaginationResult) string {
	if result.Count == 0 {
		return fmt.Sprintf("No results (offset %d exceeds total %d)", result.Offset, result.Total)
	}

	if result.HasMore || result.HasPrevious {
		return fmt.Sprintf("Showing results %d-%d of %d total", result.Start, result.End, result.Total)
	}

	return fmt.Sprintf("Found %d results", result.Total)
}

// FormatPaginationControls formats pagination control instructions
func FormatPaginationControls(result PaginationResult) string {
	var controls []string

	if result.HasMore {
		remaining := result.Total - result.End
		controls = append(controls, fmt.Sprintf("... and %d more results available (use offset=%d to see next page)", remaining, result.End))
	}

	if result.HasPrevious {
		controls = append(controls, "Use offset=0 to see from the beginning")
	}

	if len(controls) > 0 {
		return "\n" + strings.Join(controls, "\n")
	}

	return ""
}

// getRangeContent returns the text that falls between the given
// (zero-based) line/character positions.  The strict flag controls
// whether out-of-bounds character indices are an error (strict=true)
// or silently clamped to the nearest legal value (strict=false).
// When strict=false the function behaves exactly like the original
// getSymbolContent.
func getRangeContent(
	bridge interfaces.BridgeInterface,
	uri string,
	startLine, startChar, endLine, endChar uint32,
	strict bool,
) (string, error) {

	// 1. Turn the URI into a clean absolute path.
	// Accept both file URIs and raw local paths (Windows/POSIX).
	filePath := utils.URIToFilePath(uri)
	// If bridge runs in Docker/container mode, map host paths to container paths before validation/read.
	if bridge.HasPathMapper() && bridge.GetPathMapper() != nil {
		mapper := bridge.GetPathMapper()
		// Normalize path separators for cross-platform compatibility
		normalizedPath := strings.ReplaceAll(filePath, "\\", "/")
		// Check if path is relative (not starting with / and not a Windows absolute path like D:/)
		isRelative := !strings.HasPrefix(normalizedPath, "/") && !utils.IsWindowsAbsPath(normalizedPath)
		// Relative paths are treated as container-root relative (e.g. "test-workspace/..." -> "/projects/test-workspace/...").
		if isRelative {
			filePath = path.Join(mapper.ContainerRoot(), normalizedPath)
		}
		slash := strings.ReplaceAll(filePath, "\\", "/")
		cr := strings.TrimSuffix(mapper.ContainerRoot(), "/")
		if !(slash == cr || strings.HasPrefix(slash, cr+"/")) {
			if mapped, mapErr := mapper.HostToContainer(filePath); mapErr == nil {
				filePath = mapped
			} else {
				return "", fmt.Errorf("path mapping failed: %w", mapErr)
			}
		}
	}

	absPath, err := bridge.IsAllowedDirectory(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %w", err)
	}

	// 2. Read the file.
	content, err := os.ReadFile(absPath) // #nosec G304
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// 3. Split into lines.
	lines := strings.Split(string(content), "\n")
	linesLen, err := safeUint32(len(lines))
	if err != nil {
		return "", fmt.Errorf("file too large (too many lines): %w", err)
	}

	// 4. Basic line-range validation.
	if startLine >= linesLen || endLine >= linesLen {
		return "", fmt.Errorf("line range %d-%d out of bounds (file has %d lines)",
			startLine, endLine, linesLen)
	}
	if startLine > endLine ||
		(startLine == endLine && startChar > endChar) {
		return "", fmt.Errorf("invalid range order: %d:%d - %d:%d",
			startLine, startChar, endLine, endChar)
	}

	// 5. Helper: clamp or reject out-of-bounds character indices.
	clamp := func(line string, pos uint32, lineNo int, which string) (uint32, error) {
		lineLen, err := safeUint32(len(line))
		if err != nil {
			return 0, fmt.Errorf("line too long: %w", err)
		}
		if pos > lineLen {
			if strict {
				return 0, fmt.Errorf("invalid %s character on line %d: %d (line length: %d)",
					which, lineNo, pos, lineLen)
			}
			return lineLen, nil // clamp to line end
		}
		return pos, nil
	}

	// 6. Extract the requested text.
	var result []string

	if startLine == endLine {
		// Single-line slice.
		line := lines[startLine]
		start, err := clamp(line, startChar, int(startLine), "start")
		if err != nil {
			return "", err
		}
		end, err := clamp(line, endChar, int(startLine), "end")
		if err != nil {
			return "", err
		}
		if start > end {
			return "", fmt.Errorf("invalid character range on line %d: start %d > end %d",
				startLine, start, end)
		}
		lineStartLen, err := safeUint32(len(line))
		if err != nil {
			return "", fmt.Errorf("line too long: %w", err)
		}
		if start >= lineStartLen {
			result = append(result, "")
		} else {
			result = append(result, line[start:end])
		}
	} else {
		// Multi-line slice.
		// First line: from startChar to line end.
		first := lines[startLine]
		start, err := clamp(first, startChar, int(startLine), "start")
		if err != nil {
			return "", err
		}
		firstLen, err := safeUint32(len(first))
		if err != nil {
			return "", fmt.Errorf("line too long: %w", err)
		}
		if start >= firstLen {
			result = append(result, "")
		} else {
			result = append(result, first[start:])
		}

		// Full middle lines.
		for i := startLine + 1; i < endLine; i++ {
			result = append(result, lines[i])
		}

		// Last line: treat endChar as exclusive, but allow clamping/clipping
		last := lines[endLine]
		lineLen, err := safeUint32(len(last))
		if err != nil {
			return "", fmt.Errorf("line too long: %w", err)
		}

		if endChar > lineLen {
			if strict {
				return "", fmt.Errorf("invalid end character on line %d: %d (line length: %d)",
					endLine, endChar, lineLen)
			}
			// non-strict: take the entire last line
			result = append(result, last)
		} else {
			// exclusive index
			endIdx := endChar
			if endIdx > 0 {
				endIdx--
			}
			result = append(result, last[:endIdx])
		}
	}

	return strings.Join(result, "\n"), nil
}

// ResolvedFileContext represents the result of file context resolution
type ResolvedFileContext struct {
	ResolvedPath string   // Found file path (empty if not found)
	Suggestions  []string // Alternative directories/files to try
	CacheInfo    string   // Cache age info for user
	ErrorMessage string   // Helpful error with guidance
}

// DirectoryCache provides lightweight directory caching with TTL
type DirectoryCache struct {
	entries    map[string][]string  // directory -> file list
	timestamps map[string]time.Time // directory -> cache time
	ttl        time.Duration        // 5 minutes default
	mu         sync.RWMutex         // thread safety
}

// Global directory cache instance
var globalDirCache = &DirectoryCache{
	entries:    make(map[string][]string),
	timestamps: make(map[string]time.Time),
	ttl:        5 * time.Minute,
}

// GetFiles returns cached files for a directory, or scans if not cached/stale
func (dc *DirectoryCache) GetFiles(dir string) ([]string, bool, time.Duration) {
	dc.mu.RLock()
	files, exists := dc.entries[dir]
	timestamp, hasTimestamp := dc.timestamps[dir]
	dc.mu.RUnlock()

	if !exists || !hasTimestamp {
		return dc.scanDirectory(dir)
	}

	age := time.Since(timestamp)
	if age > dc.ttl {
		return dc.scanDirectory(dir)
	}

	return files, true, age
}

// scanDirectory scans a directory and caches the results
func (dc *DirectoryCache) scanDirectory(dir string) ([]string, bool, time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false, 0
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	dc.mu.Lock()
	dc.entries[dir] = files
	dc.timestamps[dir] = time.Now()
	dc.mu.Unlock()

	return files, false, 0 // Fresh scan, not cached
}

// ResolveFileContext attempts to resolve a file context to an actual file path
func ResolveFileContext(bridge interfaces.BridgeInterface, fileContext string, workspaceDir string) (*ResolvedFileContext, error) {
	result := &ResolvedFileContext{}

	// 1. Try exact path
	if exactPath := tryExactPath(workspaceDir, fileContext); exactPath != "" {
		result.ResolvedPath = exactPath
		return result, nil
	}

	// 2. Try common directory patterns
	if patternPath := tryCommonPatterns(workspaceDir, fileContext); patternPath != "" {
		result.ResolvedPath = patternPath
		return result, nil
	}

	// 3. Try with extension inference
	if extPath := tryWithExtensions(workspaceDir, fileContext); extPath != "" {
		result.ResolvedPath = extPath
		return result, nil
	}

	// 4. Generate helpful suggestions
	suggestions, cacheInfo := generateSuggestions(workspaceDir, fileContext)
	result.Suggestions = suggestions
	result.CacheInfo = cacheInfo
	result.ErrorMessage = formatHelpfulError(fileContext, suggestions, cacheInfo)

	return result, nil
}

// tryExactPath tests if the file context is an exact path that exists
func tryExactPath(workspaceDir, fileContext string) string {
	// Try absolute path first
	if filepath.IsAbs(fileContext) {
		if fileExists(fileContext) {
			return fileContext
		}
		return ""
	}

	// Try relative to workspace
	fullPath := filepath.Join(workspaceDir, fileContext)
	if fileExists(fullPath) {
		return fullPath
	}

	return ""
}

// tryCommonPatterns tests common directory patterns for the file
func tryCommonPatterns(workspaceDir, fileContext string) string {
	// Get common patterns based on file extension
	ext := filepath.Ext(fileContext)
	patterns := getCommonPatternsForExtension(ext)

	for _, pattern := range patterns {
		testPath := filepath.Join(workspaceDir, pattern, fileContext)
		if fileExists(testPath) {
			return testPath
		}
	}

	return ""
}

// tryWithExtensions tries adding common extensions if none provided
func tryWithExtensions(workspaceDir, fileContext string) string {
	// Skip if already has extension
	if filepath.Ext(fileContext) != "" {
		return ""
	}

	// Try common extensions
	extensions := []string{".js", ".ts", ".tsx", ".jsx", ".go", ".py", ".java", ".cpp", ".c", ".h"}

	for _, ext := range extensions {
		testName := fileContext + ext

		// Try exact path with extension
		if exactPath := tryExactPath(workspaceDir, testName); exactPath != "" {
			return exactPath
		}

		// Try common patterns with extension
		if patternPath := tryCommonPatterns(workspaceDir, testName); patternPath != "" {
			return patternPath
		}
	}

	return ""
}

// generateSuggestions generates helpful suggestions when file not found
func generateSuggestions(workspaceDir, fileContext string) ([]string, string) {
	var suggestions []string
	var cacheInfo strings.Builder

	// Get file extension for targeted suggestions
	ext := filepath.Ext(fileContext)
	if ext == "" {
		// Guess extensions to look for
		ext = ".js" // Default guess
	}

	// Check common directories for files with similar extension
	patterns := getCommonPatternsForExtension(ext)

	for _, pattern := range patterns {
		dirPath := filepath.Join(workspaceDir, pattern)
		files, found, age := globalDirCache.GetFiles(dirPath)

		if found && len(files) > 0 {
			// Count matching files
			matchingFiles := 0
			for _, file := range files {
				if filepath.Ext(file) == ext {
					matchingFiles++
				}
			}

			if matchingFiles > 0 {
				ageInfo := "fresh"
				if age > 0 {
					ageInfo = fmt.Sprintf("%.0f min old", age.Minutes())
				}

				suggestion := fmt.Sprintf("%s/ (%d %s files, cache: %s)", pattern, matchingFiles, ext, ageInfo)
				suggestions = append(suggestions, suggestion)

				if cacheInfo.Len() > 0 {
					cacheInfo.WriteString(", ")
				}
				cacheInfo.WriteString(ageInfo)
			}
		}
	}

	return suggestions, cacheInfo.String()
}

// formatHelpfulError creates a user-friendly error message with guidance
func formatHelpfulError(fileContext string, suggestions []string, cacheInfo string) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("File '%s' not found.", fileContext))

	if len(suggestions) > 0 {
		result.WriteString(" Found files in these directories:\n")
		for _, suggestion := range suggestions {
			result.WriteString(fmt.Sprintf("- %s\n", suggestion))
		}

		result.WriteString("\nSuggestions:\n")
		result.WriteString(fmt.Sprintf("1. Try specific path: file_context='%s/%s'\n", suggestions[0][:strings.Index(suggestions[0], "/")], fileContext))
		result.WriteString(fmt.Sprintf("2. Search with ripgrep: Use Grep tool with pattern='%s' and glob='*.%s'\n",
			strings.TrimSuffix(fileContext, filepath.Ext(fileContext)),
			strings.TrimPrefix(filepath.Ext(fileContext), ".")))
		result.WriteString(fmt.Sprintf("3. Directory analysis: Use project_analysis with file_analysis on '%s/'\n", suggestions[0][:strings.Index(suggestions[0], "/")]))
		result.WriteString(fmt.Sprintf("4. Find files by name: Use 'fd %s' or 'find . -name %s'\n", fileContext, fileContext))
	} else {
		result.WriteString(" No similar files found in common directories.\n")
		result.WriteString("\nSuggestions:\n")
		result.WriteString("1. Check file path and spelling\n")
		result.WriteString("2. Use Grep tool to search for content\n")
		result.WriteString("3. Use project_analysis to explore directory structure\n")
		result.WriteString("4. Use workspace_symbols to find all symbols\n")
	}

	if cacheInfo != "" {
		result.WriteString("\nCache info: " + cacheInfo)
	}

	return result.String()
}

// getCommonPatternsForExtension returns common directory patterns for file extensions
func getCommonPatternsForExtension(ext string) []string {
	patterns := map[string][]string{
		".js":   {"src", "assets/js", "components", "lib", "public/js", "js"},
		".jsx":  {"src", "components", "lib", "src/components"},
		".ts":   {"src", "components", "lib", "types", "src/types"},
		".tsx":  {"src", "components", "lib", "src/components"},
		".go":   {"", "cmd", "internal", "pkg"},
		".py":   {"src", "lib", "scripts", ""},
		".java": {"src/main/java", "src", "java"},
		".cpp":  {"src", "lib", "include"},
		".c":    {"src", "lib", "include"},
		".h":    {"include", "src", "lib"},
	}

	if patterns[ext] != nil {
		return patterns[ext]
	}

	// Default patterns for unknown extensions
	return []string{"src", "lib", ""}
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
