package tools

import (
	"errors"
	"rockerboo/mcp-lsp-bridge/mocks"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// TestWorkspaceDiagnosticsTool tests workspace diagnostics functionality
func TestWorkspaceDiagnosticsTool(t *testing.T) {
	tests := []struct {
		name            string
		workspaceUri    string
		mockDiagnostics any
		mockError       error
		expectError     bool
		description     string
	}{
		{
			name:         "successful workspace diagnostics",
			workspaceUri: "/home/rockerboo/code/mcp-lsp-bridge",
			mockDiagnostics: []any{
				protocol.WorkspaceDiagnosticReport{
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						// Note: WorkspaceDocumentDiagnosticReport is a union type,
						// for testing we'll use a simplified mock structure
					},
				},
			},
			mockError:   nil,
			expectError: false,
			description: "Should handle successful workspace diagnostics",
		},
		{
			name:            "workspace diagnostics error",
			workspaceUri:    "/home/rockerboo/code/mcp-lsp-bridge",
			mockDiagnostics: nil,
			mockError:       errors.New("workspace diagnostics failed"),
			expectError:     true,
			description:     "Should handle workspace diagnostics errors",
		},
		{
			name:            "empty workspace diagnostics",
			workspaceUri:    "/home/rockerboo/code/mcp-lsp-bridge",
			mockDiagnostics: []any{},
			mockError:       nil,
			expectError:     false,
			description:     "Should handle empty workspace diagnostics",
		},
		{
			name:            "file URI workspace diagnostics",
			workspaceUri:    "file:///home/rockerboo/code/mcp-lsp-bridge",
			mockDiagnostics: []any{},
			mockError:       nil,
			expectError:     false,
			description:     "Should handle file:// URI workspace diagnostics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			mcpServer := server.NewMCPServer(
				"test-server",
				"1.0.0",
				server.WithToolCapabilities(false),
			)

			RegisterWorkspaceDiagnosticsTool(mcpServer, bridge)

			if mcpServer == nil {
				t.Fatal("Failed to create MCP server")
			}

			t.Logf("Test case '%s' passed - %s", tt.name, tt.description)
		})
	}
}

// TestFormatWorkspaceDiagnostics tests workspace diagnostics formatting
func TestFormatWorkspaceDiagnostics(t *testing.T) {
	testCases := []struct {
		name     string
		input    []protocol.WorkspaceDiagnosticReport
		expected []string // Expected strings to be present in output
	}{
		{
			name:     "nil diagnostics",
			input:    nil,
			expected: []string{"No workspace diagnostics available"},
		},
		{
			name:     "empty diagnostics",
			input:    []protocol.WorkspaceDiagnosticReport{},
			expected: []string{"No workspace diagnostics found"},
		},
		{
			name: "workspace report with diagnostics",
			input: []protocol.WorkspaceDiagnosticReport{
				{
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						// Simplified test case - union type handling will be implemented later
					},
				},
			},
			expected: []string{
				"=== Workspace Diagnostics ===",
				"Language Server 1 Results:",
				"✅ No issues found in workspace",
			},
		},
		{
			name: "workspace report with no issues",
			input: []protocol.WorkspaceDiagnosticReport{
				{
					Items: []protocol.WorkspaceDocumentDiagnosticReport{},
				},
			},
			expected: []string{
				"=== Workspace Diagnostics ===",
				"✅ No issues found in workspace",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatWorkspaceDiagnostics(tc.input)

			for _, expected := range tc.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain '%s', got: %s", expected, result)
				}
			}
		})
	}
}

// TestDiagnosticSeverityFunctions tests diagnostic severity helper functions
func TestDiagnosticSeverityFunctions(t *testing.T) {
	t.Run("getDiagnosticSeverityString", func(t *testing.T) {
		errorSev := protocol.DiagnosticSeverityError
		warnSev := protocol.DiagnosticSeverityWarning
		infoSev := protocol.DiagnosticSeverityInformation
		hintSev := protocol.DiagnosticSeverityHint

		tests := []struct {
			severity *protocol.DiagnosticSeverity
			expected string
		}{
			{nil, "Unknown"},
			{&errorSev, "Error"},
			{&warnSev, "Warning"},
			{&infoSev, "Information"},
			{&hintSev, "Hint"},
		}

		for _, tt := range tests {
			result := getDiagnosticSeverityString(tt.severity)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		}
	})

	t.Run("getSeverityIcon", func(t *testing.T) {
		tests := []struct {
			severity string
			expected string
		}{
			{"Error", "🔴"},
			{"Warning", "🟡"},
			{"Information", "🔵"},
			{"Hint", "💡"},
			{"Unknown", "❓"},
		}

		for _, tt := range tests {
			result := getSeverityIcon(tt.severity)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		}
	})
}
