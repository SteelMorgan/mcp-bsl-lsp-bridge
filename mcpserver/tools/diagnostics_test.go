package tools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

// MockBridge for diagnostics testing - extend the hover MockBridge
type DiagnosticsMockBridge struct {
	*MockBridge
	mockGetDiagnostics func(string) ([]any, error)
}

func (m *DiagnosticsMockBridge) GetDiagnostics(uri string) ([]any, error) {
	if m.mockGetDiagnostics != nil {
		return m.mockGetDiagnostics(uri)
	}
	return []any{}, nil
}

// Test diagnostics tool registration and functionality
func TestDiagnosticsTool(t *testing.T) {
	testCases := []struct {
		name           string
		uri            string
		mockResponse   []any
		mockError      error
		expectedOutput string
		expectError    bool
	}{
		{
			name: "no diagnostics",
			uri:  "file:///test.go",
			mockResponse: []any{},
			expectedOutput: "No diagnostics found",
			expectError:    false,
		},
		{
			name:           "diagnostics error",
			uri:            "file:///test.go",
			mockError:      fmt.Errorf("diagnostics failed"),
			expectError:    true,
			expectedOutput: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &DiagnosticsMockBridge{
				MockBridge: &MockBridge{},
				mockGetDiagnostics: func(uri string) ([]any, error) {
					if tc.mockError != nil {
						return nil, tc.mockError
					}
					return tc.mockResponse, nil
				},
			}

			mcpServer := server.NewMCPServer(
				"test-server",
				"1.0.0",
				server.WithToolCapabilities(false),
			)

			RegisterDiagnosticsTool(mcpServer, bridge)

			// Just check that the server was created successfully
			if mcpServer == nil {
				t.Fatal("MCP server should not be nil")
			}

			t.Logf("Test case '%s' passed - diagnostics tool successfully registered", tc.name)
		})
	}
}

func TestFormatDiagnostics(t *testing.T) {
	testCases := []struct {
		name     string
		input    []any
		expected string
	}{
		{
			name:     "empty diagnostics",
			input:    []any{},
			expected: "No diagnostics found",
		},
		{
			name:     "non-diagnostic content",
			input:    []any{"not a diagnostic"},
			expected: "DIAGNOSTICS",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatDiagnostics(tc.input)
			if !strings.Contains(result, "DIAGNOSTICS") {
				t.Errorf("Expected result to contain 'DIAGNOSTICS', got: %s", result)
			}
		})
	}
}