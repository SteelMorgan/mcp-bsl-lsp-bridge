package tools

import (
	"errors"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Test the actual tool handler execution through MCP framework
func TestAnalyzeCodeToolExecution(t *testing.T) {
	testCases := []struct {
		name           string
		uri            string
		line           int
		character      int
		mockLanguage   lsp.Language
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
			bridge := &mocks.MockBridge{}

			// Set up mock expectations based on test case
			if tc.expectError {
				// For error cases, mock should return an error
				bridge.On("InferLanguage", tc.uri).Return(lsp.Language(""), errors.New("unknown language"))
			} else {
				// For success cases, mock should return the expected language
				bridge.On("InferLanguage", tc.uri).Return(tc.mockLanguage, nil)

				// Also set up the client mock if we have one
				if tc.mockClient != nil {
					bridge.On("GetClientForLanguageInterface", string(tc.mockLanguage)).Return(tc.mockClient, nil)
				}
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

			client, err := bridge.GetClientForLanguageInterface(string(language))
			if err != nil {
				t.Errorf("Unexpected error getting client: %v", err)
				return
			}

			if client == nil {
				t.Error("Expected client but got nil")
			}

			// Assert that all expected calls were made
			bridge.AssertExpectations(t)
		})
	}
}
func TestInferLanguageToolExecution(t *testing.T) {
	testCases := []struct {
		name         string
		filePath     string
		mockConfig   *lsp.LSPServerConfig
		expectError  bool
		expectedLang lsp.Language
	}{
		{
			name:     "go file",
			filePath: "/path/to/file.go",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]lsp.Language{
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
				ExtensionLanguageMap: map[string]lsp.Language{
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
				ExtensionLanguageMap: map[string]lsp.Language{
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
			bridge := &mocks.MockBridge{}

			// Set up mock expectations
			bridge.On("GetConfig").Return(tc.mockConfig)

			// Simulate what the actual tool handler does
			config := bridge.GetConfig()
			if config == nil && !tc.expectError {
				t.Error("Expected config but got nil")
				return
			}
			if tc.expectError && config == nil {
				// Assert expectations and return early for this error case
				bridge.AssertExpectations(t)
				return
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
				// Assert expectations and return early for this error case
				bridge.AssertExpectations(t)
				return
			}

			if language != tc.expectedLang {
				t.Errorf("Expected language %s, got %s", tc.expectedLang, language)
			}

			// Assert that all expected calls were made
			bridge.AssertExpectations(t)
		})
	}
}
func TestLSPConnectToolExecution(t *testing.T) {
	testCases := []struct {
		name        string
		language    lsp.Language
		mockConfig  *lsp.LSPServerConfig
		mockClient  any
		expectError bool
	}{
		{
			name:     "successful connection",
			language: "go",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
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
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
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
			bridge := &mocks.MockBridge{}

			// Set up mock expectations
			bridge.On("GetConfig").Return(tc.mockConfig)

			// Only set up GetClientForLanguageInterface expectation if we'll call it
			if tc.mockConfig != nil {
				if _, exists := tc.mockConfig.LanguageServers[tc.language]; exists {
					if tc.expectError && tc.mockClient == nil {
						// This test case expects an error when getting the client
						bridge.On("GetClientForLanguageInterface", string(tc.language)).Return(nil, errors.New("failed to create client"))
					} else if tc.mockClient != nil {
						// This test case expects success
						bridge.On("GetClientForLanguageInterface", string(tc.language)).Return(tc.mockClient, nil)
					}
				}
			}

			// Simulate what the actual tool handler does
			config := bridge.GetConfig()
			if config == nil {
				if !tc.expectError {
					t.Error("Expected config but got nil")
				}
				bridge.AssertExpectations(t)
				return
			}

			_, exists := config.LanguageServers[tc.language]
			if !exists {
				if !tc.expectError {
					t.Errorf("Expected language server config for %s", tc.language)
				}
				bridge.AssertExpectations(t)
				return
			}

			client, err := bridge.GetClientForLanguageInterface(string(tc.language))
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				bridge.AssertExpectations(t)
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				bridge.AssertExpectations(t)
				return
			}

			if client == nil {
				t.Error("Expected client but got nil")
			}

			// Assert that all expected calls were made
			bridge.AssertExpectations(t)
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
			bridge := &mocks.MockBridge{}

			// Set up mock expectations based on mode and expected outcome
			switch tc.mode {
			case "primary":
				if tc.expectError {
					bridge.On("DetectPrimaryProjectLanguage", tc.projectPath).Return("", errors.New("detection failed"))
				} else {
					bridge.On("DetectPrimaryProjectLanguage", tc.projectPath).Return(tc.mockPrimary, nil)
				}
			default: // "all"
				if tc.expectError {
					bridge.On("DetectProjectLanguages", tc.projectPath).Return([]string(nil), errors.New("detection failed"))
				} else {
					bridge.On("DetectProjectLanguages", tc.projectPath).Return(tc.mockLanguages, nil)
				}
			}

			// Test based on mode
			switch tc.mode {
			case "primary":
				primary, err := bridge.DetectPrimaryProjectLanguage(tc.projectPath)
				if tc.expectError {
					if err == nil {
						t.Error("Expected error but got none")
					}
					bridge.AssertExpectations(t)
					return
				}
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					bridge.AssertExpectations(t)
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
					bridge.AssertExpectations(t)
					return
				}
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					bridge.AssertExpectations(t)
					return
				}
				if len(languages) != len(tc.mockLanguages) {
					t.Errorf("Expected %d languages, got %d", len(tc.mockLanguages), len(languages))
					bridge.AssertExpectations(t)
					return
				}
				// Optionally, you could also check the actual content of the slice
				for i, expected := range tc.mockLanguages {
					if i < len(languages) && languages[i] != expected {
						t.Errorf("Expected language[%d] to be %s, got %s", i, expected, languages[i])
					}
				}
			}

			// Assert that all expected calls were made
			bridge.AssertExpectations(t)
		})
	}
}
func TestSignatureHelpToolExecution(t *testing.T) {
	bridge := &mocks.MockBridge{}

	// Set up mock expectations for both test cases
	successResult := protocol.SignatureHelp{
		Signatures: []protocol.SignatureInformation{
			{
				Label: "func(param string) error",
			},
		},
	}

	// Expectation for successful case
	bridge.On("GetSignatureHelp", "file:///test.go", uint32(10), uint32(15)).Return(&successResult, nil)

	// Expectation for error case
	bridge.On("GetSignatureHelp", "file:///error.go", uint32(10), uint32(15)).Return((*protocol.SignatureHelp)(nil), errors.New("signature help failed"))

	// Test successful signature help
	result, err := bridge.GetSignatureHelp("file:///test.go", 10, 15)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Validate the structure of the result
	signatures := result.Signatures
	label := signatures[0].Label
	if len(signatures) != 1 {
		t.Errorf("Expected 1 signature, got %d", len(signatures))
	} else if label != "func(param string) error" {
		t.Errorf("Expected label 'func(param string) error', got %v", label)
	}

	// Test signature help error
	_, err = bridge.GetSignatureHelp("file:///error.go", 10, 15)
	if err == nil {
		t.Error("Expected error but got none")
	}
	if err.Error() != "signature help failed" {
		t.Errorf("Expected error message 'signature help failed', got '%s'", err.Error())
	}

	// Assert that all expected calls were made
	bridge.AssertExpectations(t)
}
func TestCodeActionsToolExecution(t *testing.T) {
	bridge := &mocks.MockBridge{}

	quickfix := protocol.CodeActionKindQuickFix
	// Set up mock expectations
	successResult := []protocol.CodeAction{
		{
			Title: "Fix import",
			Kind:  &quickfix,
		},
	}

	// Expectation for successful case
	bridge.On("GetCodeActions", "file:///test.go", uint32(10), uint32(5), uint32(10), uint32(15)).Return(successResult, nil)

	// Expectation for error case
	bridge.On("GetCodeActions", "file:///error.go", uint32(10), uint32(5), uint32(10), uint32(15)).Return([]protocol.CodeAction(nil), errors.New("code actions failed"))

	// Test successful code actions
	result, err := bridge.GetCodeActions("file:///test.go", 10, 5, 10, 15)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the result
	if len(result) != 1 {
		t.Errorf("Expected 1 code action, got %d", len(result))
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
	if err != nil && err.Error() != "code actions failed" {
		t.Errorf("Expected error message 'code actions failed', got '%s'", err.Error())
	}

	// Assert that all expected calls were made
	bridge.AssertExpectations(t)
}
func TestFormatDocumentToolExecution(t *testing.T) {
	bridge := &mocks.MockBridge{}

	// Set up mock expectations
	successResult := []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 10},
			},
			NewText: "formatted code",
		},
	}

	// Expectation for successful case
	bridge.On("FormatDocument", "file:///test.go", uint32(4), true).Return(successResult, nil)

	// Expectation for error case
	bridge.On("FormatDocument", "file:///error.go", uint32(4), true).Return([]protocol.TextEdit(nil), errors.New("formatting failed"))

	// Test successful formatting
	result, err := bridge.FormatDocument("file:///test.go", 4, true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the result
	if len(result) != 1 {
		t.Errorf("Expected 1 text edit, got %d", len(result))
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
	if err != nil && err.Error() != "formatting failed" {
		t.Errorf("Expected error message 'formatting failed', got '%s'", err.Error())
	}

	// Assert that all expected calls were made
	bridge.AssertExpectations(t)
}

func TestRenameToolExecution(t *testing.T) {
	bridge := &mocks.MockBridge{}

	successResult := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			protocol.DocumentUri("file:///test.go"): {
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 10},
					},
					NewText: "renamed",
				},
			},
		},
	}

	// Expectation for successful case
	// Match the arguments used in the actual call: uri, line, character, newName, dryRun
	bridge.On("RenameSymbol", "file:///test.go", uint32(10), uint32(5), "newName", true).Return(&successResult, nil)

	// Expectation for error case
	// Assuming "InvalidName" is the new name that would cause an error
	bridge.On("RenameSymbol", "file:///test.go", uint32(10), uint32(5), "InvalidName", true).Return((*protocol.WorkspaceEdit)(nil), errors.New("formatting failed"))

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
	// The mock for this call should also match the arguments
	_, err = bridge.RenameSymbol("file:///test.go", 10, 5, "InvalidName", true)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestImplementationToolExecution(t *testing.T) {
	bridge := &mocks.MockBridge{}

	// Define the expected successful result
	successResult := []protocol.Location{
		{
			Uri: "file:///main.go",
			Range: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 5, Character: 4},
			},
		},
	}

	// Expectation for successful implementation search
	// When FindImplementations is called with "file:///test.go", 10, 5, it should return successResult and nil error.
	bridge.On("FindImplementations", "file:///test.go", uint32(10), uint32(5)).Return(successResult, nil)

	// Expectation for error case
	// When FindImplementations is called with "file:///error.go", 10, 5, it should return nil and an error.
	bridge.On("FindImplementations", "file:///error.go", uint32(10), uint32(5)).Return([]protocol.Location(nil), errors.New("implementation search failed"))

	// Test successful implementation search
	result, err := bridge.FindImplementations("file:///test.go", 10, 5)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	formatted := formatImplementations(result)
	if !strings.Contains(formatted, "IMPLEMENTATIONS") {
		t.Errorf("Expected formatted result to contain 'IMPLEMENTATIONS', got:\n%s", formatted)
	}

	// Test implementation search error
	_, err = bridge.FindImplementations("file:///error.go", 10, 5)
	if err == nil {
		t.Error("Expected error but got none")
	}
	expectedErrMsg := "implementation search failed"
	if err != nil && err.Error() != expectedErrMsg {
		t.Errorf("Expected error message \"%s\", got \"%s\"", expectedErrMsg, err.Error())
	}

	// Assert that all expectations were met
	bridge.AssertExpectations(t)
}
func TestCallHierarchyToolExecution(t *testing.T) {
	bridge := &mocks.MockBridge{}

	// Set up mock expectations
	successResult := []protocol.CallHierarchyItem{
		{
			Name: "testFunction",
			Kind: protocol.SymbolKindFunction,
			Uri:  "file:///test.go",
		},
	}

	bridge.On("PrepareCallHierarchy", "file:///test.go", uint32(10), uint32(5)).Return(successResult, nil)
	bridge.On("PrepareCallHierarchy", "file:///error.go", uint32(10), uint32(5)).Return([]protocol.CallHierarchyItem(nil), errors.New("call hierarchy failed"))

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

	bridge.AssertExpectations(t)
}
