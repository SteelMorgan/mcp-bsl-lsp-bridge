package tools

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

func TestFormatTextEdits(t *testing.T) {
	// Test formatTextEdits function with simple protocol.TextEdit
	edits := []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 0},
				End:   protocol.Position{Line: 2, Character: 1},
			},
			NewText: "",
		},
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 5, Character: 4},
			},
			NewText: "formatted",
		},
	}

	result := formatTextEdits(edits)

	// Check for line numbers (should be 1-based)
	if !strings.Contains(result, "Line 3") { // 2+1=3
		t.Errorf("Expected line 3 in output, got: %s", result)
	}

	if !strings.Contains(result, "Line 6") { // 5+1=6
		t.Errorf("Expected line 6 in output, got: %s", result)
	}

	// Check for action descriptions
	if !strings.Contains(result, "Remove whitespace/formatting") {
		t.Errorf("Expected remove action description, got: %s", result)
	}

	if !strings.Contains(result, "Replace with: \"formatted\"") {
		t.Errorf("Expected replace action description, got: %s", result)
	}

	if !strings.Contains(result, "Total edits: 2") {
		t.Errorf("Expected total edits count, got: %s", result)
	}

	if !strings.Contains(result, "Whitespace/formatting edits: 1") {
		t.Errorf("Expected whitespace edits count, got: %s", result)
	}

	if !strings.Contains(result, "Content edits: 1") {
		t.Errorf("Expected content edits count, got: %s", result)
	}
}

func TestFormatWorkspaceEditSimple(t *testing.T) {
	// Test formatWorkspaceEdit with nil input
	result := formatWorkspaceEdit(nil)
	if !strings.Contains(result, "No changes needed") {
		t.Errorf("Expected 'No changes needed' for nil edit, got: %s", result)
	}

	// Test with simple workspace edit using Changes map
	changes := map[protocol.DocumentUri][]protocol.TextEdit{
		"file:///test.go": {
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 4, Character: 0}, // 0-based
					End:   protocol.Position{Line: 4, Character: 8},
				},
				NewText: "newName",
			},
		},
	}

	mockWorkspaceEdit := protocol.WorkspaceEdit{
		Changes: changes,
	}

	result = formatWorkspaceEdit(&mockWorkspaceEdit)

	if !strings.Contains(result, "File: test.go") {
		t.Errorf("Expected filename in output, got: %s", result)
	}

	if !strings.Contains(result, "Line 5") { // Should be 1-based (4+1=5)
		t.Errorf("Expected line number in output, got: %s", result)
	}

	if !strings.Contains(result, "Replace with \"newName\"") {
		t.Errorf("Expected replacement text in output, got: %s", result)
	}

	if !strings.Contains(result, "Files to be modified: 1") {
		t.Errorf("Expected file count in output, got: %s", result)
	}

	if !strings.Contains(result, "Total edits: 1") {
		t.Errorf("Expected edit count in output, got: %s", result)
	}
}

func applyTextEditsToContent(content string, edits []protocol.TextEdit) (string, error) {
	// Convert string to lines
	lines := strings.Split(content, "\n")

	// Sort edits in reverse order to prevent index shifts
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].Range.Start.Line > edits[j].Range.Start.Line ||
			(edits[i].Range.Start.Line == edits[j].Range.Start.Line &&
				edits[i].Range.Start.Character > edits[j].Range.Start.Character)
	})

	// Apply each edit
	for _, edit := range edits {
		lineIndex := int(edit.Range.Start.Line)
		startChar := int(edit.Range.Start.Character)
		endChar := int(edit.Range.End.Character)

		if lineIndex < 0 || lineIndex >= len(lines) {
			return "", fmt.Errorf("line index out of bounds: %d", lineIndex)
		}

		line := lines[lineIndex]
		if startChar < 0 || endChar > len(line) {
			return "", fmt.Errorf("character index out of bounds on line %d", lineIndex)
		}

		// Replace the specified range with new text
		lines[lineIndex] = line[:startChar] + edit.NewText + line[endChar:]
	}

	// Rejoin the lines
	return strings.Join(lines, "\n"), nil
}

func TestApplyTextEditsToContent(t *testing.T) {
	// Test the text application logic
	content := `package main

func oldName() {
	return
}
`

	edits := []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 5}, // "oldName"
				End:   protocol.Position{Line: 2, Character: 12},
			},
			NewText: "newName",
		},
	}

	result, err := applyTextEditsToContent(content, edits)
	if err != nil {
		t.Fatalf("applyTextEditsToContent failed: %v", err)
	}

	if !strings.Contains(result, "func newName()") {
		t.Errorf("Expected function to be renamed, got: %s", result)
	}

	if strings.Contains(result, "oldName") {
		t.Errorf("Expected old name to be replaced, but still found in: %s", result)
	}
}
