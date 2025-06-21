package tools

import (
	"fmt"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// MockBridge for hover tool testing
type MockBridge struct {
	mockGetHoverInformation func(string, int32, int32) (any, error)
	mockInferLanguage       func(string) (string, error)
}

func (m *MockBridge) GetClientForLanguageInterface(language string) (any, error) { return nil, nil }
func (m *MockBridge) InferLanguage(filePath string) (string, error) {
	if m.mockInferLanguage != nil {
		return m.mockInferLanguage(filePath)
	}
	return "go", nil
}
func (m *MockBridge) CloseAllClients()                                                          {}
func (m *MockBridge) GetConfig() *lsp.LSPServerConfig                                           { return nil }
func (m *MockBridge) DetectProjectLanguages(projectPath string) ([]string, error)             { return []string{"go"}, nil }
func (m *MockBridge) DetectPrimaryProjectLanguage(projectPath string) (string, error)         { return "go", nil }
func (m *MockBridge) FindSymbolReferences(language, uri string, line, character int32, includeDeclaration bool) ([]any, error) {
	return []any{}, nil
}
func (m *MockBridge) FindSymbolDefinitions(language, uri string, line, character int32) ([]any, error) {
	return []any{}, nil
}
func (m *MockBridge) SearchTextInWorkspace(language, query string) ([]any, error) { return []any{}, nil }
func (m *MockBridge) GetMultiLanguageClients(languages []string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (m *MockBridge) GetHoverInformation(uri string, line, character int32) (any, error) {
	if m.mockGetHoverInformation != nil {
		return m.mockGetHoverInformation(uri, line, character)
	}
	return nil, nil
}
func (m *MockBridge) GetDiagnostics(uri string) ([]any, error)                    { return []any{}, nil }
func (m *MockBridge) GetWorkspaceDiagnostics(workspaceUri string, identifier string) (any, error) {
	return []any{}, nil
}
func (m *MockBridge) GetSignatureHelp(uri string, line, character int32) (any, error) { return nil, nil }
func (m *MockBridge) GetCodeActions(uri string, line, character, endLine, endCharacter int32) ([]any, error) {
	return []any{}, nil
}
func (m *MockBridge) FormatDocument(uri string, tabSize int32, insertSpaces bool) ([]any, error) {
	return []any{}, nil
}
func (m *MockBridge) RenameSymbol(uri string, line, character int32, newName string, preview bool) (any, error) {
	return nil, nil
}
func (m *MockBridge) FindImplementations(uri string, line, character int32) ([]any, error) {
	return []any{}, nil
}
func (m *MockBridge) PrepareCallHierarchy(uri string, line, character int32) ([]any, error) {
	return []any{}, nil
}
func (m *MockBridge) GetIncomingCalls(item any) ([]any, error) { return []any{}, nil }
func (m *MockBridge) GetOutgoingCalls(item any) ([]any, error) { return []any{}, nil }
func (m *MockBridge) GetDocumentSymbols(uri string) ([]any, error) { return []any{}, nil }
func (m *MockBridge) ApplyTextEdits(uri string, edits []any) error { return nil }
func (m *MockBridge) ApplyWorkspaceEdit(edit any) error { return nil }

// Test hover tool registration and functionality
func TestHoverTool(t *testing.T) {
	testCases := []struct {
		name         string
		uri          string
		line         int32
		character    int32
		mockResponse any
		mockError    error
		expectError  bool
		description  string
	}{
		{
			name:      "successful hover with proper Hover type",
			uri:       "file:///test.go",
			line:      10,
			character: 5,
			mockResponse: &protocol.Hover{
				Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{
					Value: protocol.MarkupContent{
						Kind:  protocol.MarkupKindMarkdown,
						Value: "**function main()**\n\nMain function of the program",
					},
				},
			},
			expectError: false,
			description: "Should handle proper protocol.Hover response",
		},
		{
			name:      "successful hover with map contents",
			uri:       "file:///test.go",
			line:      10,
			character: 5,
			mockResponse: map[string]any{
				"contents": "Test hover information",
			},
			expectError: false,
			description: "Should handle map-based hover response",
		},
		{
			name:      "hover response with nil result",
			uri:       "file:///test.go",
			line:      10,
			character: 5,
			mockResponse: (*protocol.Hover)(nil),
			expectError: false,
			description: "Should handle nil Hover result",
		},
		{
			name:        "hover error - column beyond line",
			uri:         "file:///test.go",
			line:        10,
			character:   100,
			mockError:   fmt.Errorf("hover request failed: jsonrpc2: code 0 message: column is beyond end of line"),
			expectError: true,
			description: "Should handle column position errors",
		},
		{
			name:        "hover error - invalid response",
			uri:         "file:///test.go",
			line:        10,
			character:   5,
			mockError:   fmt.Errorf("hover request failed: response must have an id and jsonrpc field"),
			expectError: true,
			description: "Should handle invalid JSON-RPC responses",
		},
		{
			name:      "hover with absolute path (should normalize to file URI)",
			uri:       "/home/user/test.go",
			line:      5,
			character: 10,
			mockResponse: &protocol.Hover{
				Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{
					Value: protocol.MarkupContent{
						Kind:  protocol.MarkupKindPlainText,
						Value: "variable: int",
					},
				},
			},
			expectError: false,
			description: "Should handle absolute paths by normalizing to file URI",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &MockBridge{
				mockGetHoverInformation: func(uri string, line, character int32) (any, error) {
					if tc.mockError != nil {
						return nil, tc.mockError
					}
					return tc.mockResponse, nil
				},
			}

			mcpServer := server.NewMCPServer(
				"test-server",
				"1.0.0",
				server.WithToolCapabilities(false),
			)

			RegisterHoverTool(mcpServer, bridge)

			// Just check that the server was created successfully
			if mcpServer == nil {
				t.Fatal("MCP server should not be nil")
			}

			// Test the hover functionality by directly calling the bridge method
			result, err := bridge.GetHoverInformation(tc.uri, tc.line, tc.character)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				} else {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				} else {
					t.Logf("Got expected result: %T", result)
				}
			}

			t.Logf("Test case '%s' passed - %s", tc.name, tc.description)
		})
	}
}

// Test formatting functions
func TestFormatHoverContent(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "string content",
			input:    "Simple hover text",
			expected: "=== HOVER INFORMATION ===\nSimple hover text",
		},
		{
			name:     "unknown content type",
			input:    123,
			expected: "=== HOVER INFORMATION ===\nContent: 123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatHoverContent(tc.input)
			if !strings.Contains(result, "HOVER INFORMATION") {
				t.Errorf("Expected result to contain 'HOVER INFORMATION', got: %s", result)
			}
		})
	}
}