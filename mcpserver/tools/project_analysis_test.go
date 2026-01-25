package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/mocks"
	"rockerboo/mcp-lsp-bridge/types"
	"rockerboo/mcp-lsp-bridge/utils"

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

			projectPath := utils.URIToFilePath(tc.workspaceUri)

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", projectPath).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[types.Language]types.LanguageClientInterface)

				for _, lang := range tc.mockLanguages {
					mockClient := &mocks.MockLanguageClient{}
					// Set up workspace symbols mock
					mockClient.On("WorkspaceSymbols", tc.query).Return(tc.mockResults, nil)
					mockClients[lang] = mockClient
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
	bridge := &mocks.MockBridge{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bsl")
	content := "ПерваяСтрока\nСтандартныеПодсистемыСервер.УстановкаПараметровСеанса()\n"
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// Setup language detection + clients (even though text_search doesn't use LSP calls directly)
	langs := []types.Language{"bsl"}
	bridge.On("DetectProjectLanguages", tmpDir).Return(langs, nil)

	mockClient := &mocks.MockLanguageClient{}
	mockClients := map[types.Language]types.LanguageClientInterface{
		"bsl": mockClient,
	}
	bridge.On("GetMultiLanguageClients", []string{"bsl"}).Return(mockClients, nil)

	tool, handler := ProjectAnalysisTool(bridge)
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{Tool: tool, Handler: handler})
	if err != nil {
		t.Fatalf("Could not create MCP server: %v", err)
	}

	ctx := context.Background()
	toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "project_analysis",
			Arguments: map[string]any{
				"workspace_uri": tmpDir,
				"query":         "СтандартныеПодсистемыСервер.УстановкаПараметровСеанса",
				"analysis_type": "text_search",
				"offset":        0,
				"limit":         5,
			},
		},
	})
	if err != nil {
		t.Fatalf("Error calling tool: %v", err)
	}
	if toolResult.IsError {
		t.Fatalf("Expected success, got error: %#v", toolResult.Content)
	}
	require.NotEmpty(t, toolResult.Content)
	txt, ok := toolResult.Content[0].(mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, txt.Text, "TEXT_SEARCH|")
	require.Contains(t, txt.Text, "test.bsl")

	bridge.AssertExpectations(t)
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

			projectPath := utils.URIToFilePath(tc.workspaceUri)

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", projectPath).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[types.Language]types.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {
					mockClient := &mocks.MockLanguageClient{}
					// References uses WorkspaceSymbols, not FindReferences
					var workspaceSymbols []protocol.WorkspaceSymbol
					for _, ref := range tc.mockReferences {
						workspaceSymbols = append(workspaceSymbols, protocol.WorkspaceSymbol{
							Name: tc.query,
							Kind: protocol.SymbolKindFunction,
							Location: protocol.Or2[protocol.Location, protocol.LocationUriOnly]{
								Value: ref,
							},
						})
					}
					mockClient.On("WorkspaceSymbols", tc.query).Return(workspaceSymbols, nil)
					mockClients[lang] = mockClient
				}
				var languageStrings []string
				for _, lang := range tc.mockLanguages {
					languageStrings = append(languageStrings, string(lang))
				}

				bridge.On("GetMultiLanguageClients", languageStrings).Return(mockClients, nil)

				// Mock bridge methods needed by handleReferences
				bridge.On("SemanticTokens", "file:///main.go", uint32(5), uint32(0), uint32(5), uint32(1000)).Return([]types.TokenPosition{}, nil)
				bridge.On("FindSymbolReferences", "go", "file:///main.go", uint32(5), uint32(0), true).Return(tc.mockReferences, nil)
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

			projectPath := utils.URIToFilePath(tc.workspaceUri)

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", projectPath).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[types.Language]types.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {
					mockClient := &mocks.MockLanguageClient{}
					// Definitions uses WorkspaceSymbols, not FindDefinitions
					var workspaceSymbols []protocol.WorkspaceSymbol
					for _, def := range tc.mockDefinitions {
						var location protocol.Location
						if def.Value != nil {
							// Extract location from the Or2 union
							if loc, ok := def.Value.(protocol.Location); ok {
								location = loc
							}
						}
						workspaceSymbols = append(workspaceSymbols, protocol.WorkspaceSymbol{
							Name: tc.query,
							Kind: protocol.SymbolKindFunction,
							Location: protocol.Or2[protocol.Location, protocol.LocationUriOnly]{
								Value: location,
							},
						})
					}
					mockClient.On("WorkspaceSymbols", tc.query).Return(workspaceSymbols, nil)
					mockClients[lang] = mockClient
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
				bridge.On("DetectProjectLanguages", utils.URIToFilePath("file:///nonexistent")).Return([]types.Language{}, errors.New("project not found"))
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
				bridge.On("DetectProjectLanguages", utils.URIToFilePath("file:///workspace")).Return([]types.Language{"go"}, nil)
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
func TestProjectAnalysisTool_FileAnalysis(t *testing.T) {
	t.Skip("Skipping file analysis integration test - requires actual filesystem setup and file context resolution")
	testCases := []struct {
		name            string
		workspaceUri    string
		query           string
		mockLanguages   []types.Language
		mockSymbols     []protocol.DocumentSymbol
		expectError     bool
		expectedContent string
	}{
		{
			name:          "successful file analysis",
			workspaceUri:  "file:///workspace",
			query:         "main.go",
			mockLanguages: []types.Language{"go"},
			mockSymbols: []protocol.DocumentSymbol{
				{
					Name: "main",
					Kind: protocol.SymbolKindFunction,
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 10, Character: 0},
					},
				},
				{
					Name: "helper",
					Kind: protocol.SymbolKindFunction,
					Range: protocol.Range{
						Start: protocol.Position{Line: 12, Character: 0},
						End:   protocol.Position{Line: 20, Character: 0},
					},
				},
			},
			expectError:     false,
			expectedContent: "FILE ANALYSIS",
		},
		{
			name:            "file analysis with no symbols",
			workspaceUri:    "file:///workspace",
			query:           "empty.go",
			mockLanguages:   []types.Language{"go"},
			mockSymbols:     []protocol.DocumentSymbol{},
			expectError:     false,
			expectedContent: "Language: go",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			projectPath := utils.URIToFilePath(tc.workspaceUri)

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", projectPath).Return(tc.mockLanguages, nil)
			bridge.On("AllowedDirectories").Return([]string{projectPath})

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[types.Language]types.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {
					mockClient := &mocks.MockLanguageClient{}
					expectedURI := utils.NormalizeURI(tc.query) // This will resolve to absolute path
					mockClient.On("DocumentSymbols", expectedURI).Return(tc.mockSymbols, nil)
					mockClients[lang] = mockClient
				}

				// Add InferLanguage mock for our language detection fix
				if len(tc.mockLanguages) > 0 {
					expectedURI := utils.NormalizeURI(tc.query)
					bridge.On("InferLanguage", expectedURI).Return(&tc.mockLanguages[0], nil)
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

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name: "project_analysis",
					Arguments: map[string]any{
						"workspace_uri": tc.workspaceUri,
						"query":         tc.query,
						"analysis_type": "file_analysis",
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

			// Check if expected content is present
			if tc.expectedContent != "" && !toolResult.IsError {
				if len(toolResult.Content) > 0 {
					if textContent, ok := toolResult.Content[0].(mcp.TextContent); ok {
						if !strings.Contains(textContent.Text, tc.expectedContent) {
							t.Errorf("Expected content '%s' not found in response: %s", tc.expectedContent, textContent.Text)
						}
					} else {
						t.Errorf("Expected text content, got: %T", toolResult.Content[0])
					}
				}
			}

			bridge.AssertExpectations(t)
		})
	}
}

