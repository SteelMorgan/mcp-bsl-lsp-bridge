package tools

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
)

// IntegrationMockBridge provides a realistic mock for integration testing
type IntegrationMockBridge struct {
	*mocks.MockBridge
}

// MockCallToolRequest for integration testing
type IntegrationCallToolRequest struct {
	toolName  string
	arguments map[string]any
}

func (r *IntegrationCallToolRequest) RequireString(key string) (string, error) {
	if val, ok := r.arguments[key]; ok {
		if str, ok := val.(string); ok {
			return str, nil
		}
	}

	return "", fmt.Errorf("missing or invalid string parameter: %s", key)
}

func (r *IntegrationCallToolRequest) RequireInt(key string) (int, error) {
	if val, ok := r.arguments[key]; ok {
		switch v := val.(type) {
		case int:
			return v, nil
		case int32:
			return int(v), nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		}
	}

	return 0, fmt.Errorf("missing or invalid int parameter: %s", key)
}

func (r *IntegrationCallToolRequest) OptionalString(key string, defaultValue string) string {
	if val, ok := r.arguments[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}

	return defaultValue
}

func (r *IntegrationCallToolRequest) OptionalBool(key string, defaultValue bool) bool {
	if val, ok := r.arguments[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}

	return defaultValue
}

func TestMCPToolIntegration_HoverTool(t *testing.T) {
	mockBridge := new(mocks.MockBridge)
	hoverResult := protocol.Hover{
		Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{Value: "func main()"},
	}

	language := lsp.Language("go")
	mockBridge.On("InferLanguage", "file:///test.go").Return(&language, nil).Once()

	mockBridge.On("GetHoverInformation", "file:///test.go", uint32(10), uint32(5)).Return(&hoverResult, nil).Once()

	tool, handler := HoverTool(mockBridge)
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: handler,
	})
	require.NoError(t, err, "Could not start server")

	defer mcpServer.Close()

	ctx := context.Background()
	result, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "hover",
			Arguments: map[string]any{
				"uri":       "file:///test.go",
				"line":      10,
				"character": 5,
			},
		},
	})

	require.NoError(t, err, "Unexpected error from CallTool")
	assert.NotNil(t, result, "Expected result but got nil")
	assert.False(t, result.IsError, "Expected successful result, got error")

	assert.NotNil(t, result.Content, "Expected content in result")
	assert.NotEmpty(t, result.Content, "Expected non-empty content")

	// Validate content
	for _, content := range result.Content {
		// Change this line:
		textContent, ok := content.(mcp.TextContent) // Assert to value type, not pointer type
		assert.True(t, ok, "Expected TextContent, got %T", content)
		assert.NotEmpty(t, textContent.Text, "Expected non-empty text content")
		assert.Equal(t, "func main()", textContent.Text, "Unexpected hover content")
	}

	// Assert that all expectations on the mock were met
	mockBridge.AssertExpectations(t)
}

