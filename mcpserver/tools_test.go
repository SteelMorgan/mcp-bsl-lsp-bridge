package mcpserver

import (
	"testing"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mcpserver/tools"

	"github.com/mark3labs/mcp-go/server"
)

// TestMockBridge is a mock implementation of the BridgeInterface for testing
type TestMockBridge struct {
	mockInferLanguage           func(string) (string, error)
	mockGetClientForLanguage    func(string) (any, error)
	mockGetConfig               func() *lsp.LSPServerConfig
	mockCloseAllClients         func()
	mockFindSymbolReferences    func(string, string, int32, int32, bool) ([]any, error)
	mockFindSymbolDefinitions   func(string, string, int32, int32) ([]any, error)
	mockSearchTextInWorkspace   func(string, string) ([]any, error)
	mockGetMultiLanguageClients func([]string) (map[string]any, error)
	mockGetHoverInformation     func(string, int32, int32) (any, error)
	mockGetDiagnostics          func(string) ([]any, error)
	mockGetWorkspaceDiagnostics func(string, string) (any, error)
	mockGetSignatureHelp        func(string, int32, int32) (any, error)
	mockGetCodeActions          func(string, int32, int32, int32, int32) ([]any, error)
	mockFormatDocument          func(string, int32, bool) ([]any, error)
	mockRenameSymbol            func(string, int32, int32, string, bool) (any, error)
	mockFindImplementations     func(string, int32, int32) ([]any, error)
	mockPrepareCallHierarchy    func(string, int32, int32) ([]any, error)
	mockGetIncomingCalls        func(any) ([]any, error)
	mockGetOutgoingCalls        func(any) ([]any, error)
}

func (m *TestMockBridge) GetClientForLanguageInterface(language string) (any, error) {
	if m.mockGetClientForLanguage != nil {
		return m.mockGetClientForLanguage(language)
	}
	return nil, nil
}

func (m *TestMockBridge) InferLanguage(filePath string) (string, error) {
	if m.mockInferLanguage != nil {
		return m.mockInferLanguage(filePath)
	}
	return "go", nil
}

func (m *TestMockBridge) CloseAllClients() {
	if m.mockCloseAllClients != nil {
		m.mockCloseAllClients()
	}
}

func (m *TestMockBridge) GetConfig() *lsp.LSPServerConfig {
	if m.mockGetConfig != nil {
		return m.mockGetConfig()
	}
	return &lsp.LSPServerConfig{
		LanguageServers: map[string]lsp.LanguageServerConfig{
			"go": {
				Command: "gopls",
				Filetypes: []string{".go"},
			},
			"python": {
				Command: "pyright",
				Filetypes: []string{".py"},
			},
		},
		ExtensionLanguageMap: map[string]string{
			".go": "go",
			".py": "python",
		},
	}
}

func (m *TestMockBridge) DetectProjectLanguages(projectPath string) ([]string, error) {
	return []string{"go"}, nil
}

func (m *TestMockBridge) DetectPrimaryProjectLanguage(projectPath string) (string, error) {
	return "go", nil
}

func (m *TestMockBridge) FindSymbolReferences(language, uri string, line, character int32, includeDeclaration bool) ([]any, error) {
	if m.mockFindSymbolReferences != nil {
		return m.mockFindSymbolReferences(language, uri, line, character, includeDeclaration)
	}
	return []any{}, nil
}

func (m *TestMockBridge) FindSymbolDefinitions(language, uri string, line, character int32) ([]any, error) {
	if m.mockFindSymbolDefinitions != nil {
		return m.mockFindSymbolDefinitions(language, uri, line, character)
	}
	return []any{}, nil
}

func (m *TestMockBridge) SearchTextInWorkspace(language, query string) ([]any, error) {
	if m.mockSearchTextInWorkspace != nil {
		return m.mockSearchTextInWorkspace(language, query)
	}
	return []any{}, nil
}

func (m *TestMockBridge) GetMultiLanguageClients(languages []string) (map[string]any, error) {
	if m.mockGetMultiLanguageClients != nil {
		return m.mockGetMultiLanguageClients(languages)
	}
	result := make(map[string]any)
	for _, lang := range languages {
		if lang == "go" {
			result[lang] = &lsp.LanguageClient{}
		}
	}
	return result, nil
}

