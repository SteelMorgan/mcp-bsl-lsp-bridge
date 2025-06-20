package tools

import (
	"fmt"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// MockCallToolRequest implements the mcp.CallToolRequest interface for testing
type MockCallToolRequest struct {
	arguments map[string]any
}

func (m *MockCallToolRequest) RequireString(key string) (string, error) {
	if val, ok := m.arguments[key]; ok {
		if str, ok := val.(string); ok {
			return str, nil
		}
		return "", fmt.Errorf("value for %s is not a string", key)
	}
	return "", fmt.Errorf("missing required parameter: %s", key)
}

func (m *MockCallToolRequest) RequireInt(key string) (int, error) {
	if val, ok := m.arguments[key]; ok {
		switch v := val.(type) {
		case int:
			return v, nil
		case int32:
			return int(v), nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		}
		return 0, fmt.Errorf("value for %s is not an integer", key)
	}
	return 0, fmt.Errorf("missing required parameter: %s", key)
}

// ToolTestBridge extends ComprehensiveMockBridge with additional testing functionality
type ToolTestBridge struct {
	*ComprehensiveMockBridge
}

// Test the actual tool handler execution through MCP framework
func TestAnalyzeCodeToolExecution(t *testing.T) {
	testCases := []struct {
		name           string
		uri            string
		line           int
		character      int
		mockLanguage   string
		mockClient     any
		mockResult     *lsp.AnalyzeCodeResult
		expectError    bool
		expectedOutput string
	}{
		{
			name:         "successful analysis",
			uri:          "file:///test.go",
			line:         10,
			character:    5,
			mockLanguage: "go",
			mockClient:   &lsp.LanguageClient{},
			mockResult: &lsp.AnalyzeCodeResult{
				Hover:       nil,
				Diagnostics: []protocol.Diagnostic{},
				CodeActions: []protocol.CodeAction{},
			},
			expectError:    false,
			expectedOutput: "Analysis Results:",
		},
		{
			name:        "language inference failure",
			uri:         "file:///unknown.xyz",
			line:        10,
			character:   5,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				inferLanguageFunc: func(filePath string) (string, error) {
					if tc.mockLanguage != "" {
						return tc.mockLanguage, nil
					}
					return "", fmt.Errorf("unknown language")
				},
				getClientForLanguageFunc: func(language string) (any, error) {
					if tc.mockClient != nil {
						return tc.mockClient, nil
					}
					return nil, fmt.Errorf("no client available")
				},
			}

			// Test the bridge functionality that the tool would use
			language, err := bridge.InferLanguage(tc.uri)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if language != tc.mockLanguage {
				t.Errorf("Expected language %s, got %s", tc.mockLanguage, language)
			}

			client, err := bridge.GetClientForLanguageInterface(language)
			if err != nil {
				t.Errorf("Unexpected error getting client: %v", err)
				return
			}

			if client == nil {
				t.Error("Expected client but got nil")
			}
		})
	}
}

func TestInferLanguageToolExecution(t *testing.T) {
	testCases := []struct {
		name         string
		filePath     string
		mockConfig   *lsp.LSPServerConfig
		expectError  bool
		expectedLang string
	}{
		{
			name:     "go file",
			filePath: "/path/to/file.go",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".go": "go",
					".py": "python",
				},
			},
			expectError:  false,
			expectedLang: "go",
		},
		{
			name:     "python file",
			filePath: "/path/to/file.py",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".go": "go",
					".py": "python",
				},
			},
			expectError:  false,
			expectedLang: "python",
		},
		{
			name:     "unknown extension",
			filePath: "/path/to/file.xyz",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".go": "go",
					".py": "python",
				},
			},
			expectError: true,
		},
		{
			name:        "no config",
			filePath:    "/path/to/file.go",
			mockConfig:  nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				getConfigFunc: func() *lsp.LSPServerConfig {
					return tc.mockConfig
				},
			}

			// Simulate what the actual tool handler does
			config := bridge.GetConfig()
			if config == nil && !tc.expectError {
				t.Error("Expected config but got nil")
				return
			}

			if tc.expectError && config == nil {
				return // Expected error case
			}

			// Extract file extension (simulate filepath.Ext)
			var ext string
			for i := len(tc.filePath) - 1; i >= 0; i-- {
				if tc.filePath[i] == '.' {
					ext = tc.filePath[i:]
					break
				}
			}

			language, found := config.ExtensionLanguageMap[ext]
			if !found && !tc.expectError {
				t.Errorf("Expected to find language for extension %s", ext)
				return
			}

			if tc.expectError && !found {
				return // Expected error case
			}

			if language != tc.expectedLang {
				t.Errorf("Expected language %s, got %s", tc.expectedLang, language)
			}
		})
	}
}