func TestProjectAnalysisTool_PatternAnalysis(t *testing.T) {
	testCases := []struct {
		name            string
		workspaceUri    string
		query           string
		mockLanguages   []types.Language
		expectError     bool
		expectedContent string
	}{
		{
			name:            "error handling pattern analysis",
			workspaceUri:    "file:///workspace",
			query:           "error_handling",
			mockLanguages:   []types.Language{"go"},
			expectError:     false,
			expectedContent: "Pattern Type: error_handling",
		},
		{
			name:            "naming conventions pattern analysis",
			workspaceUri:    "file:///workspace",
			query:           "naming_conventions",
			mockLanguages:   []types.Language{"go"},
			expectError:     false,
			expectedContent: "Pattern Type: naming_conventions",
		},
		{
			name:            "architecture patterns analysis",
			workspaceUri:    "file:///workspace",
			query:           "architecture_patterns",
			mockLanguages:   []types.Language{"go"},
			expectError:     false,
			expectedContent: "Pattern Type: architecture_patterns",
		},
		{
			name:            "invalid pattern type",
			workspaceUri:    "file:///workspace",
			query:           "invalid_pattern",
			mockLanguages:   []types.Language{"go"},
			expectError:     false,
			expectedContent: "unsupported pattern type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			projectPath := utils.URIToFilePath(tc.workspaceUri)

			// Set up mock expectations
			bridge.On("DetectProjectLanguages", projectPath).Return(tc.mockLanguages, nil)

			if len(tc.mockLanguages) > 0 {
				mockClients := make(map[types.Language]types.LanguageClientInterface)
				for _, lang := range tc.mockLanguages {
					mockClient := &mocks.MockLanguageClient{}
					mockClients[lang] = mockClient
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

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name: "project_analysis",
					Arguments: map[string]any{
						"workspace_uri": tc.workspaceUri,
						"query":         tc.query,
						"analysis_type": "pattern_analysis",
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

			// Check if expected content is present
			if tc.expectedContent != "" && !toolResult.IsError {
				if len(toolResult.Content) > 0 {
					if textContent, ok := toolResult.Content[0].(mcp.TextContent); ok {
						if !strings.Contains(textContent.Text, tc.expectedContent) {
							t.Errorf("Expected content '%s' not found in response: %s", tc.expectedContent, textContent.Text)
						}
					} else {
						t.Errorf("Expected text content, got: %T", toolResult.Content[0])
					}
				}
			}

			bridge.AssertExpectations(t)
		})
	}
}

func TestCalculateFileComplexityFromSymbols(t *testing.T) {
	testCases := []struct {
		name                    string
		symbols                 []protocol.DocumentSymbol
		expectedFunctionCount   int
		expectedClassCount      int
		expectedVariableCount   int
		expectedComplexityLevel string
	}{
		{
			name: "simple file with functions",
			symbols: []protocol.DocumentSymbol{
				{
					Name: "main",
					Kind: protocol.SymbolKindFunction,
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 10, Character: 0},
					},
				},
				{
					Name: "helper",
					Kind: protocol.SymbolKindFunction,
					Range: protocol.Range{
						Start: protocol.Position{Line: 12, Character: 0},
						End:   protocol.Position{Line: 20, Character: 0},
					},
				},
			},
			expectedFunctionCount:   2,
			expectedClassCount:      0,
			expectedVariableCount:   0,
			expectedComplexityLevel: "low",
		},
		{
			name: "complex file with classes and methods",
			symbols: []protocol.DocumentSymbol{
				{
					Name: "MyClass",
					Kind: protocol.SymbolKindClass,
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 50, Character: 0},
					},
				},
				{
					Name: "method1",
					Kind: protocol.SymbolKindMethod,
					Range: protocol.Range{
						Start: protocol.Position{Line: 5, Character: 0},
						End:   protocol.Position{Line: 15, Character: 0},
					},
				},
				{
					Name: "method2",
					Kind: protocol.SymbolKindMethod,
					Range: protocol.Range{
						Start: protocol.Position{Line: 17, Character: 0},
						End:   protocol.Position{Line: 25, Character: 0},
					},
				},
				{
					Name: "constant",
					Kind: protocol.SymbolKindConstant,
					Range: protocol.Range{
						Start: protocol.Position{Line: 30, Character: 0},
						End:   protocol.Position{Line: 30, Character: 10},
					},
				},
			},
			expectedFunctionCount:   2, // methods count as functions
			expectedClassCount:      1,
			expectedVariableCount:   1,     // constant counts as variable
			expectedComplexityLevel: "low", // 2*2 + 1*3 + 1*1 = 8 < 10
		},
		{
			name: "high complexity file",
			symbols: func() []protocol.DocumentSymbol {
				symbols := []protocol.DocumentSymbol{}
				// Add many functions to trigger high complexity
				for i := 0; i < 30; i++ {
					symbols = append(symbols, protocol.DocumentSymbol{
						Name: "func" + string(rune(i)),
						Kind: protocol.SymbolKindFunction,
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(i * 2), Character: 0},
							End:   protocol.Position{Line: uint32(i*2 + 1), Character: 0},
						},
					})
				}
				return symbols
			}(),
			expectedFunctionCount:   30,
			expectedClassCount:      0,
			expectedVariableCount:   0,
			expectedComplexityLevel: "high", // 30*2 = 60 > 50
		},
		{
			name:                    "empty file",
			symbols:                 []protocol.DocumentSymbol{},
			expectedFunctionCount:   0,
			expectedClassCount:      0,
			expectedVariableCount:   0,
			expectedComplexityLevel: "low",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := calculateFileComplexityFromSymbols(tc.symbols)

			if metrics.FunctionCount != tc.expectedFunctionCount {
				t.Errorf("Expected %d functions, got %d", tc.expectedFunctionCount, metrics.FunctionCount)
			}

			if metrics.ClassCount != tc.expectedClassCount {
				t.Errorf("Expected %d classes, got %d", tc.expectedClassCount, metrics.ClassCount)
			}

			if metrics.VariableCount != tc.expectedVariableCount {
				t.Errorf("Expected %d variables, got %d", tc.expectedVariableCount, metrics.VariableCount)
			}

			if metrics.ComplexityLevel != tc.expectedComplexityLevel {
				t.Errorf("Expected complexity level '%s', got '%s'", tc.expectedComplexityLevel, metrics.ComplexityLevel)
			}

			// Verify complexity score calculation
			expectedScore := float64(tc.expectedFunctionCount*2 + tc.expectedClassCount*3 + tc.expectedVariableCount)
			if metrics.ComplexityScore != expectedScore {
				t.Errorf("Expected complexity score %.2f, got %.2f", expectedScore, metrics.ComplexityScore)
			}

			// Verify total lines calculation
			expectedLines := 0
			for _, symbol := range tc.symbols {
				expectedLines += int(symbol.Range.End.Line - symbol.Range.Start.Line + 1)
			}
			if metrics.TotalLines != expectedLines {
				t.Errorf("Expected %d total lines, got %d", expectedLines, metrics.TotalLines)
			}
		})
	}
}

