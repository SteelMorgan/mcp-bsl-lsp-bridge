package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockLSPConnectionInterface simulates the LSP connection
type MockLSPConnectionInterface struct {
	mock.Mock
	ctx context.Context
}

func (m *MockLSPConnectionInterface) Call(ctx context.Context, method string, params, result interface{}, opts ...jsonrpc2.CallOption) error {
	m.ctx = ctx
	args := m.Called(ctx, method, params, result, opts)

	// Simulate populating results for different methods
	if args.Error(0) == nil {
		switch method {
		case "initialize":
			if r, ok := result.(*protocol.InitializeResult); ok {
				*r = protocol.InitializeResult{
					Capabilities: protocol.ServerCapabilities{},
				}
			}
		case "textDocument/definition":
			if r, ok := result.(*json.RawMessage); ok {
				// Return empty array as JSON
				*r = json.RawMessage("[]")
			}
		case "textDocument/hover":
			if r, ok := result.(*protocol.Hover); ok {
				*r = protocol.Hover{
					Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{
						Value: protocol.MarkupContent{
							Kind:  "markdown",
							Value: "Test hover content",
						},
					},
				}
			}
		case "workspace/symbol":
			if r, ok := result.(*[]protocol.WorkspaceSymbol); ok {
				*r = []protocol.WorkspaceSymbol{}
			}
		case "textDocument/references":
			if r, ok := result.(*[]protocol.Location); ok {
				*r = []protocol.Location{}
			}
		case "textDocument/documentSymbol":
			if r, ok := result.(*[]protocol.DocumentSymbol); ok {
				*r = []protocol.DocumentSymbol{
					{
						Name: "TestSymbol",
						Kind: 12, // Function kind
						Range: protocol.Range{
							Start: protocol.Position{Line: 0, Character: 0},
							End:   protocol.Position{Line: 1, Character: 0},
						},
						SelectionRange: protocol.Range{
							Start: protocol.Position{Line: 0, Character: 0},
							End:   protocol.Position{Line: 0, Character: 10},
						},
					},
				}
			}
		case "textDocument/implementation":
			if r, ok := result.(*[]protocol.Location); ok {
				*r = []protocol.Location{}
			}
		case "textDocument/signatureHelp":
			if r, ok := result.(*json.RawMessage); ok {
				*r = json.RawMessage(`{"signatures": []}`)
			}
		case "textDocument/codeAction":
			if r, ok := result.(*[]protocol.CodeAction); ok {
				*r = []protocol.CodeAction{}
			}
		case "textDocument/rename":
			if r, ok := result.(*protocol.WorkspaceEdit); ok {
				*r = protocol.WorkspaceEdit{}
			}
		case "workspace/diagnostic":
			if r, ok := result.(*protocol.WorkspaceDiagnosticReport); ok {
				*r = protocol.WorkspaceDiagnosticReport{}
			}
		case "textDocument/formatting":
			if r, ok := result.(*[]protocol.TextEdit); ok {
				*r = []protocol.TextEdit{}
			}
		case "textDocument/prepareCallHierarchy":
			if r, ok := result.(*[]protocol.CallHierarchyItem); ok {
				*r = []protocol.CallHierarchyItem{}
			}
		case "textDocument/semanticTokens":
			if r, ok := result.(*protocol.SemanticTokens); ok {
				*r = protocol.SemanticTokens{}
			}
		case "textDocument/semanticTokens/range":
			if r, ok := result.(*protocol.SemanticTokens); ok {
				*r = protocol.SemanticTokens{}
			}
		}
	}

	return args.Error(0)
}

func (m *MockLSPConnectionInterface) Notify(ctx context.Context, method string, params any, opts ...jsonrpc2.CallOption) error {
	m.ctx = ctx
	args := m.Called(ctx, method, params, opts)
	return args.Error(0)
}

func (m *MockLSPConnectionInterface) Reply(ctx context.Context, id jsonrpc2.ID, result any) error {
	m.ctx = ctx
	args := m.Called(ctx, id, result)
	return args.Error(0)
}