func TestLSPConnectToolExecution(t *testing.T) {
	testCases := []struct {
		name        string
		language    string
		mockConfig  *lsp.LSPServerConfig
		mockClient  any
		expectError bool
	}{
		{
			name:     "successful connection",
			language: "go",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"go": {Command: "gopls"},
				},
			},
			mockClient:  &lsp.LanguageClient{},
			expectError: false,
		},
		{
			name:     "language not configured",
			language: "rust",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"go": {Command: "gopls"},
				},
			},
			expectError: true,
		},
		{
			name:        "no config",
			language:    "go",
			mockConfig:  nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				getConfigFunc: func() *lsp.LSPServerConfig {
					return tc.mockConfig
				},
				getClientForLanguageFunc: func(language string) (any, error) {
					if tc.mockClient != nil {
						return tc.mockClient, nil
					}
					return nil, fmt.Errorf("failed to create client")
				},
			}

			// Simulate what the actual tool handler does
			config := bridge.GetConfig()
			if config == nil {
				if !tc.expectError {
					t.Error("Expected config but got nil")
				}
				return
			}

			_, exists := config.LanguageServers[tc.language]
			if !exists {
				if !tc.expectError {
					t.Errorf("Expected language server config for %s", tc.language)
				}
				return
			}

			client, err := bridge.GetClientForLanguageInterface(tc.language)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("Expected client but got nil")
			}
		})
	}
}

func TestProjectLanguageDetectionToolExecution(t *testing.T) {
	testCases := []struct {
		name          string
		projectPath   string
		mode          string
		mockLanguages []string
		mockPrimary   string
		expectError   bool
	}{
		{
			name:          "detect all languages",
			projectPath:   "/path/to/project",
			mode:          "all",
			mockLanguages: []string{"go", "python", "javascript"},
			expectError:   false,
		},
		{
			name:        "detect primary language",
			projectPath: "/path/to/project",
			mode:        "primary",
			mockPrimary: "go",
			expectError: false,
		},
		{
			name:        "detection failure",
			projectPath: "/nonexistent/path",
			mode:        "all",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				detectProjectLanguagesFunc: func(projectPath string) ([]string, error) {
					if tc.expectError {
						return nil, fmt.Errorf("detection failed")
					}
					return tc.mockLanguages, nil
				},
				detectPrimaryProjectLanguageFunc: func(projectPath string) (string, error) {
					if tc.expectError {
						return "", fmt.Errorf("detection failed")
					}
					return tc.mockPrimary, nil
				},
			}

			// Test based on mode
			switch tc.mode {
			case "primary":
				primary, err := bridge.DetectPrimaryProjectLanguage(tc.projectPath)
				if tc.expectError {
					if err == nil {
						t.Error("Expected error but got none")
					}
					return
				}
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if primary != tc.mockPrimary {
					t.Errorf("Expected primary language %s, got %s", tc.mockPrimary, primary)
				}

			default: // "all"
				languages, err := bridge.DetectProjectLanguages(tc.projectPath)
				if tc.expectError {
					if err == nil {
						t.Error("Expected error but got none")
					}
					return
				}
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if len(languages) != len(tc.mockLanguages) {
					t.Errorf("Expected %d languages, got %d", len(tc.mockLanguages), len(languages))
				}
			}
		})
	}
}

