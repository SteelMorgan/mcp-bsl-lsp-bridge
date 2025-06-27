package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test helper functions
func createTestBridge() *MCPLSPBridge {
	config := &lsp.LSPServerConfig{
		LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{},
			},
			"typescript": {
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
			},
		},
		ExtensionLanguageMap: map[string]lsp.Language{
			".go": "go",
			".ts": "typescript",
			".js": "javascript",
		},
	}

	return NewMCPLSPBridge(config, []string{"/tmp"})
}

func createTempFile(t *testing.T, name, content string) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, name)
	err := os.WriteFile(filePath, []byte(content), 0600)
	require.NoError(t, err)

	return filePath
}

// Test NewMCPLSPBridge
func TestNewMCPLSPBridge(t *testing.T) {
	config := &lsp.LSPServerConfig{
		LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
			"go": {Command: "gopls"},
		},
	}

	bridge := NewMCPLSPBridge(config, []string{"/tmp"})

	assert.NotNil(t, bridge)
	assert.NotNil(t, bridge.clients)
	assert.Equal(t, config, bridge.config)
	assert.Empty(t, bridge.clients)
}

// Test DefaultConnectionConfig
func TestDefaultConnectionConfig(t *testing.T) {
	config := DefaultConnectionConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 2*time.Second, config.RetryDelay)
	assert.Equal(t, 30*time.Second, config.TotalTimeout)
}

// Test InferLanguage
func TestInferLanguage(t *testing.T) {
	bridge := createTestBridge()

	tests := []struct {
		name     string
		filePath string
		want     lsp.Language
		wantErr  bool
	}{
		{
			name:     "Go file",
			filePath: "/path/to/file.go",
			want:     "go",
			wantErr:  false,
		},
		{
			name:     "TypeScript file",
			filePath: "/path/to/file.ts",
			want:     "typescript",
			wantErr:  false,
		},
		{
			name:     "JavaScript file",
			filePath: "/path/to/file.js",
			want:     "javascript",
			wantErr:  false,
		},
		{
			name:     "Unknown extension",
			filePath: "/path/to/file.xyz",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bridge.InferLanguage(tt.filePath)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// Test GetConfig
func TestGetConfig(t *testing.T) {
	bridge := createTestBridge()
	config := bridge.GetConfig()

	assert.NotNil(t, config)
	assert.Equal(t, bridge.config, config)
}

// Test GetServer and SetServer
func TestGetSetServer(t *testing.T) {
	bridge := createTestBridge()
	mockServer := &server.MCPServer{}

	// Initially nil
	assert.Nil(t, bridge.GetServer())

	// Set server
	bridge.SetServer(mockServer)
	assert.Equal(t, mockServer, bridge.GetServer())
}

// Test CloseAllClients
func TestCloseAllClients(t *testing.T) {
	bridge := createTestBridge()

	// Add mock clients
	mockClient1 := &mocks.MockLanguageClient{}
	mockClient2 := &mocks.MockLanguageClient{}

	mockClient1.On("Close").Return(nil)
	mockClient2.On("Close").Return(nil)

	bridge.clients["go"] = mockClient1
	bridge.clients["typescript"] = mockClient2

	bridge.CloseAllClients()

	assert.Empty(t, bridge.clients)
	mockClient1.AssertExpectations(t)
	mockClient2.AssertExpectations(t)
}

// Test normalizeURI
func TestNormalizeURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "Already has file scheme",
			uri:  "file:///path/to/file.go",
			want: "file:///path/to/file.go",
		},
		{
			name: "Has other scheme",
			uri:  "http://example.com/file.go",
			want: "http://example.com/file.go",
		},
		{
			name: "Absolute path",
			uri:  "/path/to/file.go",
			want: "file:///path/to/file.go",
		},
		{
			name: "Relative path",
			uri:  "file.go",
			want: "file://" + func() string { abs, _ := filepath.Abs("file.go"); return abs }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeURI(tt.uri)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test FindSymbolReferences
func TestFindSymbolReferences(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	// Mock the client creation and connection
	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})

	bridge.clients["go"] = mockClient

	expectedRefs := []protocol.Location{
		{
			Uri: "file:///test.go",
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 5},
			},
		},
	}

	mockClient.On("References", "file:///test.go", uint32(10), uint32(5), true).Return(expectedRefs, nil)

	result, err := bridge.FindSymbolReferences("go", "file:///test.go", 10, 5, true)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	mockClient.AssertExpectations(t)
}

// Test FindSymbolDefinitions
func TestFindSymbolDefinitions(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})

	bridge.clients["go"] = mockClient

	expectedDefs := []protocol.Or2[protocol.LocationLink, protocol.Location]{{
		Value: protocol.Location{
			Uri: "file:///test.go",
			Range: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 5, Character: 10},
			},
		},
	},
	}

	mockClient.On("Definition", "file:///test.go", uint32(10), uint32(5)).Return(expectedDefs, nil)

	result, err := bridge.FindSymbolDefinitions("go", "file:///test.go", 10, 5)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	mockClient.AssertExpectations(t)
}

