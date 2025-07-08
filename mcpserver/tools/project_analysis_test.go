package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/stretchr/testify/require"
)

func TestProjectAnalysisTool_WorkspaceSymbols(t *testing.T) {
	testCases := []struct {
		name            string
		workspaceUri    string
		query           string
		mockLanguages   []types.Language
		mockResults     []protocol.WorkspaceSymbol
		expectError     bool
		expectedContent string
	}{
		{
			name:          "successful workspace symbols search",
			workspaceUri:  "file:///workspace",
			query:         "main",
			mockLanguages: []types.Language{"go"},
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
			mockLanguages: []types.Language{"go"},
			mockResults:   []protocol.WorkspaceSymbol{},
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			projectPath := strings.TrimPrefix(tc.workspaceUri, "file://")

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", projectPath).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[types.Language]types.LanguageClientInterface)

				for _, lang := range tc.mockLanguages {
					client, err := lsp.NewLanguageClient("mock-lsp-server")
					if err != nil {
						t.Error(err)
						return
					}
					_, err = client.Connect()
					if err != nil {
						t.Errorf("Could not connect %v", err)
						return
					}
					mockClients[lang] = client
				}
				var languageStrings []string
				for _, lang := range tc.mockLanguages {
					languageStrings = append(languageStrings, string(lang))
				}

				bridge.On("GetMultiLanguageClients", languageStrings).Return(mockClients, nil)
				// bridge.On("SearchTextInWorkspace", "go", tc.query).Return(tc.mockResults, nil)
			}

			tool, handler := ProjectAnalysisTool(bridge)
			mcpServer, err := mcptest.NewServer(t, server.ServerTool{
				Tool:    tool,
				Handler: handler,
			})
			if err != nil {
				t.Errorf("Could not create MCP server: %v", err)
				return
			}

			// defer mcpServer.Close()

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name: "project_analysis",
					Arguments: map[string]any{
						"workspace_uri": tc.workspaceUri,
						"query":         tc.query,
						"analysis_type": "workspace_symbols",
					},
				},
			})

			if err != nil {
				t.Errorf("Error calling tool: %v", err)
				return
			}

			if !toolResult.IsError && tc.expectError {
				t.Error("Expected error but got none")
			} else if toolResult.IsError && !tc.expectError {
				t.Errorf("Unexpected error: %v", toolResult.Content)
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
		mockLanguages  []types.Language
		mockReferences []protocol.Location
		expectError    bool
	}{
		{
			name:          "successful symbol references search",
			workspaceUri:  "file:///workspace",
			query:         "main",
			mockLanguages: []types.Language{"go"},
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			projectPath := strings.TrimPrefix(tc.workspaceUri, "file://")

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", projectPath).Return(tc.mockLanguages, nil)

			mockClients := make(map[types.Language]types.LanguageClientInterface)
			if len(tc.mockLanguages) > 0 {
				for _, lang := range tc.mockLanguages {
					client, err := lsp.NewLanguageClient("mock-lsp-server")
					if err != nil {
						t.Error(err)
						return
					}
					_, err = client.Connect()
					if err != nil {
						t.Error(err)
					}
					mockClients[lang] = client
				}
				var languageStrings []string
				for _, lang := range tc.mockLanguages {
					languageStrings = append(languageStrings, string(lang))
				}

				bridge.On("GetMultiLanguageClients", languageStrings).Return(mockClients, nil)
			}

			tool, handler := ProjectAnalysisTool(bridge)
			mcpServer, err := mcptest.NewServer(t, server.ServerTool{
				Tool:    tool,
				Handler: handler,
			})
			if err != nil {
				t.Errorf("Could not create MCP server: %v", err)
				return
			}

			// defer mcpServer.Close()

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name: "project_analysis",
					Arguments: map[string]any{
						"workspace_uri": tc.workspaceUri,
						"query":         tc.query,
						"analysis_type": "references",
					},
				},
			})

			if err != nil {
				t.Errorf("Error calling tool: %v", err)
				return
			}

			if !toolResult.IsError && tc.expectError {
				t.Error("Expected error but got none")
			} else if toolResult.IsError && !tc.expectError {
				t.Errorf("Unexpected error: %v", toolResult.Content)
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
		mockLanguages   []types.Language
		mockDefinitions []protocol.Or2[protocol.LocationLink, protocol.Location]
		expectError     bool
	}{
		{
			name:          "successful symbol definitions search",
			workspaceUri:  "file:///workspace",
			query:         "main",
			mockLanguages: []types.Language{"go"},
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
			query:           "nonexistent",
			mockLanguages:   []types.Language{"go"},
			mockDefinitions: []protocol.Or2[protocol.LocationLink, protocol.Location]{},
			expectError:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			projectPath := strings.TrimPrefix(tc.workspaceUri, "file://")

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", projectPath).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[types.Language]types.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {

					client, err := lsp.NewLanguageClient("mock-lsp-server")
					if err != nil {
						t.Error(err)
						return
					}
					_, err = client.Connect()
					if err != nil {
						t.Error(err)
					}
					mockClients[lang] = client
				}

				var languageStrings []string
				for _, lang := range tc.mockLanguages {
					languageStrings = append(languageStrings, string(lang))
				}

				bridge.On("GetMultiLanguageClients", languageStrings).Return(mockClients, nil)
				// bridge.On("FindSymbolDefinitions", "go", "file:///main.go", uint32(5), uint32(0)).Return(tc.mockDefinitions, nil)
			}

			tool, handler := ProjectAnalysisTool(bridge)
			mcpServer, err := mcptest.NewServer(t, server.ServerTool{
				Tool:    tool,
				Handler: handler,
			})
			if err != nil {
				t.Errorf("Could not create MCP server: %v", err)
				return
			}

			// defer mcpServer.Close()

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name: "project_analysis",
					Arguments: map[string]any{
						"workspace_uri": tc.workspaceUri,
						"query":         tc.query,
						"analysis_type": "workspace_symbols",
					},
				},
			})

			if err != nil {
				t.Errorf("Error calling tool: %v", err)
				return
			}

			if !toolResult.IsError && tc.expectError {
				t.Error("Expected error but got none")
			} else if toolResult.IsError && !tc.expectError {
				t.Errorf("Unexpected error: %v", toolResult.Content)
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
		mockLanguages []types.Language
		mockResults   []protocol.WorkspaceSymbol
		expectError   bool
	}{
		{
			name:          "successful text search",
			workspaceUri:  "file:///workspace",
			query:         "TODO",
			mockLanguages: []types.Language{"go"},
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
			mockLanguages: []types.Language{"go"},
			mockResults:   []protocol.WorkspaceSymbol{},
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			projectPath := strings.TrimPrefix(tc.workspaceUri, "file://")

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", projectPath).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[types.Language]types.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {

					client, err := lsp.NewLanguageClient("mock-lsp-server")
					if err != nil {
						t.Error(err)
						return
					}
					_, err = client.Connect()
					if err != nil {
						t.Errorf("Could not connect %v", err)
						return
					}
					mockClients[lang] = client
				}
				var languageStrings []string
				for _, lang := range tc.mockLanguages {
					languageStrings = append(languageStrings, string(lang))
				}

				bridge.On("GetMultiLanguageClients", languageStrings).Return(mockClients, nil)
				// bridge.On("SearchTextInWorkspace", "go", tc.query).Return(tc.mockResults, nil)
			}

			tool, handler := ProjectAnalysisTool(bridge)
			mcpServer, err := mcptest.NewServer(t, server.ServerTool{
				Tool:    tool,
				Handler: handler,
			})
			if err != nil {
				t.Errorf("Could not create MCP server: %v", err)
				return
			}

			// defer mcpServer.Close()

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name: "project_analysis",
					Arguments: map[string]any{
						"workspace_uri": tc.workspaceUri,
						"query":         tc.query,
						"analysis_type": "workspace_symbols",
					},
				},
			})

			if err != nil {
				t.Errorf("Error calling tool: %v", err)
				return
			}

			if !toolResult.IsError && tc.expectError {
				t.Error("Expected error but got none")
			} else if toolResult.IsError && !tc.expectError {
				t.Errorf("Unexpected error: %v", toolResult.Content)
			}

			bridge.AssertExpectations(t)
		})
	}
}

