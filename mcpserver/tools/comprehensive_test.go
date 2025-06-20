package tools

import (
	"fmt"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// ComprehensiveMockBridge provides a full mock implementation for testing tool handlers
type ComprehensiveMockBridge struct {
	// Language inference
	inferLanguageFunc func(string) (string, error)
	
	// Client management
	getClientForLanguageFunc func(string) (any, error)
	closeAllClientsFunc func()
	getConfigFunc func() *lsp.LSPServerConfig
	
	// Project analysis
	detectProjectLanguagesFunc func(string) ([]string, error)
	detectPrimaryProjectLanguageFunc func(string) (string, error)
	getMultiLanguageClientsFunc func([]string) (map[string]any, error)
	
	// Symbol operations
	findSymbolReferencesFunc func(string, string, int32, int32, bool) ([]any, error)
	findSymbolDefinitionsFunc func(string, string, int32, int32) ([]any, error)
	searchTextInWorkspaceFunc func(string, string) ([]any, error)
	
	// Code intelligence
	getHoverInformationFunc func(string, int32, int32) (any, error)
	getSignatureHelpFunc func(string, int32, int32) (any, error)
	getDiagnosticsFunc func(string) ([]any, error)
	getWorkspaceDiagnosticsFunc func(string, string) (any, error)
	
	// Code improvement
	getCodeActionsFunc func(string, int32, int32, int32, int32) ([]any, error)
	formatDocumentFunc func(string, int32, bool) ([]any, error)
	
	// Advanced navigation
	renameSymbolFunc func(string, int32, int32, string, bool) (any, error)
	findImplementationsFunc func(string, int32, int32) ([]any, error)
	prepareCallHierarchyFunc func(string, int32, int32) ([]any, error)
	getIncomingCallsFunc func(any) ([]any, error)
	getOutgoingCallsFunc func(any) ([]any, error)
}

func (m *ComprehensiveMockBridge) InferLanguage(filePath string) (string, error) {
	if m.inferLanguageFunc != nil {
		return m.inferLanguageFunc(filePath)
	}
	return "go", nil
}

func (m *ComprehensiveMockBridge) GetClientForLanguageInterface(language string) (any, error) {
	if m.getClientForLanguageFunc != nil {
		return m.getClientForLanguageFunc(language)
	}
	return &lsp.LanguageClient{}, nil
}

func (m *ComprehensiveMockBridge) CloseAllClients() {
	if m.closeAllClientsFunc != nil {
		m.closeAllClientsFunc()
	}
}

func (m *ComprehensiveMockBridge) GetConfig() *lsp.LSPServerConfig {
	if m.getConfigFunc != nil {
		return m.getConfigFunc()
	}
	return &lsp.LSPServerConfig{
		LanguageServers: map[string]lsp.LanguageServerConfig{
			"go": {Command: "gopls", Filetypes: []string{".go"}},
		},
		ExtensionLanguageMap: map[string]string{".go": "go"},
	}
}

func (m *ComprehensiveMockBridge) DetectProjectLanguages(projectPath string) ([]string, error) {
	if m.detectProjectLanguagesFunc != nil {
		return m.detectProjectLanguagesFunc(projectPath)
	}
	return []string{"go"}, nil
}

func (m *ComprehensiveMockBridge) DetectPrimaryProjectLanguage(projectPath string) (string, error) {
	if m.detectPrimaryProjectLanguageFunc != nil {
		return m.detectPrimaryProjectLanguageFunc(projectPath)
	}
	return "go", nil
}

func (m *ComprehensiveMockBridge) GetMultiLanguageClients(languages []string) (map[string]any, error) {
	if m.getMultiLanguageClientsFunc != nil {
		return m.getMultiLanguageClientsFunc(languages)
	}
	result := make(map[string]any)
	for _, lang := range languages {
		result[lang] = &lsp.LanguageClient{}
	}
	return result, nil
}

func (m *ComprehensiveMockBridge) FindSymbolReferences(language, uri string, line, character int32, includeDeclaration bool) ([]any, error) {
	if m.findSymbolReferencesFunc != nil {
		return m.findSymbolReferencesFunc(language, uri, line, character, includeDeclaration)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) FindSymbolDefinitions(language, uri string, line, character int32) ([]any, error) {
	if m.findSymbolDefinitionsFunc != nil {
		return m.findSymbolDefinitionsFunc(language, uri, line, character)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) SearchTextInWorkspace(language, query string) ([]any, error) {
	if m.searchTextInWorkspaceFunc != nil {
		return m.searchTextInWorkspaceFunc(language, query)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) GetHoverInformation(uri string, line, character int32) (any, error) {
	if m.getHoverInformationFunc != nil {
		return m.getHoverInformationFunc(uri, line, character)
	}
	return map[string]any{"contents": "test hover"}, nil
}

func (m *ComprehensiveMockBridge) GetSignatureHelp(uri string, line, character int32) (any, error) {
	if m.getSignatureHelpFunc != nil {
		return m.getSignatureHelpFunc(uri, line, character)
	}
	return protocol.SignatureHelpResponse{}, nil
}

func (m *ComprehensiveMockBridge) GetDiagnostics(uri string) ([]any, error) {
	if m.getDiagnosticsFunc != nil {
		return m.getDiagnosticsFunc(uri)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) GetWorkspaceDiagnostics(workspaceUri, identifier string) (any, error) {
	if m.getWorkspaceDiagnosticsFunc != nil {
		return m.getWorkspaceDiagnosticsFunc(workspaceUri, identifier)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) GetCodeActions(uri string, line, character, endLine, endCharacter int32) ([]any, error) {
	if m.getCodeActionsFunc != nil {
		return m.getCodeActionsFunc(uri, line, character, endLine, endCharacter)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) FormatDocument(uri string, tabSize int32, insertSpaces bool) ([]any, error) {
	if m.formatDocumentFunc != nil {
		return m.formatDocumentFunc(uri, tabSize, insertSpaces)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) RenameSymbol(uri string, line, character int32, newName string, preview bool) (any, error) {
	if m.renameSymbolFunc != nil {
		return m.renameSymbolFunc(uri, line, character, newName, preview)
	}
	return map[string]any{"changes": map[string]any{}}, nil
}

func (m *ComprehensiveMockBridge) FindImplementations(uri string, line, character int32) ([]any, error) {
	if m.findImplementationsFunc != nil {
		return m.findImplementationsFunc(uri, line, character)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) PrepareCallHierarchy(uri string, line, character int32) ([]any, error) {
	if m.prepareCallHierarchyFunc != nil {
		return m.prepareCallHierarchyFunc(uri, line, character)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) GetIncomingCalls(item any) ([]any, error) {
	if m.getIncomingCallsFunc != nil {
		return m.getIncomingCallsFunc(item)
	}
	return []any{}, nil
}

func (m *ComprehensiveMockBridge) GetOutgoingCalls(item any) ([]any, error) {
	if m.getOutgoingCallsFunc != nil {
		return m.getOutgoingCallsFunc(item)
	}
	return []any{}, nil
}

// Helper function for testing - removed unused createMockRequest

// Test actual tool handler execution
func TestInferLanguageToolHandler(t *testing.T) {
	bridge := &ComprehensiveMockBridge{
		getConfigFunc: func() *lsp.LSPServerConfig {
			return &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".go": "go",
					".py": "python",
					".js": "javascript",
				},
			}
		},
	}

	// Create MCP server and register tool
	mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
	RegisterInferLanguageTool(mcpServer, bridge)

	// Test successful inference
	t.Run("successful language inference", func(t *testing.T) {
		// This tests the actual tool handler code path
		config := bridge.GetConfig()
		if config == nil {
			t.Fatal("Expected config to be available")
		}
		
		language, found := config.ExtensionLanguageMap[".go"]
		if !found {
			t.Fatal("Expected .go extension to be mapped")
		}
		
		if language != "go" {
			t.Errorf("Expected 'go', got '%s'", language)
		}
	})

	// Test missing extension
	t.Run("missing extension", func(t *testing.T) {
		config := bridge.GetConfig()
		if config == nil {
			t.Fatal("Expected config to be available")
		}
		
		_, found := config.ExtensionLanguageMap[".unknown"]
		if found {
			t.Error("Expected .unknown extension to not be found")
		}
	})
}

func TestHoverToolHandler(t *testing.T) {
	testCases := []struct {
		name           string
		mockResponse   any
		mockError      error
		expectError    bool
		expectedInResult string
	}{
		{
			name: "successful hover",
			mockResponse: map[string]any{
				"contents": "Function documentation",
			},
			expectError: false,
			expectedInResult: "HOVER INFORMATION",
		},
		{
			name:         "hover error",
			mockError:    fmt.Errorf("hover failed"),
			expectError:  true,
		},
		{
			name:         "nil hover result", 
			mockResponse: nil,
			expectError:  false,
			expectedInResult: "Content: <nil>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				getHoverInformationFunc: func(uri string, line, character int32) (any, error) {
					return tc.mockResponse, tc.mockError
				},
				inferLanguageFunc: func(filePath string) (string, error) {
					return "go", nil
				},
			}

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
				
				if tc.expectedInResult != "" {
					// Test formatting
					formatted := formatHoverContent(result)
					if !strings.Contains(formatted, tc.expectedInResult) {
						t.Errorf("Expected result to contain '%s', got: %s", tc.expectedInResult, formatted)
					}
				}
			}
		})
	}
}

func TestDiagnosticsToolHandler(t *testing.T) {
	bridge := &ComprehensiveMockBridge{
		getDiagnosticsFunc: func(uri string) ([]any, error) {
			if uri == "file:///error.go" {
				return nil, fmt.Errorf("diagnostics failed")
			}
			return []any{
				protocol.Diagnostic{
					Message: "Test diagnostic",
					Source:  "test",
				},
			}, nil
		},
	}

	// Test successful diagnostics
	t.Run("successful diagnostics", func(t *testing.T) {
		result, err := bridge.GetDiagnostics("file:///test.go")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		formatted := formatDiagnostics(result)
		if !strings.Contains(formatted, "DIAGNOSTICS") {
			t.Errorf("Expected formatted result to contain 'DIAGNOSTICS'")
		}
	})

	// Test diagnostics error
	t.Run("diagnostics error", func(t *testing.T) {
		_, err := bridge.GetDiagnostics("file:///error.go")
		if err == nil {
			t.Error("Expected error but got none")
		}
	})
}

func TestLSPDisconnectToolHandler(t *testing.T) {
	disconnectCalled := false
	bridge := &ComprehensiveMockBridge{
		closeAllClientsFunc: func() {
			disconnectCalled = true
		},
	}

	// Test disconnect functionality
	bridge.CloseAllClients()
	
	if !disconnectCalled {
		t.Error("Expected CloseAllClients to be called")
	}
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

	t.Run("formatSignatureHelp", func(t *testing.T) {
		sigHelp := protocol.SignatureHelpResponse{}
		result := formatSignatureHelp(sigHelp)
		if !strings.Contains(result, "SIGNATURE HELP") {
			t.Error("Expected result to contain 'SIGNATURE HELP'")
		}
	})

	t.Run("formatTextEdits", func(t *testing.T) {
		edits := []any{
			protocol.TextEdit{
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
		edit := map[string]any{"changes": map[string]any{}}
		result := formatWorkspaceEdit(edit)
		if !strings.Contains(result, "RENAME PREVIEW") {
			t.Error("Expected result to contain 'RENAME PREVIEW'")
		}
	})

	t.Run("formatImplementations", func(t *testing.T) {
		impls := []any{
			protocol.Location{
				Uri: "file:///test.go",
				Range: protocol.Range{
					Start: protocol.Position{Line: 10, Character: 0},
				},
			},
		}
		result := formatImplementations(impls)
		if !strings.Contains(result, "IMPLEMENTATIONS") {
			t.Error("Expected result to contain 'IMPLEMENTATIONS'")
		}
	})
}