func (m *TestMockBridge) GetHoverInformation(uri string, line, character int32) (any, error) {
	if m.mockGetHoverInformation != nil {
		return m.mockGetHoverInformation(uri, line, character)
	}
	return map[string]any{
		"contents": "Test hover information",
		"range": map[string]any{
			"start": map[string]int32{"line": line, "character": character},
			"end":   map[string]int32{"line": line, "character": character + 1},
		},
	}, nil
}

func (m *TestMockBridge) GetDiagnostics(uri string) ([]any, error) {
	if m.mockGetDiagnostics != nil {
		return m.mockGetDiagnostics(uri)
	}
	return []any{}, nil
}

func (m *TestMockBridge) GetWorkspaceDiagnostics(workspaceUri, identifier string) (any, error) {
	if m.mockGetWorkspaceDiagnostics != nil {
		return m.mockGetWorkspaceDiagnostics(workspaceUri, identifier)
	}
	return []any{}, nil
}

func (m *TestMockBridge) GetSignatureHelp(uri string, line, character int32) (any, error) {
	if m.mockGetSignatureHelp != nil {
		return m.mockGetSignatureHelp(uri, line, character)
	}
	return map[string]any{
		"signatures": []map[string]any{
			{
				"label": "testFunction(param1 string, param2 int) error",
				"parameters": []map[string]any{
					{"label": "param1 string"},
					{"label": "param2 int"},
				},
			},
		},
		"activeSignature": 0,
		"activeParameter": 0,
	}, nil
}

func (m *TestMockBridge) GetCodeActions(uri string, line, character, endLine, endCharacter int32) ([]any, error) {
	if m.mockGetCodeActions != nil {
		return m.mockGetCodeActions(uri, line, character, endLine, endCharacter)
	}
	return []any{
		map[string]any{
			"title": "Test code action",
			"kind":  "quickfix",
		},
	}, nil
}

func (m *TestMockBridge) FormatDocument(uri string, tabSize int32, insertSpaces bool) ([]any, error) {
	if m.mockFormatDocument != nil {
		return m.mockFormatDocument(uri, tabSize, insertSpaces)
	}
	return []any{
		map[string]any{
			"range": map[string]any{
				"start": map[string]int32{"line": 0, "character": 0},
				"end":   map[string]int32{"line": 0, "character": 10},
			},
			"newText": "formatted text",
		},
	}, nil
}

func (m *TestMockBridge) RenameSymbol(uri string, line, character int32, newName string, preview bool) (any, error) {
	if m.mockRenameSymbol != nil {
		return m.mockRenameSymbol(uri, line, character, newName, preview)
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
}

func (m *TestMockBridge) FindImplementations(uri string, line, character int32) ([]any, error) {
	if m.mockFindImplementations != nil {
		return m.mockFindImplementations(uri, line, character)
	}
	return []any{
		map[string]any{
			"uri": uri,
			"range": map[string]any{
				"start": map[string]int32{"line": line + 5, "character": 0},
				"end":   map[string]int32{"line": line + 5, "character": 10},
			},
		},
	}, nil
}

func (m *TestMockBridge) PrepareCallHierarchy(uri string, line, character int32) ([]any, error) {
	if m.mockPrepareCallHierarchy != nil {
		return m.mockPrepareCallHierarchy(uri, line, character)
	}
	return []any{
		map[string]any{
			"name": "testFunction",
			"kind": "function",
			"uri":  uri,
			"range": map[string]any{
				"start": map[string]int32{"line": line, "character": character},
				"end":   map[string]int32{"line": line, "character": character + 10},
			},
		},
	}, nil
}

func (m *TestMockBridge) GetIncomingCalls(item any) ([]any, error) {
	if m.mockGetIncomingCalls != nil {
		return m.mockGetIncomingCalls(item)
	}
	return []any{}, nil
}

func (m *TestMockBridge) GetOutgoingCalls(item any) ([]any, error) {
	if m.mockGetOutgoingCalls != nil {
		return m.mockGetOutgoingCalls(item)
	}
	return []any{}, nil
}

// TestRegisterAllTools tests that all tools can be registered without errors
func TestRegisterAllTools(t *testing.T) {
	mockBridge := &TestMockBridge{
		mockInferLanguage: func(path string) (string, error) {
			if path == "/test/example.go" || path == "/test/file.go" {
				return "go", nil
			}
			return "unknown", nil
		},
		mockGetClientForLanguage: func(language string) (any, error) {
			if language == "go" {
				return &lsp.LanguageClient{}, nil
			}
			return nil, nil
		},
		mockGetConfig: func() *lsp.LSPServerConfig {
			return &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"go": {Command: "gopls"},
				},
			}
		},
		mockCloseAllClients: func() {},
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"lsp-bridge-mcp",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Test that RegisterAllTools works without panicking
	RegisterAllTools(mcpServer, mockBridge)

	// No assertion needed - test passes if no panic occurs
}

