package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/mock"
)

func TestGetRangeContentTool(t *testing.T) {
	// Create a temporary test file
	testContent := `line 0: first line
line 1: second line with more content
line 2: third line
line 3: fourth line with even more content here
line 4: fifth line`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte(testContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	testCases := []struct {
		name         string
		uri          string
		startLine    int
		startChar    int
		endLine      int
		endChar      int
		strict       *bool // nil means not provided
		expectError  bool
		expectedText string
		description  string
		shouldAdjust bool // whether we expect character adjustments in non-strict mode
	}{
		{
			name:         "single line exact range",
			uri:          "file://" + testFile,
			startLine:    0,
			startChar:    0,
			endLine:      0,
			endChar:      6,
			expectError:  false,
			expectedText: "line 0",
			description:  "Should extract exact range from single line",
		},
		{
			name:         "single line mid-range",
			uri:          "file://" + testFile,
			startLine:    1,
			startChar:    8,
			endLine:      1,
			endChar:      14,
			expectError:  false,
			expectedText: "second",
			description:  "Should extract middle portion of line",
		},
		{
			name:         "multi-line range",
			uri:          "file://" + testFile,
			startLine:    1,
			startChar:    8,
			endLine:      2,
			endChar:      11,
			expectError:  false,
			expectedText: "second line with more content\nline 2: th",
			description:  "Should extract multi-line range correctly",
		},
		{
			name:         "full line range",
			uri:          "file://" + testFile,
			startLine:    2,
			startChar:    0,
			endLine:      2,
			endChar:      18,
			expectError:  false,
			expectedText: "line 2: third line",
			description:  "Should extract full line content",
		},
		{
			name:         "out of bounds end char - non-strict default",
			uri:          "file://" + testFile,
			startLine:    0,
			startChar:    0,
			endLine:      0,
			endChar:      100, // way beyond line end
			expectError:  false,
			expectedText: "line 0: first line",
			description:  "Should clamp to line end in non-strict mode (default)",
			shouldAdjust: true,
		},
		{
			name:        "out of bounds end char - strict mode",
			uri:         "file://" + testFile,
			startLine:   0,
			startChar:   0,
			endLine:     0,
			endChar:     100,
			strict:      boolPtr(true),
			expectError: true,
			description: "Should fail with out of bounds char in strict mode",
		},
		{
			name:         "out of bounds start char - non-strict",
			uri:          "file://" + testFile,
			startLine:    0,
			startChar:    100,
			endLine:      0,
			endChar:      100,
			expectError:  false,
			expectedText: "",
			description:  "Should handle start char beyond line end in non-strict mode",
			shouldAdjust: true,
		},
		{
			name:        "out of bounds start char - strict mode",
			uri:         "file://" + testFile,
			startLine:   0,
			startChar:   100,
			endLine:     0,
			endChar:     100,
			strict:      boolPtr(true),
			expectError: true,
			description: "Should fail with out of bounds start char in strict mode",
		},
		{
			name:         "multi-line with out of bounds end char - non-strict",
			uri:          "file://" + testFile,
			startLine:    1,
			startChar:    0,
			endLine:      2,
			endChar:      100,
			expectError:  false,
			expectedText: "line 1: second line with more content\nline 2: third line",
			description:  "Should clamp end char in multi-line non-strict mode",
			shouldAdjust: true,
		},
		{
			name:        "multi-line with out of bounds end char - strict",
			uri:         "file://" + testFile,
			startLine:   1,
			startChar:   0,
			endLine:     2,
			endChar:     100,
			strict:      boolPtr(true),
			expectError: true,
			description: "Should fail with out of bounds end char in multi-line strict mode",
		},
		{
			name:        "invalid line range - start line out of bounds",
			uri:         "file://" + testFile,
			startLine:   10,
			startChar:   0,
			endLine:     10,
			endChar:     5,
			expectError: true,
			description: "Should fail when start line is out of bounds",
		},
		{
			name:        "invalid line range - end line out of bounds",
			uri:         "file://" + testFile,
			startLine:   0,
			startChar:   0,
			endLine:     10,
			endChar:     5,
			expectError: true,
			description: "Should fail when end line is out of bounds",
		},
		{
			name:        "invalid range order - lines",
			uri:         "file://" + testFile,
			startLine:   2,
			startChar:   0,
			endLine:     1,
			endChar:     5,
			expectError: true,
			description: "Should fail when start line > end line",
		},
		{
			name:        "invalid range order - characters on same line",
			uri:         "file://" + testFile,
			startLine:   1,
			startChar:   10,
			endLine:     1,
			endChar:     5,
			expectError: true,
			description: "Should fail when start char > end char on same line",
		},
		{
			name:        "file not found",
			uri:         "file:///nonexistent/file.txt",
			startLine:   0,
			startChar:   0,
			endLine:     0,
			endChar:     5,
			expectError: true,
			description: "Should fail when file doesn't exist",
		},
		{
			name:        "invalid URI parameter",
			uri:         "", // empty URI
			startLine:   0,
			startChar:   0,
			endLine:     0,
			endChar:     5,
			expectError: true,
			description: "Should fail with invalid URI",
		},
		{
			name:         "range at exact line boundary",
			uri:          "file://" + testFile,
			startLine:    0,
			startChar:    0,
			endLine:      0,
			endChar:      18, // exact length of first line
			expectError:  false,
			expectedText: "line 0: first line",
			description:  "Should handle range at exact line boundary",
		},
		{
			name:         "empty range - same position",
			uri:          "file://" + testFile,
			startLine:    1,
			startChar:    5,
			endLine:      1,
			endChar:      5,
			expectError:  false,
			expectedText: "",
			description:  "Should handle empty range (same start and end position)",
		},
		{
			name:         "explicit non-strict mode",
			uri:          "file://" + testFile,
			startLine:    0,
			startChar:    0,
			endLine:      0,
			endChar:      50,
			strict:       boolPtr(false),
			expectError:  false,
			expectedText: "line 0: first line",
			description:  "Should clamp to line end when explicitly non-strict",
			shouldAdjust: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock bridge
			bridge := &mocks.MockBridge{}
			if tc.uri != "" && tc.uri != "file:///nonexistent/file.txt" {
				// Mock successful file path resolution
				bridge.On("IsAllowedDirectory", testFile).Return(testFile, nil)
			} else if tc.uri == "file:///nonexistent/file.txt" {
				// Mock file path resolution for non-existent file
				bridge.On("IsAllowedDirectory", "/nonexistent/file.txt").Return("/nonexistent/file.txt", nil)
			} else if tc.uri == "" {
				// For empty URI, the normalized path will be the current working directory or empty
				// We need to mock this call since the code will still try to resolve the path
				bridge.On("IsAllowedDirectory", mock.AnythingOfType("string")).Return("", errors.New("invalid file path"))
			}

			// Create MCP server and register tool
			tool, handler := RangeContentTool(bridge)
			mcpServer, err := mcptest.NewServer(t, server.ServerTool{
				Tool:    tool,
				Handler: handler,
			})
			if err != nil {
				t.Fatalf("Could not create MCP server: %v", err)
			}

			ctx := context.Background()

			// Build request arguments
			args := map[string]any{
				"uri":             tc.uri,
				"start_line":      tc.startLine,
				"start_character": tc.startChar,
				"end_line":        tc.endLine,
				"end_character":   tc.endChar,
			}

			// Add strict parameter if specified
			if tc.strict != nil {
				args["strict"] = *tc.strict
			}

			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name:      "get_range_content",
					Arguments: args,
				},
			})

			if err != nil {
				t.Fatalf("Could not make request: %v", err)
			}

			// Check error expectations
			if tc.expectError && !toolResult.IsError {
				t.Errorf("Expected error but got none. Result: %+v", toolResult)
			} else if !tc.expectError && toolResult.IsError {
				t.Errorf("Unexpected error: %v", toolResult.Content)
			}

			// Check content if no error expected
			if !tc.expectError && !toolResult.IsError {
				if len(toolResult.Content) == 0 {
					t.Error("Expected content but got none")
				} else {
					// Assuming the content is text content
					if textContent, ok := toolResult.Content[0].(mcp.TextContent); ok {
						if textContent.Text != tc.expectedText {
							t.Errorf("Expected text:\n%q\nGot:\n%q", tc.expectedText, textContent.Text)
						}
					} else {
						t.Errorf("Expected text content, got: %T", toolResult.Content[0])
					}
				}
			}

			// Verify mock expectations
			bridge.AssertExpectations(t)
		})
	}
}

