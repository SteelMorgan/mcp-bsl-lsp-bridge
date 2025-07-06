package mocks

import (
	"context"
	"encoding/json"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/mock"
)

type MockLSPConnectionInterface struct {
	mock.Mock
	ctx context.Context
}

func (m *MockLSPConnectionInterface) DisconnectNotify() <-chan struct{} {
	args := m.Called()
	return args.Get(0).(<-chan struct{})
}

func (m *MockLSPConnectionInterface) Call(ctx context.Context, method string, params, result any, opts ...jsonrpc2.CallOption) error {
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
