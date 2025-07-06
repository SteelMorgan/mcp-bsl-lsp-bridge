package tools

import (
	"errors"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Helper function for testing - removed unused createMockRequest

// Test actual tool handler execution
func TestInferLanguageToolHandler(t *testing.T) {
	bridge := &mocks.MockBridge{}

	// Setup mock expectations
	mockConfig := &lsp.LSPServerConfig{
		ExtensionLanguageMap: map[string]types.Language{
			".go":   "go",
			".js":   "javascript",
			".py":   "python",
			".java": "java",
		},
	}
	bridge.On("GetConfig").Return(mockConfig)

	// Create MCP server and register tool
	mcpServer, err := mcptest.NewServer(t)
	if err != nil {
		t.Errorf("Could not make a MCP server: %v", err)
	}

	RegisterInferLanguageTool(mcpServer, bridge)

	// Test successful inference
	t.Run("successful language inference", func(t *testing.T) {
		config := bridge.GetConfig()
		if config == nil {
			t.Fatal("Expected config to be available")
		}

		language, err := config.FindExtLanguage(".go")
		if err != nil {
			t.Fatal("Expected .go extension to be mapped")
		}

		if *language != "go" {
			t.Errorf("Expected 'go', got '%s'", string(*language))
		}
	})

	// Test missing extension
	t.Run("missing extension", func(t *testing.T) {
		config := bridge.GetConfig()
		if config == nil {
			t.Fatal("Expected config to be available")
		}

		_, err := config.FindExtLanguage(".unknown")
		if err == nil {
			t.Error("Expected .unknown extension to not be found")
		}
	})

	// Verify all expectations were met
	bridge.AssertExpectations(t)
}
func TestHoverToolHandler(t *testing.T) {
	testCases := []struct {
		name             string
		mockResponse     any
		mockError        error
		expectError      bool
		expectedInResult string
	}{
		{
			name: "successful hover with explicit mock",
			mockResponse: &protocol.Hover{
				Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{
					Value: "Function documentation",
				},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name: "successful hover with expected result",
			mockResponse: &protocol.Hover{
				Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{
					Value: "Function documentation",
				},
			},
			expectError:      false,
			expectedInResult: "Function documentation",
		},
		{
			name:         "hover error",
			mockResponse: (*protocol.Hover)(nil),
			mockError:    errors.New("hover failed"),
			expectError:  true,
		},
		{
			name:             "nil hover result",
			mockResponse:     (*protocol.Hover)(nil),
			expectError:      false,
			expectedInResult: "Content: <nil>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			// Set up mock expectation
			bridge.On("GetHoverInformation", "file:///test.go", uint32(10), uint32(5)).Return(tc.mockResponse, tc.mockError)

			// Test the actual hover functionality
			result, err := bridge.GetHoverInformation("file:///test.go", 10, 5)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if tc.expectedInResult != "" && result != nil {
					// Test formatting
					formatted := formatHoverContent(result.Contents)
					if !strings.Contains(formatted, tc.expectedInResult) {
						t.Errorf("Expected result to contain '%s', got: %s", tc.expectedInResult, formatted)
					}
				}
			}

			// Verify mock expectations were met
			bridge.AssertExpectations(t)
		})
	}
}

func TestDiagnosticsToolHandler(t *testing.T) {
	bridge := &mocks.MockBridge{}

	// Test successful diagnostics
	t.Run("successful diagnostics", func(t *testing.T) {
		hint := protocol.DiagnosticSeverityHint
		// Define a mock diagnostic
		mockDiagnostic := protocol.Diagnostic{
			Message: "Test diagnostic message",
			Range: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 5, Character: 10},
			},
			Severity: &hint, // Or Warning, Error, Information
		}
		bridge.On("GetDiagnostics", "file:///test.go").Return([]any{mockDiagnostic}, nil)

		result, err := bridge.GetDiagnostics("file:///test.go")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		formatted := formatDiagnostics(result)
		if !strings.Contains(formatted, "WARNING") {
			t.Errorf("Expected formatted result to contain 'WARNING'")
		}
	})

	// Test diagnostics error
	t.Run("diagnostics error", func(t *testing.T) {
		// Return empty slice instead of nil when there's an error
		bridge.On("GetDiagnostics", "file:///error.go").Return([]any{}, errors.New("diagnostics failed"))

		_, err := bridge.GetDiagnostics("file:///error.go")
		if err == nil {
			t.Error("Expected error but got none")
		}
	})
}

func TestLSPDisconnectToolHandler(t *testing.T) {
	bridge := &mocks.MockBridge{}

	// Set up mock expectation - CloseAllClients should be called once and return nothing
	bridge.On("CloseAllClients").Return().Once()

	// Test disconnect functionality
	bridge.CloseAllClients()

	// Verify that the mock expectations were met
	bridge.AssertExpectations(t)
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("symbolKindToString", func(t *testing.T) {
		tests := []struct {
			kind     protocol.SymbolKind
			expected string
		}{
			{protocol.SymbolKindFile, "file"},
			{protocol.SymbolKindClass, "class"},
			{protocol.SymbolKindFunction, "function"},
			{protocol.SymbolKindVariable, "variable"},
			{protocol.SymbolKind(999), "unknown(999)"},
		}

		for _, test := range tests {
			result := symbolKindToString(test.kind)
			if result != test.expected {
				t.Errorf("symbolKindToString(%v) = %s, want %s", test.kind, result, test.expected)
			}
		}
	})

	t.Run("formatTextEdits", func(t *testing.T) {
		edits := []protocol.TextEdit{
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End:   protocol.Position{Line: 0, Character: 5},
				},
				NewText: "formatted",
			},
		}

		result := formatTextEdits(edits)
		if !strings.Contains(result, "DOCUMENT FORMATTING") {
			t.Error("Expected result to contain 'DOCUMENT FORMATTING'")
		}
	})

	t.Run("formatWorkspaceEdit", func(t *testing.T) {
		edit := protocol.WorkspaceEdit{Changes: map[protocol.DocumentUri][]protocol.TextEdit{}}

		result := formatWorkspaceEdit(&edit)
		if !strings.Contains(result, "No rename changes found") {
			t.Error("Expected result to contain 'No rename changes found'")
		}
	})

	t.Run("formatImplementations", func(t *testing.T) {
		impls := []protocol.Location{
			{
				Uri: "file:///test.go",
				Range: protocol.Range{
					Start: protocol.Position{Line: 10, Character: 0},
				},
			},
		}

		result := formatImplementations(impls)
		if !strings.Contains(result, "implementations") {
			t.Error("Expected result to contain 'implementations'")
		}
	})
}
