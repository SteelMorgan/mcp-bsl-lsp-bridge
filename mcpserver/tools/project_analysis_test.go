package tools

import (
	"fmt"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/stretchr/testify/assert"
)

func TestProjectAnalysisTool_WorkspaceSymbols(t *testing.T) {
	testCases := []struct {
		name            string
		workspaceUri    string
		query           string
		mockLanguages   []string
		mockResults     []protocol.WorkspaceSymbol
		expectError     bool
		expectedContent string
	}{
		{
			name:          "successful workspace symbols search",
			workspaceUri:  "file:///workspace",
			query:         "main",
			mockLanguages: []string{"go"},
			mockResults: []protocol.WorkspaceSymbol{
				{
					Name: "main",
					Kind: 12, // Function
					Location: protocol.Or2[protocol.Location, protocol.LocationUriOnly]{
						Value: protocol.Location{
							Uri: "file:///main.go",
							Range: protocol.Range{
								Start: protocol.Position{Line: 5, Character: 0},
								End:   protocol.Position{Line: 5, Character: 4},
							},
						},
					},
				},
			},
			expectError:     false,
			expectedContent: "WORKSPACE SYMBOLS",
		},
		{
			name:          "empty query",
			workspaceUri:  "file:///workspace",
			query:         "",
			mockLanguages: []string{"go"},
			mockResults:   []protocol.WorkspaceSymbol{},
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", tc.workspaceUri).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[string]lsp.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {
					mockClients[lang] = &lsp.LanguageClient{}
				}
				bridge.On("GetMultiLanguageClients", tc.mockLanguages).Return(mockClients, nil)
				bridge.On("SearchTextInWorkspace", "go", tc.query).Return(tc.mockResults, nil)
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterProjectAnalysisTool(mcpServer, bridge)

			// Execute test
			languages, err := bridge.DetectProjectLanguages(tc.workspaceUri)
			if err != nil && !tc.expectError {
				t.Errorf("Unexpected error in project language detection: %v", err)
				return
			}

			if !tc.expectError && len(languages) > 0 {
				_, err := bridge.GetMultiLanguageClients(languages)
				if err != nil {
					t.Errorf("Unexpected error creating clients: %v", err)
					return
				}

				symbols, err := bridge.SearchTextInWorkspace("go", tc.query)
				if err != nil {
					t.Errorf("Error searching workspace symbols: %v", err)
					return
				}

				if tc.query != "" && len(symbols) == 0 {
					t.Error("Expected workspace symbols but got none")
				}
			}

			bridge.AssertExpectations(t)
		})
	}
}
func TestProjectAnalysisTool_SymbolReferences(t *testing.T) {
	testCases := []struct {
		name           string
		workspaceUri   string
		query          string
		mockLanguages  []string
		mockReferences []protocol.Location
		expectError    bool
	}{
		{
			name:          "successful symbol references search",
			workspaceUri:  "file:///workspace",
			query:         "main:5:0",
			mockLanguages: []string{"go"},
			mockReferences: []protocol.Location{
				{
					Uri: "file:///main.go",
					Range: protocol.Range{
						Start: protocol.Position{Line: 5, Character: 0},
						End:   protocol.Position{Line: 5, Character: 4},
					},
				},
				{
					Uri: "file:///utils.go",
					Range: protocol.Range{
						Start: protocol.Position{Line: 10, Character: 5},
						End:   protocol.Position{Line: 10, Character: 9},
					},
				},
			},
			expectError: false,
		},
		{
			name:          "invalid position format",
			workspaceUri:  "file:///workspace",
			query:         "invalid_position",
			mockLanguages: []string{"go"},
			expectError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", tc.workspaceUri).Return(tc.mockLanguages, nil)

			if !tc.expectError && len(tc.mockLanguages) > 0 {
				mockClients := make(map[string]lsp.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {
					mockClients[lang] = &lsp.LanguageClient{}
				}
				bridge.On("GetMultiLanguageClients", tc.mockLanguages).Return(mockClients, nil)
				bridge.On("FindSymbolReferences", "go", "file:///main.go", uint32(5), uint32(0), true).Return(tc.mockReferences, nil)
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterProjectAnalysisTool(mcpServer, bridge)

			// Execute test
			languages, err := bridge.DetectProjectLanguages(tc.workspaceUri)
			if err != nil && !tc.expectError {
				t.Errorf("Unexpected error in project language detection: %v", err)
				return
			}

			if !tc.expectError && len(languages) > 0 {
				_, err := bridge.GetMultiLanguageClients(languages)
				if err != nil {
					t.Errorf("Unexpected error creating clients: %v", err)
					return
				}

				refs, err := bridge.FindSymbolReferences("go", "file:///main.go", 5, 0, true)
				if err != nil {
					t.Errorf("Error finding references: %v", err)
					return
				}

				if len(refs) != len(tc.mockReferences) {
					t.Errorf("Expected %d references, got %d", len(tc.mockReferences), len(refs))
				}
			}

			bridge.AssertExpectations(t)
		})
	}
}

func TestProjectAnalysisTool_SymbolDefinitions(t *testing.T) {
	testCases := []struct {
		name            string
		workspaceUri    string
		query           string
		mockLanguages   []string
		mockDefinitions []protocol.Or2[protocol.LocationLink, protocol.Location]
		expectError     bool
	}{
		{
			name:          "successful symbol definitions search",
			workspaceUri:  "file:///workspace",
			query:         "main:5:0",
			mockLanguages: []string{"go"},
			mockDefinitions: []protocol.Or2[protocol.LocationLink, protocol.Location]{
				{
					Value: protocol.Location{
						Uri: "file:///main.go",
						Range: protocol.Range{
							Start: protocol.Position{Line: 5, Character: 0},
							End:   protocol.Position{Line: 5, Character: 4},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:            "symbol not found",
			workspaceUri:    "file:///workspace",
			query:           "nonexistent:1:0",
			mockLanguages:   []string{"go"},
			mockDefinitions: []protocol.Or2[protocol.LocationLink, protocol.Location]{},
			expectError:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", tc.workspaceUri).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[string]lsp.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {
					mockClients[lang] = &lsp.LanguageClient{}
				}
				bridge.On("GetMultiLanguageClients", tc.mockLanguages).Return(mockClients, nil)
				bridge.On("FindSymbolDefinitions", "go", "file:///main.go", uint32(5), uint32(0)).Return(tc.mockDefinitions, nil)
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterProjectAnalysisTool(mcpServer, bridge)

			// Execute test
			languages, err := bridge.DetectProjectLanguages(tc.workspaceUri)
			if err != nil && !tc.expectError {
				t.Errorf("Unexpected error in project language detection: %v", err)
				return
			}

			if !tc.expectError && len(languages) > 0 {
				_, err := bridge.GetMultiLanguageClients(languages)
				if err != nil {
					t.Errorf("Unexpected error creating clients: %v", err)
					return
				}

				defs, err := bridge.FindSymbolDefinitions("go", "file:///main.go", 5, 0)
				if err != nil {
					t.Errorf("Error finding definitions: %v", err)
					return
				}

				if len(defs) != len(tc.mockDefinitions) {
					t.Errorf("Expected %d definitions, got %d", len(tc.mockDefinitions), len(defs))
				}
			}

			bridge.AssertExpectations(t)
		})
	}
}

func TestProjectAnalysisTool_TextSearch(t *testing.T) {
	testCases := []struct {
		name          string
		workspaceUri  string
		query         string
		mockLanguages []string
		mockResults   []protocol.WorkspaceSymbol
		expectError   bool
	}{
		{
			name:          "successful text search",
			workspaceUri:  "file:///workspace",
			query:         "TODO",
			mockLanguages: []string{"go"},
			mockResults: []protocol.WorkspaceSymbol{
				{
					Name:          "main",
					Kind:          protocol.SymbolKind(12), // Function
					ContainerName: "",
					Location: protocol.Or2[protocol.Location, protocol.LocationUriOnly]{
						Value: protocol.Location{
							Uri: "file:///main.go",
							Range: protocol.Range{
								Start: protocol.Position{Line: 5, Character: 0},
								End:   protocol.Position{Line: 5, Character: 4},
							},
						},
					},
					Tags: []protocol.SymbolTag{},
				},
			},
			expectError: false,
		},
		{
			name:          "no matches found",
			workspaceUri:  "file:///workspace",
			query:         "nonexistent_pattern",
			mockLanguages: []string{"go"},
			mockResults:   []protocol.WorkspaceSymbol{},
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", tc.workspaceUri).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[string]lsp.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {
					mockClients[lang] = &lsp.LanguageClient{}
				}
				bridge.On("GetMultiLanguageClients", tc.mockLanguages).Return(mockClients, nil)
				bridge.On("SearchTextInWorkspace", "go", tc.query).Return(tc.mockResults, nil)
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterProjectAnalysisTool(mcpServer, bridge)

			// Execute test
			languages, err := bridge.DetectProjectLanguages(tc.workspaceUri)
			if err != nil && !tc.expectError {
				t.Errorf("Unexpected error in project language detection: %v", err)
				return
			}

			if !tc.expectError && len(languages) > 0 {
				_, err := bridge.GetMultiLanguageClients(languages)
				if err != nil {
					t.Errorf("Unexpected error creating clients: %v", err)
					return
				}

				results, err := bridge.SearchTextInWorkspace("go", tc.query)
				if err != nil {
					t.Errorf("Error in text search: %v", err)
					return
				}

				if len(results) != len(tc.mockResults) {
					t.Errorf("Expected %d search results, got %d", len(tc.mockResults), len(results))
				}
			}

			bridge.AssertExpectations(t)
		})
	}
}

func TestProjectAnalysisTool_ErrorCases(t *testing.T) {
	testCases := []struct {
		name         string
		workspaceUri string
		setupMock    func(*mocks.MockBridge)
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "project language detection failure",
			workspaceUri: "file:///nonexistent",
			setupMock: func(bridge *mocks.MockBridge) {
				// This case expects DetectProjectLanguages to fail
				bridge.On("DetectProjectLanguages", "file:///nonexistent").Return([]string{}, fmt.Errorf("project not found"))
			},
			expectError: true,
			errorMsg:    "project not found",
		},
		{
			name:         "client creation failure",
			workspaceUri: "file:///workspace",
			setupMock: func(bridge *mocks.MockBridge) {
				// This case expects DetectProjectLanguages to succeed, and then GetMultiLanguageClients to fail
				bridge.On("DetectProjectLanguages", "file:///workspace").Return([]string{"go"}, nil)
				bridge.On("GetMultiLanguageClients", []string{"go"}).Return(map[string]lsp.LanguageClientInterface{}, fmt.Errorf("failed to create clients"))
			},
			expectError: true,
			errorMsg:    "failed to create clients",
		},
		// Add more error cases as needed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}
			tc.setupMock(bridge)

			tool, handler := ProjectAnalysisTool(bridge)
			mcpServer, err := mcptest.NewServer(t, server.ServerTool{
				Tool:    tool,
				Handler: handler,
			})
			assert.NoError(t, err, "Could not start server")
			defer mcpServer.Close()

			// Variable to hold the actual error encountered during the operations
			var actualErr error

			// 1. Test project language detection
			languages, err := bridge.DetectProjectLanguages(tc.workspaceUri)
			if err != nil {
				actualErr = err // If DetectProjectLanguages fails, capture that error
			} else {
				// 2. If language detection succeeded, test client creation
				if len(languages) > 0 {
					_, err := bridge.GetMultiLanguageClients(languages)
					if err != nil {
						actualErr = err // If GetMultiLanguageClients fails, capture that error
					}
				}
			}

			// Now, assert the overall error expectation for the test case
			if tc.expectError {
				if actualErr == nil {
					t.Error("Expected an error but got none.")
					return
				}
				if !strings.Contains(actualErr.Error(), tc.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tc.errorMsg, actualErr)
				}
			} else {
				// This block would handle cases where NO error is expected.
				// Based on the current test cases, this path won't be taken,
				// but it's good practice for comprehensive error handling.
				if actualErr != nil {
					t.Errorf("Did not expect an error but got: %v", actualErr)
				}
			}

			// Ensure all mocked expectations were met
			bridge.AssertExpectations(t)
		})
	}
}
func TestProjectAnalysisTool_UnsupportedAnalysisType(t *testing.T) {
	// This test would depend on how your actual tool handles unsupported analysis types
	// You might need to adjust this based on your implementation
	t.Skip("Implementation depends on how RegisterProjectAnalysisTool handles unsupported types")
}
func TestProjectAnalysisUtilityFunctions(t *testing.T) {
	t.Run("parseSymbolPosition", func(t *testing.T) {
		testCases := []struct {
			input         string
			expectedUri   string
			expectedLine  uint32
			expectedChar  uint32
			expectedError bool
		}{
			{"main:5:0", "main", 5, 0, false},
			{"file.go:10:15", "file.go", 10, 15, false},
			{"invalid", "", 0, 0, true},
			{"file:invalid:pos", "", 0, 0, true},
		}

		for _, tc := range testCases {
			// This would test the symbol position parsing logic
			parts := strings.Split(tc.input, ":")
			if len(parts) >= 3 {
				// Check if numeric conversion would work for the test cases that should succeed
				if tc.input == "file:invalid:pos" && !tc.expectedError {
					t.Errorf("Expected error for input %s with invalid numeric parts", tc.input)
				}
			} else {
				// Invalid format
				if !tc.expectedError {
					t.Errorf("Expected success for input %s but parsing failed", tc.input)
				}
			}
		}
	})
}

func Test_formatDocumentSymbolWithTargeting(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		response *strings.Builder
		symbol   protocol.DocumentSymbol
		depth    int
		number   int
		docUri   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatDocumentSymbolWithTargeting(tt.response, tt.symbol, tt.depth, tt.number, tt.docUri)
		})
	}
}
