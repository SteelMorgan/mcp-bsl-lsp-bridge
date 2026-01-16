package interfaces

import (
	"encoding/json"

	"rockerboo/mcp-lsp-bridge/types"
	"rockerboo/mcp-lsp-bridge/utils"

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
	DocumentFeaturesProvider
	WorkspaceCommandProvider
	CodeInspector
	CallHierarchyProvider
	PathMapperProvider
}

type InformationProvider interface {
	SemanticTokens(uri string, targetTypes []string, startLine, startCharacter, endLine, endCharacter uint32) ([]types.TokenPosition, error)
	GetCodeActions(uri string, line, character, endLine, endCharacter uint32) ([]protocol.CodeAction, error)
}
type CallHierarchyProvider interface {
	PrepareCallHierarchy(uri string, line, character uint32) ([]protocol.CallHierarchyItem, error)
	IncomingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyIncomingCall, error)
	OutgoingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyOutgoingCall, error)
}

type CodeInspector interface {
	GetHoverInformation(uri string, line, character uint32) (*protocol.Hover, error)
	GetSignatureHelp(uri string, line, character uint32) (*protocol.SignatureHelp, error)
}

type EditProvider interface {
	FormatDocument(uri string, tabSize uint32, insertSpaces bool) ([]protocol.TextEdit, error)
	RangeFormatting(uri string, startLine, startCharacter, endLine, endCharacter uint32, tabSize uint32, insertSpaces bool) ([]protocol.TextEdit, error)
	ApplyTextEdits(uri string, edits []protocol.TextEdit) error
	RenameSymbol(uri string, line, character uint32, newName string, preview bool) (*protocol.WorkspaceEdit, error)
	PrepareRename(uri string, line, character uint32) (*protocol.PrepareRenameResult, error)
	ApplyWorkspaceEdit(edit *protocol.WorkspaceEdit) error
}

type DocumentFeaturesProvider interface {
	FoldingRange(uri string) ([]protocol.FoldingRange, error)
	SelectionRange(uri string, positions []protocol.Position) ([]protocol.SelectionRange, error)
	DocumentLink(uri string) ([]protocol.DocumentLink, error)
	DocumentColor(uri string) ([]protocol.ColorInformation, error)
	ColorPresentation(uri string, color protocol.Color, rng protocol.Range) ([]protocol.ColorPresentation, error)
}

type WorkspaceCommandProvider interface {
	ExecuteCommand(language string, command string, arguments []any) (json.RawMessage, error)
	DidChangeWatchedFiles(language string, changes []protocol.FileEvent) error
	DidChangeConfiguration(language string, settings any) error
}

type DiagnosticsProvider interface {
	GetWorkspaceDiagnostics(workspaceUri string, identifier string) ([]protocol.WorkspaceDiagnosticReport, error)
}

type SymbolNavigator interface {
	SearchTextInWorkspace(language, query string) ([]protocol.WorkspaceSymbol, error)
	SearchTextInAllLanguages(query string) ([]protocol.WorkspaceSymbol, error)
	GetDocumentSymbols(uri string) ([]protocol.DocumentSymbol, error)

	FindSymbolReferences(language, uri string, line, character uint32, includeDeclaration bool) ([]protocol.Location, error)
	FindSymbolDefinitions(language, uri string, line, character uint32) ([]protocol.Or2[protocol.LocationLink, protocol.Location], error)
	FindImplementations(uri string, line, character uint32) ([]protocol.Location, error)
}

type DirectoryManager interface {
	IsAllowedDirectory(path string) (string, error)
	AllowedDirectories() []string
}

type ClientManager interface {
	GetClientForLanguage(language string) (types.LanguageClientInterface, error)
	GetMultiLanguageClients(languages []string) (map[types.Language]types.LanguageClientInterface, error)
	CloseAllClients()
}

type ConfigManager interface {
	GetConfig() types.LSPServerConfigProvider
	GetServerConfig(language string) (types.LanguageServerConfigProvider, error)
}

type LanguageDetector interface {
	InferLanguage(filePath string) (*types.Language, error)
	DetectProjectLanguages(projectPath string) ([]types.Language, error)
	DetectPrimaryProjectLanguage(projectPath string) (*types.Language, error)
}

// type ProjectRootManager interface {
// 	ProjectRoots() ([]string, error)
// 	SetProjectRoots(paths []string)
// }

type PathMapperProvider interface {
	HasPathMapper() bool
	GetPathMapper() *utils.DockerPathMapper
	NormalizeURIForLSP(uri string) string
}