func (m *MockLSPConnectionInterface) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Initialization Method Tests
func TestInitialize(t *testing.T) {
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful initialization
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "initialize", mock.AnythingOfType("protocol.InitializeParams"), mock.AnythingOfType("*protocol.InitializeResult"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn:      mockConn,
		ctx:       ctx,
		processID: 1,
	}

	pid := int32(1)
	rootPath := "/test"
	rootPathPtr := &rootPath
	params := protocol.InitializeParams{
		ProcessId: &pid,
		RootPath:  &rootPathPtr,
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
	mockConn := new(MockLSPConnectionInterface)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful shutdown
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "shutdown", mock.Anything, mock.AnythingOfType("*protocol.ShutdownResponse"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	err := client.Shutdown()
	require.NoError(t, err)

	mockConn.AssertExpectations(t)
}

func TestExit(t *testing.T) {
	mockConn := new(MockLSPConnectionInterface)

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
	mockConn := new(MockLSPConnectionInterface)

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
	mockConn := new(MockLSPConnectionInterface)

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
	mockConn := new(MockLSPConnectionInterface)

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
	mockConn := new(MockLSPConnectionInterface)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful definition request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/definition", mock.AnythingOfType("protocol.DefinitionParams"), mock.Anything, mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful hover request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/hover", mock.AnythingOfType("protocol.HoverParams"), mock.AnythingOfType("*protocol.Hover"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	hoverInfo, err := client.Hover("file:///test.go", 10, 5)
	require.NoError(t, err)
	assert.NotNil(t, hoverInfo)

	mockConn.AssertExpectations(t)
}

func TestWorkspaceSymbols(t *testing.T) {
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful workspace symbols request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "workspace/symbol", mock.AnythingOfType("protocol.WorkspaceSymbolParams"), mock.AnythingOfType("*[]protocol.WorkspaceSymbol"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful references request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/references", mock.AnythingOfType("protocol.ReferenceParams"), mock.AnythingOfType("*[]protocol.Location"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful document symbols request (newer format)
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/documentSymbol", mock.AnythingOfType("protocol.DocumentSymbolParams"), mock.AnythingOfType("*[]protocol.DocumentSymbol"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful implementation request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/implementation", mock.AnythingOfType("protocol.ImplementationParams"), mock.AnythingOfType("*[]protocol.Location"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful signature help request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/signatureHelp", mock.AnythingOfType("protocol.SignatureHelpParams"), mock.AnythingOfType("*json.RawMessage"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful code actions request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/codeAction", mock.AnythingOfType("protocol.CodeActionParams"), mock.AnythingOfType("*[]protocol.CodeAction"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful rename request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/rename", mock.AnythingOfType("protocol.RenameParams"), mock.AnythingOfType("*protocol.WorkspaceEdit"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful workspace diagnostic request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "workspace/diagnostic", mock.AnythingOfType("protocol.WorkspaceDiagnosticParams"), mock.AnythingOfType("*protocol.WorkspaceDiagnosticReport"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful formatting request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/formatting", mock.AnythingOfType("protocol.DocumentFormattingParams"), mock.AnythingOfType("*[]protocol.TextEdit"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful prepare call hierarchy request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/prepareCallHierarchy", mock.AnythingOfType("protocol.CallHierarchyPrepareParams"), mock.AnythingOfType("*[]protocol.CallHierarchyItem"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful semantic tokens request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/semanticTokens", mock.AnythingOfType("protocol.SemanticTokensParams"), mock.AnythingOfType("*protocol.SemanticTokens"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

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
	mockConn := new(MockLSPConnectionInterface)

	// Prepare a context
	ctx := context.Background()

	// Successful semantic tokens range request
	mockConn.On("Call", mock.AnythingOfType("*context.timerCtx"), "textDocument/semanticTokens/range", mock.AnythingOfType("protocol.SemanticTokensRangeParams"), mock.AnythingOfType("*protocol.SemanticTokens"), mock.AnythingOfType("[]jsonrpc2.CallOption")).Return(nil)

	client := &LanguageClient{
		conn: mockConn,
		ctx:  ctx,
	}

	semanticTokens, err := client.SemanticTokensRange("file:///test.go", 0, 0, 10, 100)
	require.NoError(t, err)
	assert.NotNil(t, semanticTokens)

	mockConn.AssertExpectations(t)
}
