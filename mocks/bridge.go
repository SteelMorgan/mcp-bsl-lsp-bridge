package mocks

import (
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/stretchr/testify/mock"
)

type MockBridge struct {
	mock.Mock
}

func (m *MockBridge) GetClientForLanguage(language string) (lsp.LanguageClientInterface, error) {
	args := m.Called(language)

	// Safe type assertion with error checking
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	// Try to assert to the interface type
	if client, ok := args.Get(0).(lsp.LanguageClientInterface); ok {
		return client, args.Error(1)
	}

	return nil, args.Error(1)
}

func (m *MockBridge) InferLanguage(filePath string) (*lsp.Language, error) {
	args := m.Called(filePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*lsp.Language), args.Error(1)
}

func (m *MockBridge) ProjectRoots() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockBridge) SetProjectRoots(paths []string) {
	m.Called(paths)
}

func (m *MockBridge) IsAllowedDirectory(path string) (string, error) {
	args := m.Called(path)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockBridge) CloseAllClients() {
	m.Called()
}

func (m *MockBridge) GetConfig() lsp.LSPServerConfigProvider {
	args := m.Called()
	return args.Get(0).(lsp.LSPServerConfigProvider)
}

func (m *MockBridge) GetServerConfig(language string) (lsp.LanguageServerConfigProvider, error) {
	args := m.Called(language)
	return args.Get(0).(lsp.LanguageServerConfigProvider), args.Error(1)
}

func (m *MockBridge) DetectProjectLanguages(projectPath string) ([]lsp.Language, error) {
	args := m.Called(projectPath)
	return args.Get(0).([]lsp.Language), args.Error(1)
}

func (m *MockBridge) DetectPrimaryProjectLanguage(projectPath string) (*lsp.Language, error) {
	args := m.Called(projectPath)
	return args.Get(0).(*lsp.Language), args.Error(1)
}

func (m *MockBridge) FindSymbolReferences(language, uri string, line, character uint32, includeDeclaration bool) ([]protocol.Location, error) {
	args := m.Called(language, uri, line, character, includeDeclaration)
	return args.Get(0).([]protocol.Location), args.Error(1)
}

func (m *MockBridge) FindSymbolDefinitions(language, uri string, line, character uint32) ([]protocol.Or2[protocol.LocationLink, protocol.Location], error) {
	args := m.Called(language, uri, line, character)
	return args.Get(0).([]protocol.Or2[protocol.LocationLink, protocol.Location]), args.Error(1)
}

func (m *MockBridge) SearchTextInWorkspace(language, query string) ([]protocol.WorkspaceSymbol, error) {
	args := m.Called(language, query)
	return args.Get(0).([]protocol.WorkspaceSymbol), args.Error(1)
}

func (m *MockBridge) GetMultiLanguageClients(languages []string) (map[lsp.Language]lsp.LanguageClientInterface, error) {
	args := m.Called(languages)
	return args.Get(0).(map[lsp.Language]lsp.LanguageClientInterface), args.Error(1)
}

func (m *MockBridge) GetHoverInformation(uri string, line, character uint32) (*protocol.Hover, error) {
	args := m.Called(uri, line, character)
	return args.Get(0).(*protocol.Hover), args.Error(1)
}

func (m *MockBridge) GetDiagnostics(uri string) ([]any, error) {
	args := m.Called(uri)
	return args.Get(0).([]any), args.Error(1)
}

func (m *MockBridge) GetWorkspaceDiagnostics(workspaceUri string, identifier string) ([]protocol.WorkspaceDiagnosticReport, error) {
	args := m.Called(workspaceUri, identifier)
	return args.Get(0).([]protocol.WorkspaceDiagnosticReport), args.Error(1)
}

func (m *MockBridge) GetSignatureHelp(uri string, line, character uint32) (*protocol.SignatureHelp, error) {
	args := m.Called(uri, line, character)
	return args.Get(0).(*protocol.SignatureHelp), args.Error(1)
}

func (m *MockBridge) GetCodeActions(uri string, line, character, endLine, endCharacter uint32) ([]protocol.CodeAction, error) {
	args := m.Called(uri, line, character, endLine, endCharacter)
	return args.Get(0).([]protocol.CodeAction), args.Error(1)
}

func (m *MockBridge) FormatDocument(uri string, tabSize uint32, insertSpaces bool) ([]protocol.TextEdit, error) {
	args := m.Called(uri, tabSize, insertSpaces)
	return args.Get(0).([]protocol.TextEdit), args.Error(1)
}

func (m *MockBridge) ApplyTextEdits(uri string, edits []protocol.TextEdit) error {
	args := m.Called(uri, edits)
	return args.Error(0)
}

func (m *MockBridge) RenameSymbol(uri string, line, character uint32, newName string, preview bool) (*protocol.WorkspaceEdit, error) {
	args := m.Called(uri, line, character, newName, preview)
	return args.Get(0).(*protocol.WorkspaceEdit), args.Error(1)
}

func (m *MockBridge) ApplyWorkspaceEdit(edit *protocol.WorkspaceEdit) error {
	args := m.Called(edit)
	return args.Error(0)
}

func (m *MockBridge) FindImplementations(uri string, line, character uint32) ([]protocol.Location, error) {
	args := m.Called(uri, line, character)
	return args.Get(0).([]protocol.Location), args.Error(1)
}

func (m *MockBridge) SemanticTokens(uri string, targetTypes []string, startLine, startCharacter, endLine, endCharacter uint32) ([]lsp.TokenPosition, error) {
	args := m.Called(uri, startLine, startCharacter, endLine, endCharacter)
	return args.Get(0).([]lsp.TokenPosition), args.Error(1)
}

func (m *MockBridge) PrepareCallHierarchy(uri string, line, character uint32) ([]protocol.CallHierarchyItem, error) {
	args := m.Called(uri, line, character)
	return args.Get(0).([]protocol.CallHierarchyItem), args.Error(1)
}

func (m *MockBridge) GetIncomingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyIncomingCall, error) {
	args := m.Called(item)
	return args.Get(0).([]protocol.CallHierarchyIncomingCall), args.Error(1)
}

func (m *MockBridge) GetOutgoingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyOutgoingCall, error) {
	args := m.Called(item)
	return args.Get(0).([]protocol.CallHierarchyOutgoingCall), args.Error(1)
}

func (m *MockBridge) GetDocumentSymbols(uri string) ([]protocol.DocumentSymbol, error) {
	args := m.Called(uri)
	return args.Get(0).([]protocol.DocumentSymbol), args.Error(1)
}
