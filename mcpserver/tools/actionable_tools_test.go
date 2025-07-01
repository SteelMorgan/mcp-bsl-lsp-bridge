package tools

import (
	"strings"
	"testing"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Test the format output functions directly

func TestFormatDocumentActionable(t *testing.T) {
	// Test format output formatting
	mockEdits := []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 0},
				End:   protocol.Position{Line: 2, Character: 1},
			},
			NewText: "",
		},
	}

	result := formatTextEdits(mockEdits)
	
	if !strings.Contains(result, "DOCUMENT FORMATTING") {
		t.Errorf("Expected formatting header, got: %s", result)
	}

	if !strings.Contains(result, "Line 3") { // Should be 1-based (2+1=3)
		t.Errorf("Expected line number in output, got: %s", result)
	}

	if !strings.Contains(result, "Remove whitespace/formatting") {
		t.Errorf("Expected action description, got: %s", result)
	}
}

func TestRenameSymbolActionable(t *testing.T) {
	// Test workspace edit formatting (which the rename tool uses)
	changes := map[protocol.DocumentUri][]protocol.TextEdit{
		"file:///test1.go": {
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 5, Character: 4},
					End:   protocol.Position{Line: 5, Character: 12},
				},
				NewText: "newFuncName",
			},
		},
		"file:///test2.go": {
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 10, Character: 8},
					End:   protocol.Position{Line: 10, Character: 16},
				},
				NewText: "newFuncName",
			},
		},
	}
	
	mockWorkspaceEdit := protocol.WorkspaceEdit{
		Changes: changes,
	}

	result := formatWorkspaceEdit(&mockWorkspaceEdit)
	
	if !strings.Contains(result, "Files to be modified: 2") {
		t.Errorf("Expected file count in output, got: %s", result)
	}

	if !strings.Contains(result, "Total edits: 2") {
		t.Errorf("Expected edit count in output, got: %s", result)
	}

	if !strings.Contains(result, "File: test1.go") {
		t.Errorf("Expected first filename, got: %s", result)
	}

	if !strings.Contains(result, "File: test2.go") {
		t.Errorf("Expected second filename, got: %s", result)
	}

	if !strings.Contains(result, "Replace with \"newFuncName\"") {
		t.Errorf("Expected replacement text, got: %s", result)
	}
}

func TestFormatWorkspaceEditOutput(t *testing.T) {
	// Test nil edit
	result := formatWorkspaceEdit(nil)
	if !strings.Contains(result, "No changes needed") {
		t.Errorf("Expected 'No changes needed' for nil edit, got: %s", result)
	}

	// Test valid workspace edit
	mockWorkspaceEdit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			"file:///test.go": {
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 4, Character: 0}, // 0-based
						End:   protocol.Position{Line: 4, Character: 8},
					},
					NewText: "newName",
				},
			},
		},
	}

	result = formatWorkspaceEdit(&mockWorkspaceEdit)
	
	// Check for expected content
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
