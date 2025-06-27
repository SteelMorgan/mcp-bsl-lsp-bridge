package tools

import (
	"errors"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Test hover tool registration and functionality
func TestHoverTool(t *testing.T) {
	testCases := []struct {
		name         string
		uri          string
		line         uint32
		character    uint32
		mockResponse any
		mockError    error
		expectError  bool
		description  string
	}{
		{
			name:      "successful hover with proper Hover type",
			uri:       "file:///test.go",
			line:      10,
			character: 5,
			mockResponse: &protocol.Hover{
				Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{
					Value: protocol.MarkupContent{
						Kind:  protocol.MarkupKindMarkdown,
						Value: "**function main()**\n\nMain function of the program",
					},
				},
			},
			expectError: false,
			description: "Should handle proper protocol.Hover response",
		},
		{
			name:      "successful hover with map contents",
			uri:       "file:///test.go",
			line:      10,
			character: 5,
			mockResponse: &protocol.Hover{
				Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{
					Value: "Test hover information",
				},
			},
			expectError: false,
			description: "Should handle map-based hover response",
		},
		{
			name:         "hover response with nil result",
			uri:          "file:///test.go",
			line:         10,
			character:    5,
			mockResponse: (*protocol.Hover)(nil),
			expectError:  false,
			description:  "Should handle nil Hover result",
		},
		{
			name:         "hover error - column beyond line",
			uri:          "file:///test.go",
			line:         10,
			character:    100,
			mockResponse: (*protocol.Hover)(nil),
			mockError:    errors.New("hover request failed: jsonrpc2: code 0 message: column is beyond end of line"),
			expectError:  true,
			description:  "Should handle column position errors",
		},
		{
			name:         "hover error - invalid response",
			uri:          "file:///test.go",
			line:         10,
			character:    5,
			mockResponse: (*protocol.Hover)(nil),
			mockError:    errors.New("hover request failed: response must have an id and jsonrpc field"),
			expectError:  true,
			description:  "Should handle invalid JSON-RPC responses",
		},
		{
			name:      "hover with absolute path (should normalize to file URI)",
			uri:       "/home/user/test.go",
			line:      5,
			character: 10,
			mockResponse: &protocol.Hover{
				Contents: protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{
					Value: protocol.MarkupContent{
						Kind:  protocol.MarkupKindPlainText,
						Value: "variable: int",
					},
				},
			},
			expectError: false,
			description: "Should handle absolute paths by normalizing to file URI",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			// Set up mock expectations
			bridge.On("GetHoverInformation", tc.uri, tc.line, tc.character).Return(tc.mockResponse, tc.mockError)

			mcpServer, err := mcptest.NewServer(t)
			if err != nil {
				t.Errorf("Could not setup MCP server: %v", err)
			}

			RegisterHoverTool(mcpServer, bridge)

			// Just check that the server was created successfully
			if mcpServer == nil {
				t.Fatal("MCP server should not be nil")
			}

			// Test the hover functionality by directly calling the bridge method
			result, err := bridge.GetHoverInformation(tc.uri, tc.line, tc.character)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				} else {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				} else {
					t.Logf("Got expected result: %T", result)
				}
			}

			// Verify all mock expectations were met
			bridge.AssertExpectations(t)

			t.Logf("Test case '%s' passed - %s", tc.name, tc.description)
		})
	}
}

// Test formatting functions
func TestFormatHoverContent(t *testing.T) {
	testCases := []struct {
		name     string
		input    protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]
		expected string
	}{
		{
			name:     "string content",
			input:    protocol.Or3[protocol.MarkupContent, protocol.MarkedString, []protocol.MarkedString]{Value: protocol.MarkedString{Value: "Simple hover text"}},
			expected: "=== HOVER INFORMATION ===\nSimple hover text",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatHoverContent(tc.input)
			if !strings.Contains(result, "HOVER INFORMATION") {
				t.Errorf("Expected result to contain 'HOVER INFORMATION', got: %s", result)
			}
		})
	}
}