// Test FindSymbolDefinitions with error (should return empty, not fail)
func TestFindSymbolDefinitionsWithError(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})

	bridge.clients["go"] = mockClient

	mockClient.On("Definition", "file:///test.go", uint32(10), uint32(5)).Return([]protocol.Or2[protocol.LocationLink, protocol.Location]{}, errors.New("definition failed"))

	result, err := bridge.FindSymbolDefinitions("go", "file:///test.go", 10, 5)

	require.NoError(t, err) // Should not error, just return empty
	assert.Empty(t, result)
	mockClient.AssertExpectations(t)
}

// Test SearchTextInWorkspace
func TestSearchTextInWorkspace(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})

	bridge.clients["go"] = mockClient

	expectedSymbols := []protocol.WorkspaceSymbol{
		{
			Name: "TestFunction",
			Kind: protocol.SymbolKindFunction,
			Location: protocol.Or2[protocol.Location, protocol.LocationUriOnly]{Value: protocol.Location{
				Uri: "file:///test.go",
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End:   protocol.Position{Line: 0, Character: 12},
				},
			},
			}},
	}

	mockClient.On("WorkspaceSymbols", "TestFunction").Return(expectedSymbols, nil)

	result, err := bridge.SearchTextInWorkspace("go", "TestFunction")

	require.NoError(t, err)
	assert.Len(t, result, 1)
	mockClient.AssertExpectations(t)
}

// Test GetDocumentSymbols
func TestGetDocumentSymbols(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
	mockClient.On("ProjectRoots").Return([]string{"."})
	mockClient.On("SetProjectRoots", []string{"."})

	mockClient.SetProjectRoots([]string{"."})

	bridge.clients["go"] = mockClient

	// Create temp file for the test
	testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")
	testURI := "file://" + testFile

	expectedSymbols := []protocol.DocumentSymbol{
		{
			Name: "main",
			Kind: protocol.SymbolKindFunction,
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 0},
				End:   protocol.Position{Line: 2, Character: 15},
			},
			SelectionRange: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 5},
				End:   protocol.Position{Line: 2, Character: 9},
			},
		},
	}

	mockClient.On("DocumentSymbols", testURI).Return(expectedSymbols, nil)

	result, err := bridge.GetDocumentSymbols(testFile)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	mockClient.AssertExpectations(t)
}

// Test GetSignatureHelp
func TestGetSignatureHelp(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
	mockClient.On("ProjectRoots").Return([]string{"/tmp"})

	bridge.clients["go"] = mockClient

	testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")
	testURI := "file://" + testFile

	expectedSigHelp := &protocol.SignatureHelp{
		Signatures: []protocol.SignatureInformation{
			{
				Label: "func(x int) int",
				Parameters: []protocol.ParameterInformation{
					{Label: protocol.Or2[string, protocol.Tuple[uint32, uint32]]{Value: "x int"}},
				},
			},
		},
		ActiveSignature: 0,
		ActiveParameter: func() **uint32 { v := uint32(0); p := &v; return &p }(),
	}

	mockClient.On("SendNotification", "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams")).Return(nil)
	mockClient.On("SignatureHelp", testURI, uint32(2), uint32(10)).Return(expectedSigHelp, nil)

	result, err := bridge.GetSignatureHelp(testFile, 2, 10)

	require.NoError(t, err)
	assert.Equal(t, expectedSigHelp, result)
	mockClient.AssertExpectations(t)
}

// Test GetHoverInformation
func TestGetHoverInformation(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
	mockClient.On("ProjectRoots").Return([]string{"/tmp"})

	bridge.clients["go"] = mockClient

	testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")
	// testURI := "file://" + testFile

	expectedHover := &protocol.Hover{
		Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{
			Value: protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: "main function",
			},
		},
		Range: &protocol.Range{
			Start: protocol.Position{Line: 2, Character: 5},
			End:   protocol.Position{Line: 2, Character: 9},
		},
	}

	mockClient.On("SendNotification", "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams")).Return(nil)
	mockClient.On("SendRequest", "textDocument/hover", mock.AnythingOfType("protocol.HoverParams"), mock.AnythingOfType("**protocol.Hover"), 5*time.Second).Return(nil).Run(func(args mock.Arguments) {
		result := args.Get(2).(**protocol.Hover)
		*result = expectedHover
	})

	result, err := bridge.GetHoverInformation(testFile, 2, 7)

	require.NoError(t, err)
	assert.Equal(t, expectedHover, result)
	mockClient.AssertExpectations(t)
}

// Test ApplyTextEditsToContent
func TestApplyTextEditsToContent(t *testing.T) {
	content := "line 1\nline 2\nline 3"

	edits := []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 1, Character: 0},
				End:   protocol.Position{Line: 1, Character: 6},
			},
			NewText: "modified line 2",
		},
	}

	result, err := applyTextEditsToContent(content, edits)

	require.NoError(t, err)

	expected := "line 1\nmodified line 2\nline 3"
	assert.Equal(t, expected, result)
}