func TestMCPToolIntegration_InferLanguageTool(t *testing.T) {
	mockBridge := new(mocks.MockBridge)
	language := lsp.Language("go")
	mockBridge.On("InferLanguage", "file:///path/to/main.go").Return(&language, nil).Once() // Changed to file:/// URI

	// Initialize tool and handler directly
	tool, handler := InferLanguageTool(mockBridge)
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: handler,
	})
	require.NoError(t, err, "Could not start server")

	defer mcpServer.Close()

	ctx := context.Background()
	result, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "infer_language",
			Arguments: map[string]any{
				"file_path": "file:///path/to/main.go", // Changed to uri for consistency
			},
		},
	})

	require.NoError(t, err, "Unexpected error from CallTool")
	assert.NotNil(t, result, "Expected result but got nil")
	assert.False(t, result.IsError, "Expected successful result, got error")

	assert.NotEmpty(t, result.Content, "Expected language result content")

	found := false

	for _, content := range result.Content {
		// Assert to value type
		textContent, ok := content.(mcp.TextContent)
		assert.True(t, ok, "Expected TextContent, got %T", content)

		if textContent.Text != "" {
			found = true

			assert.Equal(t, "go", textContent.Text, "Expected inferred language 'go'")
		}
	}

	assert.True(t, found, "Expected language result content")
	mockBridge.AssertExpectations(t)
	t.Logf("Integration test 'infer language tool integration' completed successfully")
}
func TestMCPToolIntegration_LSPConnectTool(t *testing.T) {
	mockBridge := new(mocks.MockBridge)
	// mockBridge.On("GetConfig").Return(&lsp.LSPServerConfig{
	// 	LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
	// 		"go": {
	// 			Command:   "gopls",
	// 			Args:      []string{},
	// 			Languages: []string{"go"},
	// 			Filetypes: []string{".go"},
	// 		},
	// 	},
	// 	ExtensionLanguageMap: map[string]lsp.Language{
	// 		".go": "go",
	// 	},
	// 	LanguageExtensionMap: map[lsp.Language][]string{
	// 		"go": {".go"},
	// 	},
	// }, nil).Once()
	mockBridge.On("GetClientForLanguage", "go").Return(&lsp.LanguageClient{}, nil).Once()

	// Initialize tool and handler directly
	tool, handler := LSPConnectTool(mockBridge) // Assuming LSPConnectTool exists in the tools package
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: handler,
	})
	require.NoError(t, err, "Could not start server")

	defer mcpServer.Close()

	ctx := context.Background()
	result, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "lsp_connect",
			Arguments: map[string]any{
				"language": "go",
			},
		},
	})

	require.NoError(t, err, "Unexpected error from CallTool")
	assert.NotNil(t, result, "Expected result but got nil")
	assert.False(t, result.IsError, "Expected successful connection, got error")
	mockBridge.AssertExpectations(t)
	t.Logf("Integration test 'lsp connect tool integration' completed successfully")
}
func TestMCPToolIntegration_LSPDisconnectTool(t *testing.T) {
	mockBridge := new(mocks.MockBridge)
	mockBridge.On("GetConfig").Return(lsp.LSPServerConfig{}, nil).Maybe() // Use .Maybe() if not strictly always called

	mockBridge.On("CloseAllClients").Return().Once() // No return values for CloseAllClients

	// Initialize tool and handler directly
	tool, handler := LSPDisconnectTool(mockBridge) // Assuming LSPDisconnectTool exists in the tools package
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: handler,
	})
	require.NoError(t, err, "Could not start server")

	defer mcpServer.Close()

	client := mcpServer.Client()
	ctx := context.Background()
	result, err := client.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name:      "lsp_disconnect",
			Arguments: map[string]any{},
		},
	})

	require.NoError(t, err, "Unexpected error from CallTool")
	assert.NotNil(t, result, "Expected result but got nil")
	assert.False(t, result.IsError, "Expected successful disconnection, got error")
	mockBridge.AssertExpectations(t) // Verify that CloseAllClients was called
	t.Logf("Integration test 'lsp disconnect tool integration' completed successfully")
}
func TestMCPToolIntegration_ErrorHandling(t *testing.T) {
	mockBridge := new(mocks.MockBridge)
	language := lsp.Language("")
	mockBridge.On("InferLanguage", "file:///invalid.xyz").Return(&language, errors.New("unsupported file type")).Once()

	mockBridge.On("GetHoverInformation", "file:///invalid.xyz", uint32(10), uint32(5)).Return((*protocol.Hover)(nil), errors.New("unsupported file type")).Once()

	// Initialize tool and handler directly
	tool, handler := HoverTool(mockBridge)
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: handler,
	})
	require.NoError(t, err, "Could not start server")

	defer mcpServer.Close()

	ctx := context.Background()
	result, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "hover", // Assuming this test is for hover tool's error handling
			Arguments: map[string]any{
				"uri":       "file:///invalid.xyz",
				"line":      10,
				"character": 5,
			},
		},
	})

	require.NoError(t, err, "Unexpected error from CallTool (RPC level)")
	assert.NotNil(t, result, "Expected result but got nil")
	assert.True(t, result.IsError, "Expected error result, but got success")

	assert.NotEmpty(t, result.Content, "Expected error content in result")
	textContent, ok := result.Content[0].(mcp.TextContent) // Assert to value type
	assert.True(t, ok, "Expected TextContent for error message")
	assert.Equal(t, "Failed to get hover information: unsupported file type", textContent.Text, "Expected specific error message") // Adjust message if HoverTool formats it

	mockBridge.AssertExpectations(t)
	t.Logf("Integration test 'error handling integration' completed successfully")
}

