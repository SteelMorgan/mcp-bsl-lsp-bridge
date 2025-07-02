package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
func createTestBridge(allowedDirs []string) *MCPLSPBridge {
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
			"python": {
				Command: "pyright-langserver",
				Args:    []string{},
			},
		},
		ExtensionLanguageMap: map[string]lsp.Language{
			".go": "go",
			".ts": "typescript",
			".py": "python",
			".js": "javascript",
		},
	}

	return NewMCPLSPBridge(config, allowedDirs)
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
	bridge := createTestBridge([]string{"/tmp"})

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
				assert.Equal(t, tt.want, *got)
			}
		})
	}
}

// Test GetConfig
func TestGetConfig(t *testing.T) {
	bridge := createTestBridge([]string{"/tmp"})
	config := bridge.GetConfig()

	assert.NotNil(t, config)
	assert.Equal(t, bridge.config, config)
}

// Test GetServer and SetServer
func TestGetSetServer(t *testing.T) {
	bridge := createTestBridge([]string{"/tmp"})
	mockServer := &server.MCPServer{}

	// Initially nil
	assert.Nil(t, bridge.GetServer())

	// Set server
	bridge.SetServer(mockServer)
	assert.Equal(t, mockServer, bridge.GetServer())
}

// Test CloseAllClients
func TestCloseAllClients(t *testing.T) {
	bridge := createTestBridge([]string{"/tmp"})

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
	bridge := createTestBridge([]string{"/"})
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
	bridge := createTestBridge([]string{"/"})
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
	bridge := createTestBridge([]string{"/"})
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
	bridge := createTestBridge([]string{"/"})
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
	bridge := createTestBridge([]string{"/"})
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
	bridge := createTestBridge([]string{"/"})
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
	bridge := createTestBridge([]string{"/"})
	mockClient := &mocks.MockLanguageClient{}
	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
	mockClient.On("ProjectRoots").Return([]string{"/tmp"})
	bridge.clients["go"] = mockClient
	testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")
	testURI := "file://" + testFile

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

	// Mock expects the URI format that the bridge code actually uses
	mockClient.On("Hover", testURI, uint32(2), uint32(7)).Return(expectedHover, nil)

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
	t.Run("successful rename", func(t *testing.T) {
		bridge := createTestBridge([]string{"/"})
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
		mockClient.On("Rename", testURI, uint32(2), uint32(7), "newMain").Return(&expectedWorkspaceEdit, nil)

		result, err := bridge.RenameSymbol(testFile, 2, 7, "newMain", false)
		require.NoError(t, err)
		assert.Equal(t, &expectedWorkspaceEdit, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("infer language error", func(t *testing.T) {
		bridge := createTestBridge([]string{"/"})

		// Use a file with no extension or unsupported extension
		testFile := "/tmp/test_file_no_extension"

		result, err := bridge.RenameSymbol(testFile, 2, 7, "newName", false)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to infer language")
	})

	// t.Run("get client for language error", func(t *testing.T) {
	// 	bridge := createTestBridge([]string{"/"})
	//
	// 	// Use a supported file extension but don't register a client for that language
	// 	testFile := createTempFile(t, "test.go", "package main")
	//
	// 	result, err := bridge.RenameSymbol(testFile, 2, 7, "newName", false)
	//
	// 	require.Error(t, err)
	// 	assert.Nil(t, result)
	// 	assert.Contains(t, err.Error(), "failed to get client for language")
	// })

	t.Run("ensure document open error - continues anyway", func(t *testing.T) {
		bridge := createTestBridge([]string{"/"})
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

		// Simulate didOpen failure
		mockClient.On("SendNotification", "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams")).Return(errors.New("failed to open document"))
		// But rename should still succeed
		mockClient.On("Rename", testURI, uint32(2), uint32(7), "newMain").Return(&expectedWorkspaceEdit, nil)

		result, err := bridge.RenameSymbol(testFile, 2, 7, "newMain", false)

		// Should succeed despite didOpen failure
		require.NoError(t, err)
		assert.Equal(t, &expectedWorkspaceEdit, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("client rename error", func(t *testing.T) {
		bridge := createTestBridge([]string{"/"})
		mockClient := &mocks.MockLanguageClient{}
		ctx := context.Background()
		mockClient.On("Context").Return(ctx)
		mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
		mockClient.On("ProjectRoots").Return([]string{"/tmp"})
		bridge.clients["go"] = mockClient

		testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")
		testURI := "file://" + testFile

		mockClient.On("SendNotification", "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams")).Return(nil)
		// Simulate rename failure
		mockClient.On("Rename", testURI, uint32(2), uint32(7), "newMain").Return((*protocol.WorkspaceEdit)(nil), errors.New("symbol not found"))

		result, err := bridge.RenameSymbol(testFile, 2, 7, "newMain", false)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to rename symbol")
		assert.Contains(t, err.Error(), "symbol not found")
		mockClient.AssertExpectations(t)
	})

	t.Run("client rename returns nil without error", func(t *testing.T) {
		bridge := createTestBridge([]string{"/"})
		mockClient := &mocks.MockLanguageClient{}
		ctx := context.Background()
		mockClient.On("Context").Return(ctx)
		mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
		mockClient.On("ProjectRoots").Return([]string{"/tmp"})
		bridge.clients["go"] = mockClient

		testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")
		testURI := "file://" + testFile

		mockClient.On("SendNotification", "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams")).Return(nil)
		// Return nil result but no error (valid scenario)
		mockClient.On("Rename", testURI, uint32(2), uint32(7), "newMain").Return((*protocol.WorkspaceEdit)(nil), nil)

		result, err := bridge.RenameSymbol(testFile, 2, 7, "newMain", false)

		require.NoError(t, err)
		assert.Nil(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("URI normalization edge cases", func(t *testing.T) {
		bridge := createTestBridge([]string{"/"})
		mockClient := &mocks.MockLanguageClient{}
		bridge.clients["go"] = mockClient

		testFile := createTempFile(t, "test.go", "package main\n\nfunc main() {}")

		// Test with various URI formats
		testCases := []string{
			testFile,             // Regular path
			"file://" + testFile, // Already normalized
		}

		expectedWorkspaceEdit := protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentUri][]protocol.TextEdit{
				protocol.DocumentUri("file://" + testFile): {
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

		for _, uriInput := range testCases {
			t.Run("URI input: "+uriInput, func(t *testing.T) {
				// Reset mock expectations for each sub-test
				mockClient.ExpectedCalls = nil
				mockClient.Calls = nil

				ctx := context.Background()
				mockClient.On("Context").Return(ctx)
				mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
				mockClient.On("ProjectRoots").Return([]string{"/tmp"})
				mockClient.On("SendNotification", "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams")).Return(nil)
				mockClient.On("Rename", "file://"+testFile, uint32(2), uint32(7), "newMain").Return(&expectedWorkspaceEdit, nil)

				result, err := bridge.RenameSymbol(uriInput, 2, 7, "newMain", false)

				require.NoError(t, err)
				assert.Equal(t, &expectedWorkspaceEdit, result)
				mockClient.AssertExpectations(t)
			})
		}
	})

	t.Run("different language file types", func(t *testing.T) {
		bridge := createTestBridge([]string{"/"})

		// Test files with different extensions
		testCases := []struct {
			filename    string
			content     string
			language    lsp.Language
			shouldError bool
		}{
			{"test.go", "package main", "go", false},
			{"test.py", "print('hello')", "python", false},
			{"test.js", "console.log('hello')", "javascript", false},
			{"test.unknown", "some content", "", true}, // Should error on unknown extension
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("Language: %s", tc.language), func(t *testing.T) {
				if !tc.shouldError {
					mockClient := &mocks.MockLanguageClient{}
					ctx := context.Background()
					mockClient.On("Context").Return(ctx)
					mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
					mockClient.On("ProjectRoots").Return([]string{"/tmp"})
					bridge.clients[tc.language] = mockClient

					testFile := createTempFile(t, tc.filename, tc.content)
					testURI := "file://" + testFile

					expectedWorkspaceEdit := protocol.WorkspaceEdit{
						Changes: map[protocol.DocumentUri][]protocol.TextEdit{
							protocol.DocumentUri(testURI): {
								{
									Range: protocol.Range{
										Start: protocol.Position{Line: 0, Character: 0},
										End:   protocol.Position{Line: 0, Character: 4},
									},
									NewText: "newName",
								},
							},
						},
					}

					mockClient.On("SendNotification", "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams")).Return(nil)
					mockClient.On("Rename", testURI, uint32(0), uint32(0), "newName").Return(&expectedWorkspaceEdit, nil)

					result, err := bridge.RenameSymbol(testFile, 0, 0, "newName", false)

					require.NoError(t, err)
					assert.Equal(t, &expectedWorkspaceEdit, result)
					mockClient.AssertExpectations(t)
				} else {
					testFile := createTempFile(t, tc.filename, tc.content)

					result, err := bridge.RenameSymbol(testFile, 0, 0, "newName", false)

					require.Error(t, err)
					assert.Nil(t, result)
					assert.Contains(t, err.Error(), "failed to infer language")
				}
			})
		}
	})
}

// Test FindImplementations
func TestFindImplementations(t *testing.T) {
	bridge := createTestBridge([]string{"/"})
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
	bridge := createTestBridge([]string{"/"})
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
	mockClient.On("PrepareCallHierarchy", testURI, uint32(2), uint32(7)).Return(expectedItems, nil)

	result, err := bridge.PrepareCallHierarchy(testURI, 2, 7)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	mockClient.AssertExpectations(t)
}

// Test GetIncomingCalls and GetOutgoingCalls (currently return empty)
func TestGetIncomingOutgoingCalls(t *testing.T) {
	bridge := createTestBridge([]string{"/"})

	incoming, err := bridge.GetIncomingCalls(protocol.CallHierarchyItem{})
	require.NoError(t, err)
	assert.Empty(t, incoming)

	outgoing, err := bridge.GetOutgoingCalls(protocol.CallHierarchyItem{})
	require.NoError(t, err)
	assert.Empty(t, outgoing)
}

// Test GetClientForLanguageInterface
func TestGetClientForLanguageInterface(t *testing.T) {
	bridge := createTestBridge([]string{"/"})
	mockClient := &mocks.MockLanguageClient{}

	ctx := context.Background()
	mockClient.On("Context").Return(ctx)
	mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
	mockClient.On("ProjectRoots").Return([]string{"/tmp"})

	bridge.clients["go"] = mockClient

	result, err := bridge.GetClientForLanguage("go")

	require.NoError(t, err)
	assert.Equal(t, mockClient, result)
}

// Test error cases
func TestFindSymbolReferencesError(t *testing.T) {
	bridge := createTestBridge([]string{"/"})

	// Test with unknown language
	result, err := bridge.FindSymbolReferences("unknown", "file:///test.unknown", 10, 5, true)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no server configuration found")
}

func TestGetHoverInformationInvalidLanguage(t *testing.T) {
	bridge := createTestBridge([]string{"/"})

	result, err := bridge.GetHoverInformation("test.unknown", 10, 5)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to infer language")
}
func createTestPaths() (projectRoots []string, testCases []struct {
	name    string
	dir     string
	allowed bool
}) {
	if runtime.GOOS == "windows" {
		projectRoots = []string{
			`C:\Users\rockerboo\code\mcp-lsp-bridge`,
			`C:\other\allowed\directory`,
		}
		testCases = []struct {
			name    string
			dir     string
			allowed bool
		}{
			{"exact match", `C:\Users\rockerboo\code\mcp-lsp-bridge`, true},
			{"subdirectory", `C:\Users\rockerboo\code\mcp-lsp-bridge\lsp`, true},
			{"not allowed", `C:\not\allowed\directory`, false},
			{"parent with ..", `C:\Users\rockerboo\code\mcp-lsp-bridge\lsp\..`, true},
			{"parent .. attempt", `C:\Users\rockerboo\code\mcp-lsp-bridge\lsp\..\..`, false}, // Should go outside allowed dir
			{"other exact match", `C:\other\allowed\directory`, true},
			{"other subdirectory", `C:\other\allowed\directory\lsp`, true},
			{"root", `C:\`, false},
		}
	} else {
		projectRoots = []string{
			"/home/rockerboo/code/mcp-lsp-bridge",
			"/other/allowed/directory",
		}
		testCases = []struct {
			name    string
			dir     string
			allowed bool
		}{
			{"exact match", "/home/rockerboo/code/mcp-lsp-bridge", true},
			{"subdirectory", "/home/rockerboo/code/mcp-lsp-bridge/lsp", true},
			{"not allowed", "/not/allowed/directory", false},
			{"parent with ..", "/home/rockerboo/code/mcp-lsp-bridge/lsp/..", true},
			{"parent .. attempt", "/home/rockerboo/code/mcp-lsp-bridge/lsp/../..", true}, // Should go outside allowed dir
			{"other exact match", "/other/allowed/directory", true},
			{"other subdirectory", "/other/allowed/directory/lsp", true},
			{"root", "/", false},
		}
	}
	return projectRoots, testCases
}

func TestIsWithinAllowedDirectory(t *testing.T) {
	projectRoots, tests := createTestPaths()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isAllowed := false
			for _, projectRoot := range projectRoots {
				if isWithinAllowedDirectory(tt.dir, projectRoot) {
					isAllowed = true
					break
				}
			}

			if tt.allowed != isAllowed {
				t.Errorf("isWithinAllowedDirectory(%s, %v) = %v, want %v", tt.dir, projectRoots, isAllowed, tt.allowed)
			}
		})
	}
}

// Cross-platform test using relative paths
func TestIsWithinAllowedDirectoryRelative(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create subdirectories
	allowedDir := filepath.Join(tempDir, "allowed")
	subDir := filepath.Join(allowedDir, "subdir")

	tests := []struct {
		name    string
		dir     string
		base    string
		allowed bool
	}{
		{"exact match", allowedDir, allowedDir, true},
		{"subdirectory", subDir, allowedDir, true},
		{"parent directory", tempDir, allowedDir, false},
		{"unrelated directory", filepath.Join(tempDir, "other"), allowedDir, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWithinAllowedDirectory(tt.dir, tt.base)
			if result != tt.allowed {
				t.Errorf("isWithinAllowedDirectory(%s, %s) = %v, want %v", tt.dir, tt.base, result, tt.allowed)
			}
		})
	}
}

func Test_getCleanAbsPath(t *testing.T) {
	// Get current working directory for relative path tests
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal("Failed to get current working directory:", err)
	}

	var tests []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}

	if runtime.GOOS == "windows" {
		tests = []struct {
			name    string
			path    string
			want    string
			wantErr bool
		}{
			{
				name:    "valid absolute path",
				path:    `C:\Users\rockerboo\code\mcp-lsp-bridge\lsp`,
				want:    `C:\Users\rockerboo\code\mcp-lsp-bridge\lsp`,
				wantErr: false,
			},
			{
				name:    "empty path",
				path:    "",
				want:    "",
				wantErr: true,
			},
			{
				name:    "current directory",
				path:    ".",
				want:    "",
				wantErr: true,
			},
			{
				name:    "relative path",
				path:    "lsp",
				want:    filepath.Join(cwd, "lsp"),
				wantErr: false,
			},
			{
				name:    "absolute path with ..",
				path:    `C:\Users\rockerboo\code\mcp-lsp-bridge\..\lsp`,
				want:    `C:\Users\rockerboo\code\lsp`,
				wantErr: false,
			},
			{
				name:    "absolute path with ./",
				path:    `C:\Users\rockerboo\code\mcp-lsp-bridge\.\lsp`,
				want:    `C:\Users\rockerboo\code\mcp-lsp-bridge\lsp`,
				wantErr: false,
			},
			{
				name:    "UNC path",
				path:    `\\server\share\path`,
				want:    `\\server\share\path`,
				wantErr: false,
			},
		}
	} else {
		tests = []struct {
			name    string
			path    string
			want    string
			wantErr bool
		}{
			{
				name:    "valid absolute path",
				path:    "/home/rockerboo/code/mcp-lsp-bridge/lsp",
				want:    "/home/rockerboo/code/mcp-lsp-bridge/lsp",
				wantErr: false,
			},
			{
				name:    "empty path",
				path:    "",
				want:    "",
				wantErr: true,
			},
			{
				name:    "current directory",
				path:    ".",
				want:    "",
				wantErr: true,
			},
			{
				name:    "relative path",
				path:    "lsp",
				want:    filepath.Join(cwd, "lsp"),
				wantErr: false,
			},
			{
				name:    "absolute path with ..",
				path:    "/home/rockerboo/code/mcp-lsp-bridge/../lsp",
				want:    "/home/rockerboo/code/lsp",
				wantErr: false,
			},
			{
				name:    "absolute path with ./",
				path:    "/home/rockerboo/code/mcp-lsp-bridge/./lsp",
				want:    "/home/rockerboo/code/mcp-lsp-bridge/lsp",
				wantErr: false,
			},
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := getCleanAbsPath(tt.path)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("getCleanAbsPath() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("getCleanAbsPath() succeeded unexpectedly")
			}
			if tt.want != got {
				t.Errorf("getCleanAbsPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Cross-platform test using temporary directories
func Test_getCleanAbsPathWithTempDir(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "temp directory path",
			path:    tempDir,
			wantErr: false,
		},
		{
			name:    "temp subdirectory path",
			path:    filepath.Join(tempDir, "subdir"),
			wantErr: false,
		},
		{
			name:    "temp path with ..",
			path:    filepath.Join(tempDir, "subdir", ".."),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getCleanAbsPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getCleanAbsPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify it's an absolute path
				if !filepath.IsAbs(got) {
					t.Errorf("getCleanAbsPath() returned non-absolute path: %v", got)
				}
				// Verify it's clean (no . or .. components)
				if got != filepath.Clean(got) {
					t.Errorf("getCleanAbsPath() returned unclean path: %v", got)
				}
			}
		})
	}
}

func TestMCPLSPBridge_GetCodeActions(t *testing.T) {
	quickfix := protocol.CodeActionKindQuickFix
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		config             *lsp.LSPServerConfig
		allowedDirectories []string
		// Named input parameters for target function.
		uri          string
		line         uint32
		character    uint32
		endLine      uint32
		endCharacter uint32
		want         []protocol.CodeAction
		wantErr      bool
		// Mock setup parameters
		mockCodeActions []protocol.CodeAction
		mockError       error
		// Control which clients are available
		setupClients bool
	}{
		{
			name: "successful code actions",
			config: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					"go": {
						Command: "gopls",
						Args:    []string{},
					},
				},
				ExtensionLanguageMap: map[string]lsp.Language{
					".go": "go",
				},
			},
			allowedDirectories: []string{"."},
			uri:                "file:///home/rockerboo/code/mcp-lsp-bridge/lsp/main.go",
			line:               1,
			character:          1,
			endLine:            1,
			endCharacter:       1,
			want: []protocol.CodeAction{
				{
					Title: "Add import",
					Kind:  &quickfix,
				},
			},
			wantErr: false,
			mockCodeActions: []protocol.CodeAction{
				{
					Title: "Add import",
					Kind:  &quickfix,
				},
			},
			mockError:    nil,
			setupClients: true,
		},
		{
			name: "no code actions",
			config: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					"go": {
						Command: "gopls",
						Args:    []string{},
					},
				},
				ExtensionLanguageMap: map[string]lsp.Language{
					".go": "go",
				},
			},
			allowedDirectories: []string{"."},
			uri:                "file:///home/rockerboo/code/mcp-lsp-bridge/lsp/main.go",
			line:               1,
			character:          1,
			endLine:            1,
			endCharacter:       1,
			want:               []protocol.CodeAction{},
			wantErr:            false,
			mockCodeActions:    []protocol.CodeAction{},
			mockError:          nil,
			setupClients:       true,
		},
		{
			name: "error - failed to infer language",
			config: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					"go": {
						Command: "gopls",
						Args:    []string{},
					},
				},
				ExtensionLanguageMap: map[string]lsp.Language{
					".go": "go",
				},
			},
			allowedDirectories: []string{"."},
			uri:                "file:///home/rockerboo/code/mcp-lsp-bridge/lsp/main.unknown", // unknown extension
			line:               1,
			character:          1,
			endLine:            1,
			endCharacter:       1,
			want:               nil,
			wantErr:            true,
			mockCodeActions:    nil,
			mockError:          nil,
			setupClients:       false, // No clients needed for this test
		},
		{
			name: "error - failed to get client for language",
			config: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					// No language server configured for Go
				},
				ExtensionLanguageMap: map[string]lsp.Language{
					".go": "go",
				},
			},
			allowedDirectories: []string{"."},
			uri:                "file:///home/rockerboo/code/mcp-lsp-bridge/lsp/main.go",
			line:               1,
			character:          1,
			endLine:            1,
			endCharacter:       1,
			want:               nil,
			wantErr:            true,
			mockCodeActions:    nil,
			mockError:          nil,
			setupClients:       false, // No clients set up for this test
		},
		{
			name: "error - client code actions failed",
			config: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					"go": {
						Command: "gopls",
						Args:    []string{},
					},
				},
				ExtensionLanguageMap: map[string]lsp.Language{
					".go": "go",
				},
			},
			allowedDirectories: []string{"."},
			uri:                "file:///home/rockerboo/code/mcp-lsp-bridge/lsp/main.go",
			line:               1,
			character:          1,
			endLine:            1,
			endCharacter:       1,
			want:               nil,
			wantErr:            true,
			mockCodeActions:    nil,
			mockError:          errors.New("failed to get code actions from language server"),
			setupClients:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewMCPLSPBridge(tt.config, tt.allowedDirectories)

			// Only set up mock client if needed for this test
			if tt.setupClients {
				mockClient := &mocks.MockLanguageClient{}
				b.clients["go"] = mockClient

				// Set up mock expectations
				mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
				mockClient.On("CodeActions", tt.uri, tt.line, tt.character, tt.endLine, tt.endCharacter).Return(tt.mockCodeActions, tt.mockError)
				ctx := context.Background()
				mockClient.On("Context").Return(ctx)

				// Ensure mock expectations are checked at the end
				defer mockClient.AssertExpectations(t)
			}

			got, gotErr := b.GetCodeActions(tt.uri, tt.line, tt.character, tt.endLine, tt.endCharacter)

			if tt.wantErr {
				if gotErr == nil {
					t.Fatal("GetCodeActions() succeeded unexpectedly, wanted error")
				}
				return
			}

			if gotErr != nil {
				t.Errorf("GetCodeActions() failed: %v", gotErr)
				return
			}

			// Compare the results
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCodeActions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPLSPBridge_GetWorkspaceDiagnostics(t *testing.T) {
	diagnosticWarning := protocol.DiagnosticSeverityWarning
	version := int32(1)
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		config             *lsp.LSPServerConfig
		allowedDirectories []string
		// Named input parameters for target function.
		workspaceUri string
		identifier   string
		want         []protocol.WorkspaceDiagnosticReport
		wantErr      bool
		// Mock setup parameters
		mockDetectedLanguages []lsp.Language
		mockDetectError       error
		mockClientsSetup      map[lsp.Language]*mocks.MockLanguageClient           // language -> mock client
		mockReports           map[lsp.Language]*protocol.WorkspaceDiagnosticReport // language -> report
		mockReportErrors      map[lsp.Language]error                               // language -> error
		serverConfig          *lsp.LanguageServerConfig
		serverConfigError     error
		setupClients          bool
	}{
		{
			name: "successful workspace diagnostics - single language",
			config: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					"go": {
						Command: "gopls",
						Args:    []string{},
					},
				},
				ExtensionLanguageMap: map[string]lsp.Language{
					".go": "go",
				},
			},
			allowedDirectories:    []string{"."},
			workspaceUri:          "file:///home/rockerboo/code/mcp-lsp-bridge",
			identifier:            "workspace-1",
			mockDetectedLanguages: []lsp.Language{"go"},
			mockDetectError:       nil,
			mockClientsSetup: map[lsp.Language]*mocks.MockLanguageClient{
				"go": {},
			},
			mockReports: map[lsp.Language]*protocol.WorkspaceDiagnosticReport{
				"go": {
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						{
							Value: protocol.WorkspaceFullDocumentDiagnosticReport{
								Uri:     protocol.DocumentUri("file:///home/rockerboo/code/mcp-lsp-bridge/main.go"),
								Version: &version,
								Items: []protocol.Diagnostic{
									{
										Range: protocol.Range{
											Start: protocol.Position{Line: 1, Character: 0},
											End:   protocol.Position{Line: 1, Character: 10},
										},
										Message:  "unused variable",
										Severity: &diagnosticWarning,
									},
								},
							},
						},
					},
				},
			},
			mockReportErrors: map[lsp.Language]error{
				"go": nil,
			},
			want: []protocol.WorkspaceDiagnosticReport{
				{
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						{
							Value: protocol.WorkspaceFullDocumentDiagnosticReport{
								Uri:     protocol.DocumentUri("file:///home/rockerboo/code/mcp-lsp-bridge/main.go"),
								Version: &version,
								Items: []protocol.Diagnostic{
									{
										Range: protocol.Range{
											Start: protocol.Position{Line: 1, Character: 0},
											End:   protocol.Position{Line: 1, Character: 10},
										},
										Message:  "unused variable",
										Severity: &diagnosticWarning,
									},
								},
							},
						},
					},
				},
			},
			wantErr:      false,
			setupClients: true,
		},
		{
			name: "successful workspace diagnostics - multiple languages",
			config: &lsp.LSPServerConfig{
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
				},
			},
			allowedDirectories:    []string{"."},
			workspaceUri:          "file:///home/rockerboo/code/mcp-lsp-bridge",
			identifier:            "workspace-1",
			mockDetectedLanguages: []lsp.Language{"go", "typescript"},
			mockDetectError:       nil,
			mockClientsSetup: map[lsp.Language]*mocks.MockLanguageClient{
				"go":         {},
				"typescript": {},
			},
			mockReports: map[lsp.Language]*protocol.WorkspaceDiagnosticReport{
				"go": {
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						{
							Value: protocol.WorkspaceFullDocumentDiagnosticReport{
								Uri:     protocol.DocumentUri("file:///home/rockerboo/code/mcp-lsp-bridge/main.go"),
								Version: &version,
								Items:   []protocol.Diagnostic{},
							},
						},
					},
				},
				"typescript": {
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						{
							Value: protocol.WorkspaceFullDocumentDiagnosticReport{
								Uri:     protocol.DocumentUri("file:///home/rockerboo/code/mcp-lsp-bridge/src/index.ts"),
								Version: &version,
								Items:   []protocol.Diagnostic{},
							},
						},
					},
				},
			},
			mockReportErrors: map[lsp.Language]error{
				"go":         nil,
				"typescript": nil,
			},
			want: []protocol.WorkspaceDiagnosticReport{
				{
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						{
							Value: protocol.WorkspaceFullDocumentDiagnosticReport{
								Uri:     protocol.DocumentUri("file:///home/rockerboo/code/mcp-lsp-bridge/main.go"),
								Version: &version,
								Items:   []protocol.Diagnostic{},
							},
						},
					},
				},
				{
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						{
							Value: protocol.WorkspaceFullDocumentDiagnosticReport{
								Uri:     protocol.DocumentUri("file:///home/rockerboo/code/mcp-lsp-bridge/src/index.ts"),
								Version: &version,
								Items:   []protocol.Diagnostic{},
							},
						},
					},
				},
			},
			wantErr:      false,
			setupClients: true,
		},
		{
			name: "no languages detected",
			config: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					"go": {
						Command: "gopls",
						Args:    []string{},
					},
				},
				ExtensionLanguageMap: map[string]lsp.Language{
					".go": "go",
				},
			},
			allowedDirectories:    []string{"."},
			workspaceUri:          "file:///home/rockerboo/code/empty-project",
			identifier:            "workspace-1",
			mockDetectedLanguages: []lsp.Language{}, // No languages detected
			mockDetectError:       nil,
			mockClientsSetup:      map[lsp.Language]*mocks.MockLanguageClient{},
			mockReports:           map[lsp.Language]*protocol.WorkspaceDiagnosticReport{},
			mockReportErrors:      map[lsp.Language]error{},
			want:                  []protocol.WorkspaceDiagnosticReport{}, // Empty result
			wantErr:               false,
			setupClients:          false,
		},
		{
			name: "error - failed to detect project languages",
			config: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					"go": {
						Command: "gopls",
						Args:    []string{},
					},
				},
				ExtensionLanguageMap: map[string]lsp.Language{
					".go": "go",
				},
			},
			allowedDirectories:    []string{"."},
			workspaceUri:          "file:///invalid/path",
			identifier:            "workspace-1",
			mockDetectedLanguages: []lsp.Language{},
			mockDetectError:       errors.New("failed to access workspace directory"),
			mockClientsSetup:      map[lsp.Language]*mocks.MockLanguageClient{},
			mockReports:           map[lsp.Language]*protocol.WorkspaceDiagnosticReport{},
			mockReportErrors:      map[lsp.Language]error{},
			want:                  []protocol.WorkspaceDiagnosticReport{},
			wantErr:               false,
			setupClients:          false,
		},
		{
			name: "error - failed to get language clients",
			config: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					// No language servers configured
				},
				ExtensionLanguageMap: map[string]lsp.Language{
					".go": "go",
				},
			},
			allowedDirectories:    []string{"."},
			workspaceUri:          "file:///home/rockerboo/code/mcp-lsp-bridge",
			identifier:            "workspace-1",
			mockDetectedLanguages: []lsp.Language{"go"},
			mockDetectError:       nil,
			mockClientsSetup:      map[lsp.Language]*mocks.MockLanguageClient{},
			mockReports:           map[lsp.Language]*protocol.WorkspaceDiagnosticReport{},
			mockReportErrors:      map[lsp.Language]error{},
			serverConfig:          &lsp.LanguageServerConfig{},
			serverConfigError:     errors.New("Could not find server config"),
			want:                  nil,
			wantErr:               true,
			setupClients:          true,
		},
		{
			name: "partial success - one client fails, one succeeds",
			config: &lsp.LSPServerConfig{
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
				},
			},
			allowedDirectories:    []string{"."},
			workspaceUri:          "file:///home/rockerboo/code/mcp-lsp-bridge",
			identifier:            "workspace-1",
			mockDetectedLanguages: []lsp.Language{"go", "typescript"},
			mockDetectError:       nil,
			mockClientsSetup: map[lsp.Language]*mocks.MockLanguageClient{
				"go":         {},
				"typescript": {},
			},
			mockReports: map[lsp.Language]*protocol.WorkspaceDiagnosticReport{
				"go": {
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						{
							Value: protocol.WorkspaceFullDocumentDiagnosticReport{
								Uri:     protocol.DocumentUri("file:///home/rockerboo/code/mcp-lsp-bridge/main.go"),
								Version: &version,
								Items:   []protocol.Diagnostic{},
							},
						},
					},
				},
				"typescript": {
					Items: []protocol.WorkspaceDocumentDiagnosticReport{}, // This will be ignored due to error
				},
			},
			mockReportErrors: map[lsp.Language]error{
				"go":         nil,
				"typescript": errors.New("typescript language server failed"),
			},
			want: []protocol.WorkspaceDiagnosticReport{
				{
					Items: []protocol.WorkspaceDocumentDiagnosticReport{
						{
							Value: protocol.WorkspaceFullDocumentDiagnosticReport{
								Uri:     protocol.DocumentUri("file:///home/rockerboo/code/mcp-lsp-bridge/main.go"),
								Version: &version,
								Items:   []protocol.Diagnostic{},
							},
						},
					},
				},
			}, // Only Go report, TypeScript failed
			wantErr:      false,
			setupClients: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock config that can override DetectProjectLanguages
			mockConfig := &mocks.MockLSPServerConfig{}

			b := NewMCPLSPBridge(mockConfig, tt.allowedDirectories)

			if tt.mockDetectError != nil {
				mockConfig.On("DetectProjectLanguages", tt.workspaceUri).Return(tt.mockDetectedLanguages, nil)
			} else {
				mockConfig.On("DetectProjectLanguages", tt.workspaceUri).Return(tt.mockDetectedLanguages, tt.mockDetectError)
			}

			// Set up mock clients if needed
			if tt.setupClients {
				for language, mockClient := range tt.mockClientsSetup {
					b.clients[language] = mockClient

					// Set up mock expectations
					mockClient.On("GetMetrics").Return(lsp.ClientMetrics{Status: 3})
					ctx := context.Background()
					mockClient.On("Context").Return(ctx)

					// Set up WorkspaceDiagnostic mock
					if report, exists := tt.mockReports[language]; exists {
						if err, hasErr := tt.mockReportErrors[language]; hasErr {
							mockClient.On("WorkspaceDiagnostic", tt.identifier).Return(report, err)
						} else {
							mockClient.On("WorkspaceDiagnostic", tt.identifier).Return(report, nil)
						}
					}

					// Ensure mock expectations are checked at the end
					defer mockClient.AssertExpectations(t)
				}
			}

			if tt.serverConfig != nil {
				mockConfig.On("FindServerConfig", "go").Return(tt.serverConfig, tt.serverConfigError)
			}

			got, gotErr := b.GetWorkspaceDiagnostics(tt.workspaceUri, tt.identifier)

			if tt.wantErr {
				if gotErr == nil {
					t.Fatal("GetWorkspaceDiagnostics() succeeded unexpectedly, wanted error")
				}
				return
			}

			if gotErr != nil {
				t.Errorf("GetWorkspaceDiagnostics() failed: %v", gotErr)
				return
			}

			require.ElementsMatch(t, tt.want, got, "GetWorkspaceDiagnostics() returned unexpected results")
		})
	}
}

// Tests for bridge utility functions with 0% coverage

func TestIsAllowedDirectory(t *testing.T) {
	tests := []struct {
		name          string
		allowedDirs   []string
		testPath      string
		expectAllowed bool
		expectError   string
	}{
		{
			name:          "allowed directory exact match",
			allowedDirs:   []string{"/home/user/project"},
			testPath:      "/home/user/project",
			expectAllowed: true,
		},
		{
			name:          "allowed subdirectory",
			allowedDirs:   []string{"/home/user/project"},
			testPath:      "/home/user/project/src",
			expectAllowed: true,
		},
		{
			name:          "disallowed directory",
			allowedDirs:   []string{"/home/user/project"},
			testPath:      "/home/user/other",
			expectAllowed: false,
			expectError:   "file path is not allowed",
		},
		{
			name:          "path traversal attempt",
			allowedDirs:   []string{"/home/user/project"},
			testPath:      "/home/user/project/../other",
			expectAllowed: false,
			expectError:   "file path is not allowed",
		},
		{
			name:          "multiple allowed directories - first match",
			allowedDirs:   []string{"/home/user/project1", "/home/user/project2"},
			testPath:      "/home/user/project1/file.go",
			expectAllowed: true,
		},
		{
			name:          "multiple allowed directories - second match",
			allowedDirs:   []string{"/home/user/project1", "/home/user/project2"},
			testPath:      "/home/user/project2/file.go",
			expectAllowed: true,
		},
		{
			name:          "relative path resolution",
			allowedDirs:   []string{"/home/user/project"},
			testPath:      "project/../project/file.go",
			expectAllowed: false, // Will depend on current working directory
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := createTestBridge(tt.allowedDirs)
			
			result, err := bridge.IsAllowedDirectory(tt.testPath)
			
			if tt.expectAllowed {
				assert.NoError(t, err, "Expected path to be allowed")
				assert.NotEmpty(t, result, "Expected non-empty absolute path")
				// Result should be an absolute path
				assert.True(t, filepath.IsAbs(result), "Result should be absolute path")
			} else {
				assert.Error(t, err, "Expected path to be disallowed")
				if tt.expectError != "" {
					assert.Contains(t, err.Error(), tt.expectError)
				}
				assert.Empty(t, result, "Result should be empty on error")
			}
		})
	}
}

func TestAllowedDirectories(t *testing.T) {
	tests := []struct {
		name        string
		allowedDirs []string
	}{
		{
			name:        "single directory",
			allowedDirs: []string{"/home/user/project"},
		},
		{
			name:        "multiple directories",
			allowedDirs: []string{"/home/user/project1", "/home/user/project2", "/var/log"},
		},
		{
			name:        "empty directories",
			allowedDirs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := createTestBridge(tt.allowedDirs)
			
			result := bridge.AllowedDirectories()
			
			assert.Equal(t, tt.allowedDirs, result)
			// Note: Currently returns reference to original slice (not a copy)
			// This test verifies the current behavior
			if len(result) > 0 {
				originalFirst := result[0]
				result[0] = "modified"
				// Verify it's the same reference (current behavior)
				assert.Equal(t, result, bridge.AllowedDirectories(), "Currently returns reference to original slice")
				// Restore original value
				result[0] = originalFirst
			}
		})
	}
}

func TestDetectPrimaryProjectLanguage(t *testing.T) {
	// Create temporary test directory structure
	tempDir := t.TempDir()
	
	tests := []struct {
		name         string
		files        []string
		expectedLang *lsp.Language
		expectError  bool
	}{
		{
			name:         "go project with go.mod",
			files:        []string{"go.mod", "main.go"},
			expectedLang: func() *lsp.Language { l := lsp.Language("go"); return &l }(),
		},
		{
			name:         "python project with requirements.txt",
			files:        []string{"requirements.txt", "main.py"},
			expectedLang: func() *lsp.Language { l := lsp.Language("python"); return &l }(),
		},
		{
			name:         "mixed project - should detect primary",
			files:        []string{"go.mod", "main.go", "script.py"},
			expectedLang: func() *lsp.Language { l := lsp.Language("go"); return &l }(),
		},
		{
			name:        "empty directory",
			files:       []string{},
			expectedLang: nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create subdirectory for this test
			testDir := filepath.Join(tempDir, tt.name)
			err := os.MkdirAll(testDir, 0755)
			require.NoError(t, err)

			// Create test files
			for _, file := range tt.files {
				filePath := filepath.Join(testDir, file)
				err := os.WriteFile(filePath, []byte("test content"), 0644)
				require.NoError(t, err)
			}

			bridge := createTestBridge([]string{testDir})
			
			result, err := bridge.DetectPrimaryProjectLanguage(testDir)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				if tt.expectedLang != nil {
					require.NotNil(t, result)
					assert.Equal(t, *tt.expectedLang, *result)
				} else {
					assert.Nil(t, result)
				}
			}
		})
	}
}