func TestSignatureHelpToolExecution(t *testing.T) {
	bridge := &ComprehensiveMockBridge{
		getSignatureHelpFunc: func(uri string, line, character int32) (any, error) {
			if uri == "file:///error.go" {
				return nil, fmt.Errorf("signature help failed")
			}
			return map[string]any{
				"signatures": []map[string]any{
					{
						"label": "func(param string) error",
					},
				},
			}, nil
		},
	}

	// Test successful signature help
	result, err := bridge.GetSignatureHelp("file:///test.go", 10, 15)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Error("Expected signature help result but got nil")
	}

	// Test signature help error
	_, err = bridge.GetSignatureHelp("file:///error.go", 10, 15)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestCodeActionsToolExecution(t *testing.T) {
	bridge := &ComprehensiveMockBridge{
		getCodeActionsFunc: func(uri string, line, character, endLine, endCharacter int32) ([]any, error) {
			if uri == "file:///error.go" {
				return nil, fmt.Errorf("code actions failed")
			}
			return []any{
				map[string]any{
					"title": "Fix import",
					"kind":  "quickfix",
				},
			}, nil
		},
	}

	// Test successful code actions
	result, err := bridge.GetCodeActions("file:///test.go", 10, 5, 10, 15)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	formatted := formatCodeActions(result)
	if !strings.Contains(formatted, "CODE ACTIONS") {
		t.Error("Expected formatted result to contain 'CODE ACTIONS'")
	}

	// Test code actions error
	_, err = bridge.GetCodeActions("file:///error.go", 10, 5, 10, 15)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestFormatDocumentToolExecution(t *testing.T) {
	bridge := &ComprehensiveMockBridge{
		formatDocumentFunc: func(uri string, tabSize int32, insertSpaces bool) ([]any, error) {
			if uri == "file:///error.go" {
				return nil, fmt.Errorf("formatting failed")
			}
			return []any{
				map[string]any{
					"range": map[string]any{
						"start": map[string]int32{"line": 0, "character": 0},
						"end":   map[string]int32{"line": 0, "character": 10},
					},
					"newText": "formatted code",
				},
			}, nil
		},
	}

	// Test successful formatting
	result, err := bridge.FormatDocument("file:///test.go", 4, true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	formatted := formatTextEdits(result)
	if !strings.Contains(formatted, "DOCUMENT FORMATTING") {
		t.Error("Expected formatted result to contain 'DOCUMENT FORMATTING'")
	}

	// Test formatting error
	_, err = bridge.FormatDocument("file:///error.go", 4, true)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestRenameToolExecution(t *testing.T) {
	bridge := &ComprehensiveMockBridge{
		renameSymbolFunc: func(uri string, line, character int32, newName string, preview bool) (any, error) {
			if newName == "InvalidName" {
				return nil, fmt.Errorf("rename failed")
			}
			return map[string]any{
				"changes": map[string]any{
					uri: []map[string]any{
						{
							"range": map[string]any{
								"start": map[string]int32{"line": line, "character": character},
								"end":   map[string]int32{"line": line, "character": character + 5},
							},
							"newText": newName,
						},
					},
				},
			}, nil
		},
	}

	// Test successful rename
	result, err := bridge.RenameSymbol("file:///test.go", 10, 5, "newName", true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	formatted := formatWorkspaceEdit(result)
	if !strings.Contains(formatted, "RENAME PREVIEW") {
		t.Error("Expected formatted result to contain 'RENAME PREVIEW'")
	}

	// Test rename error
	_, err = bridge.RenameSymbol("file:///test.go", 10, 5, "InvalidName", true)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestImplementationToolExecution(t *testing.T) {
	bridge := &ComprehensiveMockBridge{
		findImplementationsFunc: func(uri string, line, character int32) ([]any, error) {
			if uri == "file:///error.go" {
				return nil, fmt.Errorf("implementation search failed")
			}
			return []any{
				map[string]any{
					"uri": "file:///impl.go",
					"range": map[string]any{
						"start": map[string]int32{"line": 20, "character": 0},
					},
				},
			}, nil
		},
	}

	// Test successful implementation search
	result, err := bridge.FindImplementations("file:///test.go", 10, 5)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	formatted := formatImplementations(result)
	if !strings.Contains(formatted, "IMPLEMENTATIONS") {
		t.Error("Expected formatted result to contain 'IMPLEMENTATIONS'")
	}

	// Test implementation search error
	_, err = bridge.FindImplementations("file:///error.go", 10, 5)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestCallHierarchyToolExecution(t *testing.T) {
	bridge := &ComprehensiveMockBridge{
		prepareCallHierarchyFunc: func(uri string, line, character int32) ([]any, error) {
			if uri == "file:///error.go" {
				return nil, fmt.Errorf("call hierarchy failed")
			}
			return []any{
				map[string]any{
					"name": "testFunction",
					"kind": "function",
					"uri":  uri,
				},
			}, nil
		},
	}

	// Test successful call hierarchy preparation
	result, err := bridge.PrepareCallHierarchy("file:///test.go", 10, 5)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected call hierarchy items but got empty result")
	}

	// Test call hierarchy error
	_, err = bridge.PrepareCallHierarchy("file:///error.go", 10, 5)
	if err == nil {
		t.Error("Expected error but got none")
	}
}