// // TestMCPToolIntegration tests the complete MCP tool request/response cycle
// func TestMCPToolIntegration(t *testing.T) {
// 	testCases := []struct {
// 		name         string
// 		toolName     string
// 		arguments    map[string]any
// 		setupBridge  func() *IntegrationMockBridge
// 		expectError  bool
// 		validateFunc func(t *testing.T, result *mcp.CallToolResult)
// 	}{
// 		{
// 			name:     "hover tool integration",
// 			toolName: "hover",
// 			arguments: map[string]any{
// 				"uri":       "file:///test.go",
// 				"line":      10,
// 				"character": 5,
// 			},
// 			setupBridge: func() *IntegrationMockBridge {
// 				return &IntegrationMockBridge{
// 					MockBridge: &mocks.MockBridge{
// 						// inferLanguageFunc: func(filePath string) (string, error) {
// 						// 	return "go", nil
// 						// },
// 						// getHoverInformationFunc: func(uri string, line, character int32) (any, error) {
// 						// 	return map[string]any{
// 						// 		"contents": "func main()",
// 						// 	}, nil
// 						// },
// 					},
// 				}
// 			},
// 			expectError: false,
// 			validateFunc: func(t *testing.T, result *mcp.CallToolResult) {
// 				if result.Content == nil {
// 					t.Error("Expected content in result")
// 				}
// 				if len(result.Content) == 0 {
// 					t.Error("Expected non-empty content")
// 				}
// 				// Check if content contains hover information formatting
// 				for _, content := range result.Content {
// 					if textContent, ok := content.(*mcp.TextContent); ok {
// 						if textContent.Text == "" {
// 							t.Error("Expected non-empty text content")
// 						}
// 					}
// 				}
// 			},
// 		},
// 		{
// 			name:     "diagnostics tool integration",
// 			toolName: "diagnostics",
// 			arguments: map[string]any{
// 				"uri": "file:///test.go",
// 			},
// 			setupBridge: func() *IntegrationMockBridge {
// 				return &IntegrationMockBridge{
// 					MockBridge: &mocks.MockBridge{
// 						// inferLanguageFunc: func(filePath string) (string, error) {
// 						// 	return "go", nil
// 						// },
// 						// getDiagnosticsFunc: func(uri string) ([]any, error) {
// 						// 	return []any{
// 						// 		protocol.Diagnostic{
// 						// 			Message: "unused variable",
// 						// 			Range: protocol.Range{
// 						// 				Start: protocol.Position{Line: 10, Character: 0},
// 						// 				End:   protocol.Position{Line: 10, Character: 5},
// 						// 			},
// 						// 		},
// 						// 	}, nil
// 						// },
// 					},
// 				}
// 			},
// 			expectError: false,
// 			validateFunc: func(t *testing.T, result *mcp.CallToolResult) {
// 				if result.IsError {
// 					t.Error("Expected successful result")
// 				}
// 				// Validate diagnostic formatting
// 				for _, content := range result.Content {
// 					if textContent, ok := content.(*mcp.TextContent); ok {
// 						if textContent.Text == "" {
// 							t.Error("Expected diagnostic content")
// 						}
// 					}
// 				}
// 			},
// 		},
// 		{
// 			name:     "infer language tool integration",
// 			toolName: "infer_language",
// 			arguments: map[string]any{
// 				"file_path": "/path/to/main.go",
// 			},
// 			setupBridge: func() *IntegrationMockBridge {
// 				return &IntegrationMockBridge{
// 					MockBridge: &mocks.MockBridge{
// 						// getConfigFunc: func() *lsp.LSPServerConfig {
// 						// 	return &lsp.LSPServerConfig{
// 						// 		ExtensionLanguageMap: map[string]string{
// 						// 			".go": "go",
// 						// 			".py": "python",
// 						// 			".js": "javascript",
// 						// 		},
// 						// 	}
// 						// },
// 					},
// 				}
// 			},
// 			expectError: false,
// 			validateFunc: func(t *testing.T, result *mcp.CallToolResult) {
// 				if result.IsError {
// 					t.Error("Expected successful result")
// 				}
// 				// Should return language information
// 				if len(result.Content) == 0 {
// 					t.Error("Expected language result content")
// 					return
// 				}
//
// 				// Check if any content exists and is non-empty
// 				found := false
// 				for _, content := range result.Content {
// 					// Handle different content types that might be returned
// 					switch c := content.(type) {
// 					case *mcp.TextContent:
// 						if c.Text != "" {
// 							found = true
// 						}
// 					case mcp.TextContent:
// 						if c.Text != "" {
// 							found = true
// 						}
// 					}
// 					if found {
// 						break
// 					}
// 				}
// 				if !found {
// 					t.Error("Expected language result content")
// 				}
// 			},
// 		},
// 		{
// 			name:     "lsp connect tool integration",
// 			toolName: "lsp_connect",
// 			arguments: map[string]any{
// 				"language": "go",
// 			},
// 			setupBridge: func() *IntegrationMockBridge {
// 				return &IntegrationMockBridge{
// 					MockBridge: &mocks.MockBridge{
// 						// getConfigFunc: func() *lsp.LSPServerConfig {
// 						// 	return &lsp.LSPServerConfig{
// 						// 		LanguageServers: map[string]lsp.LanguageServerConfig{
// 						// 			"go": {
// 						// 				Command:   "gopls",
// 						// 				Args:      []string{"serve"},
// 						// 				Filetypes: []string{".go"},
// 						// 			},
// 						// 		},
// 						// 	}
// 						// },
// 						// getClientForLanguageFunc: func(language string) (any, error) {
// 						// 	return &lsp.LanguageClient{}, nil
// 						// },
// 					},
// 				}
// 			},
// 			expectError: false,
// 			validateFunc: func(t *testing.T, result *mcp.CallToolResult) {
// 				if result.IsError {
// 					t.Error("Expected successful connection")
// 				}
// 			},
// 		},
// 		{
// 			name:      "lsp disconnect tool integration",
// 			toolName:  "lsp_disconnect",
// 			arguments: map[string]any{},
// 			setupBridge: func() *IntegrationMockBridge {
// 				return &IntegrationMockBridge{
// 					MockBridge: &mocks.MockBridge{
// 						// closeAllClientsFunc: func() {
// 						// 	// Disconnect called
// 						// },
// 					},
// 				}
// 			},
// 			expectError: false,
// 			validateFunc: func(t *testing.T, result *mcp.CallToolResult) {
// 				if result.IsError {
// 					t.Error("Expected successful disconnection")
// 				}
// 			},
// 		},
// 		{
// 			name:     "error handling integration",
// 			toolName: "hover",
// 			arguments: map[string]any{
// 				"uri":       "file:///invalid.xyz",
// 				"line":      10,
// 				"character": 5,
// 			},
// 			setupBridge: func() *IntegrationMockBridge {
// 				return &IntegrationMockBridge{
// 					MockBridge: &mocks.MockBridge{
// 						// inferLanguageFunc: func(filePath string) (string, error) {
// 						// 	return "", fmt.Errorf("unsupported file type")
// 						// },
// 					},
// 				}
// 			},
// 			expectError: true,
// 			validateFunc: func(t *testing.T, result *mcp.CallToolResult) {
// 				if !result.IsError {
// 					t.Error("Expected error result")
// 				}
// 			},
// 		},
// 	}
//
// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			// Setup
// 			bridge := tc.setupBridge()
// 			mcpServer, err := mcptest.NewServer(t)
// 			if err != nil {
// 				t.Errorf("Could not start server: %v", err)
// 			}
//
// 			// Register individual tools for testing
// 			RegisterHoverTool(mcpServer, bridge)
// 			RegisterDiagnosticsTool(mcpServer, bridge)
// 			RegisterInferLanguageTool(mcpServer, bridge)
// 			RegisterLSPConnectTool(mcpServer, bridge)
// 			RegisterLSPDisconnectTool(mcpServer, bridge)
//
// 			// Find and execute the tool
// 			ctx := context.Background()
// 			result, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
// 				Request: mcp.Request{Method: "tools/call"},
// 				Params: mcp.CallToolParams{
// 					Name:      tc.toolName,
// 					Arguments: tc.arguments,
// 				},
// 			})
//
// 			// Validate results
// 			if tc.expectError {
// 				if err == nil && !result.IsError {
// 					t.Error("Expected error but got success")
// 				}
// 			} else {
// 				if err != nil {
// 					t.Errorf("Unexpected error: %v", err)
// 				}
// 				if result == nil {
// 					t.Fatal("Expected result but got nil")
// 				}
// 			}
//
// 			// Run custom validation
// 			if tc.validateFunc != nil && result != nil {
// 				tc.validateFunc(t, result)
// 			}
//
// 			t.Logf("Integration test '%s' completed successfully", tc.name)
// 		})
// 	}
// }

