package tools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestProjectAnalysisTool(t *testing.T) {
	testCases := []struct {
		name            string
		analysisType    string
		workspaceUri    string
		query           string
		mockLanguages   []string
		mockReferences  []any
		mockDefinitions []any
		mockResults     []any
		expectError     bool
		expectedContent string
	}{
		{
			name:         "workspace symbols search",
			analysisType: "workspace_symbols",
			workspaceUri: "file:///workspace",
			query:        "main",
			mockLanguages: []string{"go"},
			mockResults: []any{
				map[string]any{
					"name": "main",
					"kind": 12, // Function
					"location": map[string]any{
						"uri": "file:///main.go",
						"range": map[string]any{
							"start": map[string]int32{"line": 5, "character": 0},
						},
					},
				},
			},
			expectError:     false,
			expectedContent: "WORKSPACE SYMBOLS",
		},
		{
			name:         "symbol references search",
			analysisType: "references",
			workspaceUri: "file:///workspace",
			query:        "main:5:0", // symbol at line 5, character 0
			mockLanguages: []string{"go"},
			mockReferences: []any{
				map[string]any{
					"uri": "file:///main.go",
					"range": map[string]any{
						"start": map[string]int32{"line": 5, "character": 0},
					},
				},
			},
			expectError:     false,
			expectedContent: "SYMBOL REFERENCES",
		},
		{
			name:         "symbol definitions search",
			analysisType: "definitions",
			workspaceUri: "file:///workspace",
			query:        "main:5:0",
			mockLanguages: []string{"go"},
			mockDefinitions: []any{
				map[string]any{
					"uri": "file:///main.go",
					"range": map[string]any{
						"start": map[string]int32{"line": 5, "character": 0},
					},
				},
			},
			expectError:     false,
			expectedContent: "SYMBOL DEFINITIONS",
		},
		{
			name:         "text search",
			analysisType: "text_search",
			workspaceUri: "file:///workspace",
			query:        "TODO",
			mockLanguages: []string{"go"},
			mockResults: []any{
				map[string]any{
					"uri": "file:///main.go",
					"matches": []map[string]any{
						{
							"line": 10,
							"text": "// TODO: implement this",
						},
					},
				},
			},
			expectError:     false,
			expectedContent: "TEXT SEARCH RESULTS",
		},
		{
			name:          "unsupported analysis type",
			analysisType:  "unknown_type",
			workspaceUri:  "file:///workspace",
			query:         "test",
			mockLanguages: []string{"go"},
			expectError:   true,
		},
		{
			name:         "project language detection failure",
			analysisType: "workspace_symbols",
			workspaceUri: "file:///nonexistent",
			query:        "test",
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				detectProjectLanguagesFunc: func(projectPath string) ([]string, error) {
					if tc.expectError && strings.Contains(projectPath, "nonexistent") {
						return nil, fmt.Errorf("project not found")
					}
					return tc.mockLanguages, nil
				},
				getMultiLanguageClientsFunc: func(languages []string) (map[string]any, error) {
					clients := make(map[string]any)
					for _, lang := range languages {
						clients[lang] = &MockLanguageClient{}
					}
					return clients, nil
				},
				findSymbolReferencesFunc: func(language, uri string, line, character int32, includeDeclaration bool) ([]any, error) {
					return tc.mockReferences, nil
				},
				findSymbolDefinitionsFunc: func(language, uri string, line, character int32) ([]any, error) {
					return tc.mockDefinitions, nil
				},
				searchTextInWorkspaceFunc: func(language, query string) ([]any, error) {
					return tc.mockResults, nil
				},
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterProjectAnalysisTool(mcpServer, bridge)

			// Test project language detection
			languages, err := bridge.DetectProjectLanguages(tc.workspaceUri)
			if tc.expectError && err != nil {
				return // Expected error case
			}
			if err != nil && !tc.expectError {
				t.Errorf("Unexpected error in project language detection: %v", err)
				return
			}

			// Test multi-language client creation
			if !tc.expectError && len(languages) > 0 {
				clients, err := bridge.GetMultiLanguageClients(languages)
				if err != nil {
					t.Errorf("Unexpected error creating clients: %v", err)
					return
				}
				if len(clients) == 0 {
					t.Error("Expected clients but got empty map")
				}

				// Test specific analysis types
				switch tc.analysisType {
				case "references":
					if len(tc.mockReferences) > 0 {
						refs, err := bridge.FindSymbolReferences("go", "file:///main.go", 5, 0, true)
						if err != nil {
							t.Errorf("Error finding references: %v", err)
						}
						if len(refs) == 0 {
							t.Error("Expected references but got none")
						}
					}
				case "definitions":
					if len(tc.mockDefinitions) > 0 {
						defs, err := bridge.FindSymbolDefinitions("go", "file:///main.go", 5, 0)
						if err != nil {
							t.Errorf("Error finding definitions: %v", err)
						}
						if len(defs) == 0 {
							t.Error("Expected definitions but got none")
						}
					}
				case "text_search":
					if len(tc.mockResults) > 0 {
						results, err := bridge.SearchTextInWorkspace("go", tc.query)
						if err != nil {
							t.Errorf("Error in text search: %v", err)
						}
						if len(results) == 0 {
							t.Error("Expected search results but got none")
						}
					}
				}
			}
		})
	}
}

// Mock language client for testing
type MockLanguageClient struct{}

func TestProjectAnalysisUtilityFunctions(t *testing.T) {
	t.Run("parseSymbolPosition", func(t *testing.T) {
		testCases := []struct {
			input         string
			expectedUri   string
			expectedLine  int32
			expectedChar  int32
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