// Test ApplyTextEditsToContent with multi-line edit
func TestApplyTextEditsToContentMultiLine(t *testing.T) {
	content := "line 1\nline 2\nline 3\nline 4"

	edits := []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 1, Character: 0},
				End:   protocol.Position{Line: 2, Character: 6},
			},
			NewText: "replaced content",
		},
	}

	result, err := applyTextEditsToContent(content, edits)

	require.NoError(t, err)

	expected := "line 1\nreplaced content\nline 4"
	assert.Equal(t, expected, result)
}

// Test RenameSymbol
func TestRenameSymbol(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
	mockClient.On("ProjectRoots").Return([]string{"/tmp"})

	bridge.clients["go"] = mockClient

	testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")
	testURI := "file://" + testFile

	expectedWorkspaceEdit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			protocol.DocumentUri(testURI): {
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 2, Character: 5},
						End:   protocol.Position{Line: 2, Character: 9},
					},
					NewText: "newMain",
				},
			},
		},
	}

	mockClient.On("SendNotification", "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams")).Return(nil)
	mockClient.On("SendRequest", "textDocument/rename", mock.AnythingOfType("protocol.RenameParams"), mock.AnythingOfType("*protocol.WorkspaceEdit"), 10*time.Second).Return(nil).Run(func(args mock.Arguments) {
		result := args.Get(2).(*protocol.WorkspaceEdit)
		*result = expectedWorkspaceEdit
	})

	result, err := bridge.RenameSymbol(testFile, 2, 7, "newMain", false)

	require.NoError(t, err)
	assert.Equal(t, &expectedWorkspaceEdit, result)
	mockClient.AssertExpectations(t)
}

// Test GetDiagnostics
func TestGetDiagnostics(t *testing.T) {
	bridge := createTestBridge()

	result, err := bridge.GetDiagnostics("file:///test.go")

	require.NoError(t, err)
	assert.Empty(t, result) // Currently returns empty
}

// Test FindImplementations
func TestFindImplementations(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
	mockClient.On("ProjectRoots").Return([]string{"/tmp"})

	bridge.clients["go"] = mockClient

	testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")
	testURI := "file://" + testFile

	expectedImpls := []protocol.Location{
		{
			Uri: protocol.DocumentUri(testURI),
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 0},
				End:   protocol.Position{Line: 2, Character: 15},
			},
		},
	}

	mockClient.On("SendNotification", "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams")).Return(nil)
	mockClient.On("Implementation", testURI, uint32(2), uint32(7)).Return(expectedImpls, nil)

	result, err := bridge.FindImplementations(testFile, 2, 7)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	mockClient.AssertExpectations(t)
}

// Test PrepareCallHierarchy
func TestPrepareCallHierarchy(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})

	bridge.clients["go"] = mockClient

	testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")
	testURI := "file://" + testFile

	expectedItems := []protocol.CallHierarchyItem{
		{
			Name: "main",
			Kind: protocol.SymbolKindFunction,
			Uri:  protocol.DocumentUri(testURI),
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 0},
				End:   protocol.Position{Line: 2, Character: 15},
			},
			SelectionRange: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 5},
				End:   protocol.Position{Line: 2, Character: 9},
			},
		},
	}

	mockClient.On("SendRequest", "textDocument/prepareCallHierarchy", mock.AnythingOfType("protocol.CallHierarchyPrepareParams"), mock.AnythingOfType("*[]protocol.CallHierarchyItem"), 5*time.Second).Return(nil).Run(func(args mock.Arguments) {
		result := args.Get(2).(*[]protocol.CallHierarchyItem)
		*result = expectedItems
	})

	result, err := bridge.PrepareCallHierarchy(testURI, 2, 7)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	mockClient.AssertExpectations(t)
}

// Test GetIncomingCalls and GetOutgoingCalls (currently return empty)
func TestGetIncomingOutgoingCalls(t *testing.T) {
	bridge := createTestBridge()

	incoming, err := bridge.GetIncomingCalls(protocol.CallHierarchyItem{})
	require.NoError(t, err)
	assert.Empty(t, incoming)

	outgoing, err := bridge.GetOutgoingCalls(protocol.CallHierarchyItem{})
	require.NoError(t, err)
	assert.Empty(t, outgoing)
}

// Test GetClientForLanguageInterface
func TestGetClientForLanguageInterface(t *testing.T) {
	bridge := createTestBridge()
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
	mockClient.On("ProjectRoots").Return([]string{"/tmp"})

	bridge.clients["go"] = mockClient

	result, err := bridge.GetClientForLanguageInterface("go")

	require.NoError(t, err)
	assert.Equal(t, mockClient, result)
}

// Test error cases
func TestFindSymbolReferencesError(t *testing.T) {
	bridge := createTestBridge()

	// Test with unknown language
	result, err := bridge.FindSymbolReferences("unknown", "file:///test.unknown", 10, 5, true)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no server configuration found")
}

func TestGetHoverInformationInvalidLanguage(t *testing.T) {
	bridge := createTestBridge()

	result, err := bridge.GetHoverInformation("test.unknown", 10, 5)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to infer language")
}