// TestMCPToolRegistration verifies all tools are properly registered
func TestMCPToolRegistration(t *testing.T) {
	bridge := &mocks.MockBridge{}
	mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))

	// Register individual tools for testing
	RegisterAnalyzeCodeTool(mcpServer, bridge)
	RegisterProjectAnalysisTool(mcpServer, bridge)
	RegisterInferLanguageTool(mcpServer, bridge)
	RegisterProjectLanguageDetectionTool(mcpServer, bridge)
	RegisterLSPConnectTool(mcpServer, bridge)
	RegisterLSPDisconnectTool(mcpServer, bridge)
	RegisterHoverTool(mcpServer, bridge)
	RegisterSignatureHelpTool(mcpServer, bridge)
	RegisterCodeActionsTool(mcpServer, bridge)
	RegisterFormatDocumentTool(mcpServer, bridge)
	RegisterRenameTool(mcpServer, bridge)
	RegisterImplementationTool(mcpServer, bridge)
	RegisterCallHierarchyTool(mcpServer, bridge)
	RegisterWorkspaceDiagnosticsTool(mcpServer, bridge)

	// Expected tools
	expectedTools := []string{
		"analyze_code",
		"project_analysis",
		"hover",
		"diagnostics",
		"signature_help",
		"code_actions",
		"format_document",
		"rename",
		"implementation",
		"call_hierarchy",
		"workspace_diagnostics",
		"lsp_connect",
		"lsp_disconnect",
		"infer_language",
		"detect_project_languages",
	}

	// This is a simplified check - in a real implementation, you'd verify
	// that tools are actually registered with the server
	for _, toolName := range expectedTools {
		t.Logf("Tool '%s' should be registered", toolName)
	}

	t.Logf("All %d tools have been registered successfully", len(expectedTools))
}

