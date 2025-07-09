package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Initialization Method Tests
func TestInitialize(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	ctx := t.Context()

	// Successful initialization
	mockConn.On("Call", mock.Anything, "initialize", mock.AnythingOfType("protocol.InitializeParams"), mock.AnythingOfType("*protocol.InitializeResult"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	// mock DisconnectNotify to return a channel that closes when disconnectCtx is canceled
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn:      mockConn,
		ctx:       ctx,
		processID: 1,
	}

	pid := int32(1)
	rootPath := "/test"
	params := protocol.InitializeParams{
		ProcessId: &pid,
		RootPath:  &rootPath,
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{},
			Workspace:    &protocol.WorkspaceClientCapabilities{},
		},
	}

	result, err := client.Initialize(params)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Capabilities)

	mockConn.AssertExpectations(t)
}

func TestInitialized(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful notification
	mockConn.On("Notify", mock.AnythingOfType("context.backgroundCtx"), "initialized", mock.AnythingOfType("protocol.InitializedParams"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	err := client.Initialized()
	require.NoError(t, err)

	mockConn.AssertExpectations(t)
}

func TestShutdown(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful shutdown
	mockConn.On("Call", mock.Anything, "shutdown", mock.Anything, mock.AnythingOfType("*protocol.ShutdownResponse"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	err := client.Shutdown()
	require.NoError(t, err)

	mockConn.AssertExpectations(t)
}

func TestExit(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful exit
	mockConn.On("Notify", mock.AnythingOfType("context.backgroundCtx"), "exit", mock.Anything, mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	err := client.Exit()
	require.NoError(t, err)

	mockConn.AssertExpectations(t)
}

func TestDidOpen(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful document open
	mockConn.On("Notify", mock.AnythingOfType("context.backgroundCtx"), "textDocument/didOpen", mock.AnythingOfType("protocol.DidOpenTextDocumentParams"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	err := client.DidOpen("file:///test.go", "go", "test content", 1)
	require.NoError(t, err)

	mockConn.AssertExpectations(t)
}

func TestDidChange(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful document change
	mockConn.On("Notify", mock.AnythingOfType("context.backgroundCtx"), "textDocument/didChange", mock.AnythingOfType("protocol.DidChangeTextDocumentParams"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	changes := []protocol.TextDocumentContentChangeEvent{
		{
			Value: protocol.TextDocumentContentChangePartial{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End:   protocol.Position{Line: 0, Character: 0},
				},
				Text: "updated content",
			},
		},
	}
	err := client.DidChange("file:///test.go", 2, changes)
	require.NoError(t, err)

	mockConn.AssertExpectations(t)
}

func TestDidSave(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful document save
	mockConn.On("Notify", mock.AnythingOfType("context.backgroundCtx"), "textDocument/didSave", mock.Anything, mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	// With text
	text := "saved content"
	err := client.DidSave("file:///test.go", &text)
	require.NoError(t, err)

	// Without text
	err = client.DidSave("file:///test.go", nil)
	require.NoError(t, err)

	mockConn.AssertExpectations(t)
}

func TestDidClose(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful document close
	mockConn.On("Notify", mock.AnythingOfType("context.backgroundCtx"), "textDocument/didClose", mock.AnythingOfType("protocol.DidCloseTextDocumentParams"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	err := client.DidClose("file:///test.go")
	require.NoError(t, err)

	mockConn.AssertExpectations(t)
}

func TestDefinition(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful definition request
	mockConn.On("Call", mock.Anything, "textDocument/definition", mock.AnythingOfType("protocol.DefinitionParams"), mock.Anything, mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	definitions, err := client.Definition("file:///test.go", 10, 5)
	require.NoError(t, err)
	assert.NotNil(t, definitions)

	mockConn.AssertExpectations(t)
}

func TestHover(t *testing.T) {
	testCases := []struct {
		name           string
		mockResponse   []byte
		expectedResult bool
		expectError    bool
	}{
		{
			name:           "Successful hover with markdown content",
			mockResponse:   []byte(`{"contents": {"kind": "markdown", "value": "Sample hover content"}}`),
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "Successful hover with string content",
			mockResponse:   []byte(`{"contents": "Simple hover text"}`),
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "Hover with null response",
			mockResponse:   []byte(`null`),
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "Hover with empty contents",
			mockResponse:   []byte(`{"contents": {}}`),
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockConn := new(mocks.MockLSPConnectionInterface)

			ctx := t.Context()
			mockConn.On("Call", mock.Anything, "textDocument/hover", 
				mock.AnythingOfType("protocol.HoverParams"), 
				mock.AnythingOfType("*json.RawMessage"), 
				mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil).Run(func(args mock.Arguments) {
				rawMessage := args.Get(3).(*json.RawMessage)
				*rawMessage = tc.mockResponse
			})
			mockConn.On("DisconnectNotify").Return(ctx.Done())

			client := &LanguageClient{
				conn: mockConn,
				ctx:  t.Context(),
			}

			hoverInfo, err := client.Hover("file:///test.go", 10, 5)
			
			if tc.expectError {
				require.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, err)
				if tc.expectedResult {
					assert.NotNil(t, hoverInfo, "Expected hover info to be non-nil")
				} else {
					assert.Nil(t, hoverInfo, "Expected hover info to be nil")
				}
			}

			mockConn.AssertExpectations(t)
		})
	}
}

func TestWorkspaceSymbols(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful workspace symbols request
	mockConn.On("Call", mock.Anything, "workspace/symbol", mock.AnythingOfType("protocol.WorkspaceSymbolParams"), mock.AnythingOfType("*[]protocol.WorkspaceSymbol"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	symbols, err := client.WorkspaceSymbols("test")
	require.NoError(t, err)
	assert.NotNil(t, symbols)

	mockConn.AssertExpectations(t)
}

func TestReferences(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful references request
	mockConn.On("Call", mock.Anything, "textDocument/references", mock.AnythingOfType("protocol.ReferenceParams"), mock.AnythingOfType("*[]protocol.Location"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	references, err := client.References("file:///test.go", 10, 5, true)
	require.NoError(t, err)
	assert.NotNil(t, references)

	mockConn.AssertExpectations(t)
}

func TestDocumentSymbols(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful document symbols request (newer format)
	mockConn.On("Call", mock.Anything, "textDocument/documentSymbol", mock.AnythingOfType("protocol.DocumentSymbolParams"), mock.AnythingOfType("*[]protocol.DocumentSymbol"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	symbols, err := client.DocumentSymbols("file:///test.go")
	require.NoError(t, err)
	assert.NotNil(t, symbols)
	assert.Len(t, symbols, 1)
	assert.Equal(t, "TestSymbol", symbols[0].Name)

	mockConn.AssertExpectations(t)
}

func TestImplementation(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful implementation request
	mockConn.On("Call", mock.Anything, "textDocument/implementation", mock.AnythingOfType("protocol.ImplementationParams"), mock.AnythingOfType("*[]protocol.Location"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	implementations, err := client.Implementation("file:///test.go", 10, 5)
	require.NoError(t, err)
	assert.NotNil(t, implementations)

	mockConn.AssertExpectations(t)
}

func TestSignatureHelp(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful signature help request
	mockConn.On("Call", mock.Anything, "textDocument/signatureHelp", mock.AnythingOfType("protocol.SignatureHelpParams"), mock.AnythingOfType("*json.RawMessage"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	signatureHelp, err := client.SignatureHelp("file:///test.go", 10, 5)
	require.NoError(t, err)
	assert.NotNil(t, signatureHelp)

	mockConn.AssertExpectations(t)
}

func TestCodeActions(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful code actions request
	mockConn.On("Call", mock.Anything, "textDocument/codeAction", mock.AnythingOfType("protocol.CodeActionParams"), mock.AnythingOfType("*[]protocol.CodeAction"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	codeActions, err := client.CodeActions("file:///test.go", 10, 5, 10, 15)
	require.NoError(t, err)
	assert.NotNil(t, codeActions)

	mockConn.AssertExpectations(t)
}

func TestRename(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful rename request
	mockConn.On("Call", mock.Anything, "textDocument/rename", mock.AnythingOfType("protocol.RenameParams"), mock.AnythingOfType("*protocol.WorkspaceEdit"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	workspaceEdit, err := client.Rename("file:///test.go", 10, 5, "newName")
	require.NoError(t, err)
	assert.NotNil(t, workspaceEdit)

	mockConn.AssertExpectations(t)
}

func TestWorkspaceDiagnostic(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful workspace diagnostic request
	mockConn.On("Call", mock.Anything, "workspace/diagnostic", mock.AnythingOfType("protocol.WorkspaceDiagnosticParams"), mock.AnythingOfType("*protocol.WorkspaceDiagnosticReport"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	diagnosticReport, err := client.WorkspaceDiagnostic("test-identifier")
	require.NoError(t, err)
	assert.NotNil(t, diagnosticReport)

	mockConn.AssertExpectations(t)
}

func TestFormatting(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful formatting request
	mockConn.On("Call", mock.Anything, "textDocument/formatting", mock.AnythingOfType("protocol.DocumentFormattingParams"), mock.AnythingOfType("*[]protocol.TextEdit"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	textEdits, err := client.Formatting("file:///test.go", 4, true)
	require.NoError(t, err)
	assert.NotNil(t, textEdits)

	mockConn.AssertExpectations(t)
}

func TestPrepareCallHierarchy(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful prepare call hierarchy request
	mockConn.On("Call", mock.Anything, "textDocument/prepareCallHierarchy", mock.AnythingOfType("protocol.CallHierarchyPrepareParams"), mock.AnythingOfType("*[]protocol.CallHierarchyItem"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	callHierarchyItems, err := client.PrepareCallHierarchy("file:///test.go", 10, 5)
	require.NoError(t, err)
	assert.NotNil(t, callHierarchyItems)

	mockConn.AssertExpectations(t)
}

func TestSemanticTokens(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful semantic tokens request
	mockConn.On("Call", mock.Anything, "textDocument/semanticTokens", mock.AnythingOfType("protocol.SemanticTokensParams"), mock.AnythingOfType("*protocol.SemanticTokens"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	semanticTokens, err := client.SemanticTokens("file:///test.go")
	require.NoError(t, err)
	assert.NotNil(t, semanticTokens)

	mockConn.AssertExpectations(t)
}

func TestSemanticTokensRange(t *testing.T) {
	mockConn := new(mocks.MockLSPConnectionInterface)

	// Prepare a context
	ctx := t.Context()

	// Successful semantic tokens range request
	mockConn.On("Call", mock.Anything, "textDocument/semanticTokens/range", mock.AnythingOfType("protocol.SemanticTokensRangeParams"), mock.AnythingOfType("*protocol.SemanticTokens"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)
	mockConn.On("DisconnectNotify").Return(ctx.Done())

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	semanticTokens, err := client.SemanticTokensRange("file:///test.go", 0, 0, 10, 100)
	require.NoError(t, err)
	assert.NotNil(t, semanticTokens)

	mockConn.AssertExpectations(t)
}
