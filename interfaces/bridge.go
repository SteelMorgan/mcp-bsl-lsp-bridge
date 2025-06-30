package interfaces

import (
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

type BridgeInterface interface {
	ConfigManager
	InformationProvider
	// ProjectRootManager
	ClientManager
	LanguageDetector
	DirectoryManager
	SymbolNavigator
	DiagnosticsProvider
	EditProvider
	CodeInspector
	CallHierarchyProvider
}

type InformationProvider interface {
	SemanticTokens(uri string, targetTypes []string, startLine, startCharacter, endLine, endCharacter uint32) ([]lsp.TokenPosition, error)
	GetCodeActions(uri string, line, character, endLine, endCharacter uint32) ([]protocol.CodeAction, error)
}
type CallHierarchyProvider interface {
	PrepareCallHierarchy(uri string, line, character uint32) ([]protocol.CallHierarchyItem, error)
	GetIncomingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyIncomingCall, error)
	GetOutgoingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyOutgoingCall, error)
}

type CodeInspector interface {
	GetHoverInformation(uri string, line, character uint32) (*protocol.Hover, error)
	GetSignatureHelp(uri string, line, character uint32) (*protocol.SignatureHelp, error)
}

type EditProvider interface {
	FormatDocument(uri string, tabSize uint32, insertSpaces bool) ([]protocol.TextEdit, error)
	ApplyTextEdits(uri string, edits []protocol.TextEdit) error
	RenameSymbol(uri string, line, character uint32, newName string, preview bool) (*protocol.WorkspaceEdit, error)
	ApplyWorkspaceEdit(edit *protocol.WorkspaceEdit) error
}

type DiagnosticsProvider interface {
	GetWorkspaceDiagnostics(workspaceUri string, identifier string) ([]protocol.WorkspaceDiagnosticReport, error)
}

type SymbolNavigator interface {
	SearchTextInWorkspace(language, query string) ([]protocol.WorkspaceSymbol, error)
	GetDocumentSymbols(uri string) ([]protocol.DocumentSymbol, error)

	FindSymbolReferences(language, uri string, line, character uint32, includeDeclaration bool) ([]protocol.Location, error)
	FindSymbolDefinitions(language, uri string, line, character uint32) ([]protocol.Or2[protocol.LocationLink, protocol.Location], error)
	FindImplementations(uri string, line, character uint32) ([]protocol.Location, error)
}

type DirectoryManager interface {
	IsAllowedDirectory(path string) (string, error)
}

type ClientManager interface {
	GetClientForLanguage(language string) (lsp.LanguageClientInterface, error)
	GetMultiLanguageClients(languages []string) (map[lsp.Language]lsp.LanguageClientInterface, error)
	CloseAllClients()
}

type ConfigManager interface {
	GetConfig() lsp.LSPServerConfigProvider
	GetServerConfig(language string) (lsp.LanguageServerConfigProvider, error)
}

type LanguageDetector interface {
	InferLanguage(filePath string) (*lsp.Language, error)
	DetectProjectLanguages(projectPath string) ([]lsp.Language, error)
	DetectPrimaryProjectLanguage(projectPath string) (*lsp.Language, error)
}

// type ProjectRootManager interface {
// 	ProjectRoots() ([]string, error)
// 	SetProjectRoots(paths []string)
// }