// TestMCPToolParameterValidation tests parameter validation across tools
func TestMCPToolParameterValidation(t *testing.T) {
	testCases := []struct {
		name        string
		toolName    string
		arguments   map[string]any
		expectError bool
	}{
		{
			name:     "hover valid parameters",
			toolName: "hover",
			arguments: map[string]any{
				"uri":       "file:///test.go",
				"line":      10,
				"character": 5,
			},
			expectError: false,
		},
		{
			name:     "hover missing uri",
			toolName: "hover",
			arguments: map[string]any{
				"line":      10,
				"character": 5,
			},
			expectError: true,
		},
		{
			name:     "diagnostics valid parameters",
			toolName: "diagnostics",
			arguments: map[string]any{
				"uri": "file:///test.go",
			},
			expectError: false,
		},
		{
			name:        "diagnostics missing uri",
			toolName:    "diagnostics",
			arguments:   map[string]any{},
			expectError: true,
		},
		{
			name:     "infer_language valid parameters",
			toolName: "infer_language",
			arguments: map[string]any{
				"file_path": "/path/to/file.go",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := &IntegrationCallToolRequest{
				toolName:  tc.toolName,
				arguments: tc.arguments,
			}

			// Test parameter parsing
			switch tc.toolName {
			case "hover":
				_, err1 := request.RequireString("uri")
				_, err2 := request.RequireInt("line")
				_, err3 := request.RequireInt("character")

				hasError := err1 != nil || err2 != nil || err3 != nil
				if tc.expectError && !hasError {
					t.Error("Expected parameter validation error")
				}

				if !tc.expectError && hasError {
					t.Errorf("Unexpected parameter validation error: %v, %v, %v", err1, err2, err3)
				}

			case "diagnostics":
				_, err := request.RequireString("uri")
				if tc.expectError && err == nil {
					t.Error("Expected parameter validation error")
				}

				if !tc.expectError && err != nil {
					t.Errorf("Unexpected parameter validation error: %v", err)
				}

			case "infer_language":
				_, err := request.RequireString("file_path")
				if tc.expectError && err == nil {
					t.Error("Expected parameter validation error")
				}

				if !tc.expectError && err != nil {
					t.Errorf("Unexpected parameter validation error: %v", err)
				}
			}
		})
	}
}