// TestIndividualToolRegistration tests individual tool registration functions
func TestIndividualToolRegistration(t *testing.T) {
	testCases := []struct {
		name             string
		toolRegistration func(*server.MCPServer, interfaces.BridgeInterface)
	}{
		{
			name: "Analyze Code Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterAnalyzeCodeTool(mcpServer, bridge)
			},
		},
		{
			name: "Project Analysis Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterProjectAnalysisTool(mcpServer, bridge)
			},
		},
		{
			name: "Infer Language Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterInferLanguageTool(mcpServer, bridge)
			},
		},
		{
			name: "LSP Connect Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterLSPConnectTool(mcpServer, bridge)
			},
		},
		{
			name: "LSP Disconnect Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterLSPDisconnectTool(mcpServer, bridge)
			},
		},
		{
			name: "Detect Project Languages Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterProjectLanguageDetectionTool(mcpServer, bridge)
			},
		},
		{
			name: "Hover Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterHoverTool(mcpServer, bridge)
			},
		},
		{
			name: "Diagnostics Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterDiagnosticsTool(mcpServer, bridge)
			},
		},
		{
			name: "Signature Help Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterSignatureHelpTool(mcpServer, bridge)
			},
		},
		{
			name: "Code Actions Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterCodeActionsTool(mcpServer, bridge)
			},
		},
		{
			name: "Format Document Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterFormatDocumentTool(mcpServer, bridge)
			},
		},
		{
			name: "Rename Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterRenameTool(mcpServer, bridge)
			},
		},
		{
			name: "Implementation Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterImplementationTool(mcpServer, bridge)
			},
		},
		{
			name: "Call Hierarchy Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterCallHierarchyTool(mcpServer, bridge)
			},
		},
		{
			name: "Workspace Diagnostics Tool",
			toolRegistration: func(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
				tools.RegisterWorkspaceDiagnosticsTool(mcpServer, bridge)
			},
		},
	}

	// Run each test case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare mock bridge
			mockBridge := &TestMockBridge{
				mockInferLanguage: func(path string) (string, error) {
					if path == "/test/example.go" || path == "/test/file.go" {
						return "go", nil
					}
					return "unknown", nil
				},
				mockGetClientForLanguage: func(language string) (any, error) {
					if language == "go" {
						return &lsp.LanguageClient{}, nil
					}
					return nil, nil
				},
				mockGetConfig: func() *lsp.LSPServerConfig {
					return &lsp.LSPServerConfig{
						LanguageServers: map[string]lsp.LanguageServerConfig{
							"go": {Command: "gopls"},
						},
					}
				},
				mockCloseAllClients: func() {},
			}

			// Create MCP server
			mcpServer := server.NewMCPServer(
				"lsp-bridge-mcp",
				"1.0.0",
				server.WithToolCapabilities(false),
			)

			// Register the tool - test passes if it doesn't panic
			tc.toolRegistration(mcpServer, mockBridge)
		})
	}
}

// Benchmark tool registration performance
func BenchmarkMCPServerToolRegistration(b *testing.B) {
	mockBridge := &TestMockBridge{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mcpServer := server.NewMCPServer(
			"lsp-bridge-mcp",
			"1.0.0",
			server.WithToolCapabilities(false),
		)
		RegisterAllTools(mcpServer, mockBridge)
	}
}