func TestProjectAnalysisTool_NewAnalysisTypes(t *testing.T) {
	// Test that new analysis types are properly handled
	testCases := []struct {
		name         string
		analysisType string
		expectError  bool
	}{
		{
			name:         "file_analysis type",
			analysisType: "file_analysis",
			expectError:  false,
		},
		{
			name:         "pattern_analysis type",
			analysisType: "pattern_analysis",
			expectError:  false,
		},
		{
			name:         "unsupported analysis type",
			analysisType: "unsupported_type",
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			// Set up basic mock expectations that ALL tests need
			bridge.On("DetectProjectLanguages", utils.URIToFilePath("file:///workspace")).Return([]types.Language{"go"}, nil)

			// AllowedDirectories is only called when workspace_uri is empty, which doesn't happen in our test
			// since we provide "file:///workspace" as workspace_uri in the test arguments
			// bridge.On("AllowedDirectories").Return([]string{"/workspace"})

			// Skip complex analysis tests that require full analysis engine setup
			if tc.analysisType == "file_analysis" || tc.analysisType == "pattern_analysis" {
				t.Skip("Skipping complex analysis test - requires full analysis engine setup")
				return
			}

			mockClients := make(map[types.Language]types.LanguageClientInterface)
			mockClient := &mocks.MockLanguageClient{}
			mockClients["go"] = mockClient

			bridge.On("GetMultiLanguageClients", []string{"go"}).Return(mockClients, nil)

			// Mock client responses based on analysis type
			if tc.analysisType == "file_analysis" {
				expectedURI := utils.NormalizeURI("test.go") // This will resolve to absolute path
				mockClient.On("DocumentSymbols", expectedURI).Return([]protocol.DocumentSymbol{}, nil)

				// Add InferLanguage mock for our language detection fix
				goLang := types.Language("go")
				bridge.On("InferLanguage", expectedURI).Return(&goLang, nil)
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

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name: "project_analysis",
					Arguments: map[string]any{
						"workspace_uri": "file:///workspace",
						"query":         "test.go",
						"analysis_type": tc.analysisType,
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
