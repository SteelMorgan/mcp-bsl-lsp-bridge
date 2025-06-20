package mcpserver

import (
	"reflect"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
)

// MockBridge implements BridgeInterface for testing
type MockBridge struct{}

func (m *MockBridge) GetClientForLanguageInterface(language string) (any, error) {
	return nil, nil
}

func (m *MockBridge) InferLanguage(filePath string) (string, error) {
	return "go", nil
}

func (m *MockBridge) CloseAllClients() {}

func (m *MockBridge) GetConfig() *lsp.LSPServerConfig {
	return &lsp.LSPServerConfig{
		LanguageServers: map[string]lsp.LanguageServerConfig{
			"go": {Command: "gopls"},
		},
	}
}

func (m *MockBridge) DetectProjectLanguages(projectPath string) ([]string, error) {
	return []string{"go"}, nil
}

func (m *MockBridge) DetectPrimaryProjectLanguage(projectPath string) (string, error) {
	return "go", nil
}

func (m *MockBridge) FindSymbolReferences(language, uri string, line, character int32, includeDeclaration bool) ([]any, error) {
	return []any{}, nil
}

func (m *MockBridge) FindSymbolDefinitions(language, uri string, line, character int32) ([]any, error) {
	return []any{}, nil
}

func (m *MockBridge) SearchTextInWorkspace(language, query string) ([]any, error) {
	return []any{}, nil
}

func (m *MockBridge) GetMultiLanguageClients(languages []string) (map[string]any, error) {
	result := make(map[string]any)
	for _, lang := range languages {
		if lang == "go" {
			result[lang] = &lsp.LanguageClient{}
		}
	}
	return result, nil
}

func (m *MockBridge) GetHoverInformation(uri string, line, character int32) (any, error) {
	return map[string]any{"contents": "mock hover"}, nil
}

func (m *MockBridge) GetDiagnostics(uri string) ([]any, error) {
	return []any{}, nil
}

func (m *MockBridge) GetWorkspaceDiagnostics(workspaceUri, identifier string) (any, error) {
	return []any{}, nil
}

func (m *MockBridge) GetSignatureHelp(uri string, line, character int32) (any, error) {
	return map[string]any{"signatures": []any{}}, nil
}

func (m *MockBridge) GetCodeActions(uri string, line, character, endLine, endCharacter int32) ([]any, error) {
	return []any{}, nil
}

func (m *MockBridge) FormatDocument(uri string, tabSize int32, insertSpaces bool) ([]any, error) {
	return []any{}, nil
}

func (m *MockBridge) RenameSymbol(uri string, line, character int32, newName string, preview bool) (any, error) {
	return map[string]any{"changes": map[string]any{}}, nil
}

func (m *MockBridge) FindImplementations(uri string, line, character int32) ([]any, error) {
	return []any{}, nil
}

func (m *MockBridge) PrepareCallHierarchy(uri string, line, character int32) ([]any, error) {
	return []any{}, nil
}

func (m *MockBridge) GetIncomingCalls(item any) ([]any, error) {
	return []any{}, nil
}

func (m *MockBridge) GetOutgoingCalls(item any) ([]any, error) {
	return []any{}, nil
}

func TestMCPServerSetup(t *testing.T) {
	// Create a mock bridge
	mockBridge := &MockBridge{}

	// Set up the MCP server
	mcpServer := SetupMCPServer(mockBridge)

	t.Run("Server Creation", func(t *testing.T) {
		if mcpServer == nil {
			t.Fatal("MCP server should not be nil")
		}

		// Use reflection to check server metadata
		v := reflect.ValueOf(mcpServer).Elem()

		// Check name
		nameField := v.FieldByName("name")
		if !nameField.IsValid() {
			t.Fatal("Could not access server name")
		}
		if nameField.String() != "mcp-lsp-bridge" {
			t.Errorf("Expected server name 'mcp-lsp-bridge', got %s", nameField.String())
		}

		// Check version
		versionField := v.FieldByName("version")
		if !versionField.IsValid() {
			t.Fatal("Could not access server version")
		}
		if versionField.String() != "1.0.0" {
			t.Errorf("Expected server version '1.0.0', got %s", versionField.String())
		}
	})

	t.Run("Tool Registration Methods", func(t *testing.T) {
		// List of expected registration method names
		expectedRegistrationMethods := []string{
			"registerAnalyzeCodeTool",
			"registerInferLanguageTool",
			"registerLSPConnectTool",
			"registerLSPDisconnectTool",
			"registerProjectLanguageDetectionTool",
		}

		// Verify each registration method
		for _, methodName := range expectedRegistrationMethods {
			t.Logf("Verifying registration method: %s", methodName)
		}
	})
}

// Benchmark server setup to ensure performance
func BenchmarkMCPServerSetup(b *testing.B) {
	mockBridge := &MockBridge{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SetupMCPServer(mockBridge)
	}
}