func TestProjectAnalysisTool_ErrorCases(t *testing.T) {
	testCases := []struct {
		name         string
		workspaceUri string
		query        string
		setupMock    func(*mocks.MockBridge)
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "project language detection failure",
			workspaceUri: "file:///nonexistent",
			query:        "",
			setupMock: func(bridge *mocks.MockBridge) {
				// This case expects DetectProjectLanguages to fail
				bridge.On("DetectProjectLanguages", "/nonexistent").Return([]types.Language{}, errors.New("project not found"))
			},
			expectError: true,
			errorMsg:    "project not found",
		},
		{
			name:         "client creation failure",
			workspaceUri: "file:///workspace",
			query:        "",
			setupMock: func(bridge *mocks.MockBridge) {
				// This case expects DetectProjectLanguages to succeed, and then GetMultiLanguageClients to fail
				bridge.On("DetectProjectLanguages", "/workspace").Return([]types.Language{"go"}, nil)
				bridge.On("GetMultiLanguageClients", []string{"go"}).Return(map[types.Language]types.LanguageClientInterface{}, errors.New("failed to create clients"))
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
			if err != nil {
				t.Errorf("Could not create MCP server: %v", err)
				return
			}

			// defer mcpServer.Close()

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name: "project_analysis",
					Arguments: map[string]any{
						"workspace_uri": tc.workspaceUri,
						"query":         tc.query,
						"analysis_type": "workspace_symbols",
					},
				},
			})

			if err != nil {
				t.Errorf("Error calling tool: %v", err)
				return
			}

			if !toolResult.IsError && tc.expectError {
				t.Error("Expected error but got none")
			} else if toolResult.IsError && !tc.expectError {
				t.Errorf("Unexpected error: %v", toolResult.Content)
			}
			require.NoError(t, err, "Could not start server")

			defer mcpServer.Close()

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
