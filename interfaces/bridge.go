package interfaces

import "rockerboo/mcp-lsp-bridge/lsp"

// BridgeInterface defines the interface that the bridge must implement
type BridgeInterface interface {
	GetClientForLanguageInterface(language string) (any, error)
	InferLanguage(filePath string) (string, error)
	CloseAllClients()
	GetConfig() *lsp.LSPServerConfig
	DetectProjectLanguages(projectPath string) ([]string, error)
	DetectPrimaryProjectLanguage(projectPath string) (string, error)
	// Enhanced project analysis methods
	FindSymbolReferences(language, uri string, line, character int32, includeDeclaration bool) ([]any, error)
	FindSymbolDefinitions(language, uri string, line, character int32) ([]any, error)
	SearchTextInWorkspace(language, query string) ([]any, error)
	GetMultiLanguageClients(languages []string) (map[string]any, error)
	// Core information tools
	GetHoverInformation(uri string, line, character int32) (any, error)
	GetDiagnostics(uri string) ([]any, error)
	GetWorkspaceDiagnostics(workspaceUri string, identifier string) (any, error)
	GetSignatureHelp(uri string, line, character int32) (any, error)
	// Code actions and formatting tools
	GetCodeActions(uri string, line, character, endLine, endCharacter int32) ([]any, error)
	FormatDocument(uri string, tabSize int32, insertSpaces bool) ([]any, error)
	// Advanced navigation tools
	RenameSymbol(uri string, line, character int32, newName string, preview bool) (any, error)
	FindImplementations(uri string, line, character int32) ([]any, error)
	PrepareCallHierarchy(uri string, line, character int32) ([]any, error)
	GetIncomingCalls(item any) ([]any, error)
	GetOutgoingCalls(item any) ([]any, error)
}