// Test parameter validation separately
func TestGetRangeContentTool_ParameterValidation(t *testing.T) {
	parameterTests := []struct {
		name         string
		args         map[string]any
		checkAllowed bool
		expectError  bool
		description  string
	}{
		{
			name: "missing URI",
			args: map[string]any{
				"start_line":      0,
				"start_character": 0,
				"end_line":        0,
				"end_character":   5,
			},
			checkAllowed: true,
			expectError:  true,
			description:  "Should fail when URI is missing",
		},
		{
			name: "missing start_line",
			args: map[string]any{
				"uri":             "file:///test.txt",
				"start_character": 0,
				"end_line":        0,
				"end_character":   5,
			},
			checkAllowed: false,
			expectError:  true,
			description:  "Should fail when start_line is missing",
		},
		{
			name: "invalid start_line type",
			args: map[string]any{
				"uri":             "file:///test.txt",
				"start_line":      "invalid",
				"start_character": 0,
				"end_line":        0,
				"end_character":   5,
			},
			checkAllowed: false,
			expectError:  true,
			description:  "Should fail when start_line is not a number",
		},
		{
			name: "negative start_line",
			args: map[string]any{
				"uri":             "file:///test.txt",
				"start_line":      -1,
				"start_character": 0,
				"end_line":        0,
				"end_character":   5,
			},
			checkAllowed: true,
			expectError: true,
			description: "Should handle negative start_line (converted to large uint32)",
		},
	}

	for _, tc := range parameterTests {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh mock bridge for each test
			bridge := &mocks.MockBridge{}

			// Mock IsAllowedDirectory for any URI that might be processed
			// Some parameter validation happens before URI processing, but others happen after
			if uri, hasURI := tc.args["uri"].(string); hasURI && uri != "" {
				// Mock the URI processing - it will fail anyway due to file not existing
				// or other parameter issues, but we need to mock the call
				normalizedPath := uri
				if uri == "file:///test.txt" {
					normalizedPath = "/test.txt"
				}
				if tc.checkAllowed {
					bridge.On("IsAllowedDirectory", normalizedPath).Return(normalizedPath, nil)
				}
			}

			tool, handler := RangeContentTool(bridge)
			mcpServer, err := mcptest.NewServer(t, server.ServerTool{
				Tool:    tool,
				Handler: handler,
			})
			if err != nil {
				t.Fatalf("Could not create MCP server: %v", err)
			}

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name:      "get_range_content",
					Arguments: tc.args,
				},
			})

			if err != nil {
				t.Fatalf("Could not make request: %v", err)
			}

			if tc.expectError && !toolResult.IsError {
				t.Error("Expected error but got none")
			} else if !tc.expectError && toolResult.IsError {
				t.Errorf("Unexpected error: %v", toolResult.Content)
			}

			// Verify mock expectations if any were set
			bridge.AssertExpectations(t)
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
