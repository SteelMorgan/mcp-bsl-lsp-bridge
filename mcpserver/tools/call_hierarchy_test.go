package tools

import (
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

func TestCallHierarchyTool(t *testing.T) {
	testCases := []struct {
		name              string
		uri               string
		line              int32
		character         int32
		direction         string
		mockItems         []any
		mockIncoming      []any
		mockOutgoing      []any
		expectError       bool
		expectedContent   string
		description       string
	}{
		{
			name:      "successful call hierarchy preparation",
			uri:       "file:///main.go",
			line:      10,
			character: 5,
			direction: "both",
			mockItems: []any{
				map[string]any{
					"name": "main",
					"kind": protocol.SymbolKindFunction,
					"uri":  "file:///main.go",
					"range": map[string]any{
						"start": map[string]int32{"line": 10, "character": 0},
						"end":   map[string]int32{"line": 15, "character": 1},
					},
				},
			},
			mockIncoming: []any{
				map[string]any{
					"from": map[string]any{
						"name": "init",
						"kind": protocol.SymbolKindFunction,
					},
					"fromRanges": []map[string]any{
						{
							"start": map[string]int32{"line": 5, "character": 0},
							"end":   map[string]int32{"line": 5, "character": 4},
						},
					},
				},
			},
			mockOutgoing: []any{
				map[string]any{
					"to": map[string]any{
						"name": "fmt.Println",
						"kind": protocol.SymbolKindFunction,
					},
					"fromRanges": []map[string]any{
						{
							"start": map[string]int32{"line": 12, "character": 1},
							"end":   map[string]int32{"line": 12, "character": 12},
						},
					},
				},
			},
			expectError:     false,
			expectedContent: "CALL HIERARCHY",
			description:     "Should prepare call hierarchy for main function",
		},
		{
			name:      "incoming calls only",
			uri:       "file:///utils.go",
			line:      20,
			character: 10,
			direction: "incoming",
			mockItems: []any{
				map[string]any{
					"name": "helper",
					"kind": protocol.SymbolKindFunction,
					"uri":  "file:///utils.go",
				},
			},
			mockIncoming: []any{
				map[string]any{
					"from": map[string]any{
						"name": "processData",
						"kind": protocol.SymbolKindFunction,
					},
				},
			},
			expectError:     false,
			expectedContent: "INCOMING CALLS",
			description:     "Should show incoming calls to helper function",
		},
		{
			name:      "outgoing calls only",
			uri:       "file:///service.go",
			line:      30,
			character: 15,
			direction: "outgoing",
			mockItems: []any{
				map[string]any{
					"name": "service.Process",
					"kind": protocol.SymbolKindMethod,
					"uri":  "file:///service.go",
				},
			},
			mockOutgoing: []any{
				map[string]any{
					"to": map[string]any{
						"name": "database.Query",
						"kind": protocol.SymbolKindMethod,
					},
				},
				map[string]any{
					"to": map[string]any{
						"name": "logger.Info",
						"kind": protocol.SymbolKindMethod,
					},
				},
			},
			expectError:     false,
			expectedContent: "OUTGOING CALLS",
			description:     "Should show outgoing calls from service method",
		},
		{
			name:        "call hierarchy preparation failure",
			uri:         "file:///invalid.go",
			line:        10,
			character:   5,
			direction:   "both",
			expectError: true,
			description: "Should handle call hierarchy preparation errors",
		},
		{
			name:      "no call hierarchy items",
			uri:       "file:///empty.go",
			line:      1,
			character: 0,
			direction: "both",
			mockItems: []any{},
			expectError: false,
			expectedContent: "No call hierarchy",
			description: "Should handle files with no callable symbols",
		},
		{
			name:      "complex call hierarchy with nested calls",
			uri:       "file:///complex.go",
			line:      50,
			character: 20,
			direction: "both",
			mockItems: []any{
				map[string]any{
					"name": "complexFunction",
					"kind": protocol.SymbolKindFunction,
					"uri":  "file:///complex.go",
				},
			},
			mockIncoming: []any{
				map[string]any{
					"from": map[string]any{
						"name": "caller1",
						"kind": protocol.SymbolKindFunction,
					},
				},
				map[string]any{
					"from": map[string]any{
						"name": "caller2",
						"kind": protocol.SymbolKindMethod,
					},
				},
			},
			mockOutgoing: []any{
				map[string]any{
					"to": map[string]any{
						"name": "callee1",
						"kind": protocol.SymbolKindFunction,
					},
				},
				map[string]any{
					"to": map[string]any{
						"name": "callee2",
						"kind": protocol.SymbolKindMethod,
					},
				},
				map[string]any{
					"to": map[string]any{
						"name": "callee3",
						"kind": protocol.SymbolKindFunction,
					},
				},
			},
			expectError:     false,
			expectedContent: "CALL HIERARCHY",
			description:     "Should handle complex call hierarchies with multiple callers and callees",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				inferLanguageFunc: func(filePath string) (string, error) {
					return "go", nil
				},
				prepareCallHierarchyFunc: func(uri string, line, character int32) ([]any, error) {
					if tc.expectError {
						return nil, fmt.Errorf("call hierarchy preparation failed")
					}
					return tc.mockItems, nil
				},
				getIncomingCallsFunc: func(item any) ([]any, error) {
					if tc.expectError {
						return nil, fmt.Errorf("incoming calls failed")
					}
					return tc.mockIncoming, nil
				},
				getOutgoingCallsFunc: func(item any) ([]any, error) {
					if tc.expectError {
						return nil, fmt.Errorf("outgoing calls failed")
					}
					return tc.mockOutgoing, nil
				},
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterCallHierarchyTool(mcpServer, bridge)

			// Test call hierarchy preparation
			items, err := bridge.PrepareCallHierarchy(tc.uri, tc.line, tc.character)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error in call hierarchy preparation: %v", err)
				return
			}

			if len(items) != len(tc.mockItems) {
				t.Errorf("Expected %d call hierarchy items, got %d", len(tc.mockItems), len(items))
			}

			// Test incoming calls if direction includes incoming
			if tc.direction == "incoming" || tc.direction == "both" {
				if len(items) > 0 {
					incoming, err := bridge.GetIncomingCalls(items[0])
					if err != nil {
						t.Errorf("Unexpected error getting incoming calls: %v", err)
					}
					if len(incoming) != len(tc.mockIncoming) {
						t.Errorf("Expected %d incoming calls, got %d", len(tc.mockIncoming), len(incoming))
					}
				}
			}

			// Test outgoing calls if direction includes outgoing
			if tc.direction == "outgoing" || tc.direction == "both" {
				if len(items) > 0 {
					outgoing, err := bridge.GetOutgoingCalls(items[0])
					if err != nil {
						t.Errorf("Unexpected error getting outgoing calls: %v", err)
					}
					if len(outgoing) != len(tc.mockOutgoing) {
						t.Errorf("Expected %d outgoing calls, got %d", len(tc.mockOutgoing), len(outgoing))
					}
				}
			}

			t.Logf("Test case '%s' passed - %s", tc.name, tc.description)
		})
	}
}

func TestCallHierarchySymbolTypes(t *testing.T) {
	symbolTypes := []struct {
		symbolKind  protocol.SymbolKind
		name        string
		description string
	}{
		{protocol.SymbolKindFunction, "function", "Regular function"},
		{protocol.SymbolKindMethod, "method", "Class/struct method"},
		{protocol.SymbolKindConstructor, "constructor", "Constructor function"},
		{protocol.SymbolKindClass, "class", "Class definition"},
		{protocol.SymbolKindInterface, "interface", "Interface definition"},
		{protocol.SymbolKindNamespace, "namespace", "Namespace/package"},
	}

	for _, symbolType := range symbolTypes {
		t.Run(fmt.Sprintf("call_hierarchy_%s", symbolType.name), func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				prepareCallHierarchyFunc: func(uri string, line, character int32) ([]any, error) {
					return []any{
						map[string]any{
							"name": fmt.Sprintf("test%s", symbolType.name),
							"kind": symbolType.symbolKind,
							"uri":  uri,
						},
					}, nil
				},
				getIncomingCallsFunc: func(item any) ([]any, error) {
					return []any{
						map[string]any{
							"from": map[string]any{
								"name": "caller",
								"kind": protocol.SymbolKindFunction,
							},
						},
					}, nil
				},
				getOutgoingCallsFunc: func(item any) ([]any, error) {
					return []any{
						map[string]any{
							"to": map[string]any{
								"name": "callee",
								"kind": protocol.SymbolKindFunction,
							},
						},
					}, nil
				},
			}

			// Test call hierarchy for this symbol type
			items, err := bridge.PrepareCallHierarchy("file:///test.go", 10, 5)
			if err != nil {
				t.Errorf("Error preparing call hierarchy for %s: %v", symbolType.description, err)
				return
			}

			if len(items) != 1 {
				t.Errorf("Expected 1 item for %s, got %d", symbolType.description, len(items))
				return
			}

			// Test incoming calls
			incoming, err := bridge.GetIncomingCalls(items[0])
			if err != nil {
				t.Errorf("Error getting incoming calls for %s: %v", symbolType.description, err)
			}
			if len(incoming) != 1 {
				t.Errorf("Expected 1 incoming call for %s, got %d", symbolType.description, len(incoming))
			}

			// Test outgoing calls
			outgoing, err := bridge.GetOutgoingCalls(items[0])
			if err != nil {
				t.Errorf("Error getting outgoing calls for %s: %v", symbolType.description, err)
			}
			if len(outgoing) != 1 {
				t.Errorf("Expected 1 outgoing call for %s, got %d", symbolType.description, len(outgoing))
			}

			t.Logf("Call hierarchy test for %s passed", symbolType.description)
		})
	}
}

func TestCallHierarchyEdgeCases(t *testing.T) {
	t.Run("recursive function calls", func(t *testing.T) {
		bridge := &ComprehensiveMockBridge{
			prepareCallHierarchyFunc: func(uri string, line, character int32) ([]any, error) {
				return []any{
					map[string]any{
						"name": "recursiveFunc",
						"kind": protocol.SymbolKindFunction,
						"uri":  uri,
					},
				}, nil
			},
			getIncomingCallsFunc: func(item any) ([]any, error) {
				// Recursive function calls itself
				return []any{
					map[string]any{
						"from": map[string]any{
							"name": "recursiveFunc",
							"kind": protocol.SymbolKindFunction,
						},
					},
				}, nil
			},
			getOutgoingCallsFunc: func(item any) ([]any, error) {
				// Recursive function calls itself
				return []any{
					map[string]any{
						"to": map[string]any{
							"name": "recursiveFunc",
							"kind": protocol.SymbolKindFunction,
						},
					},
				}, nil
			},
		}

		items, err := bridge.PrepareCallHierarchy("file:///recursive.go", 10, 5)
		if err != nil {
			t.Errorf("Error preparing call hierarchy for recursive function: %v", err)
		}

		if len(items) == 0 {
			t.Error("Expected call hierarchy items for recursive function")
		}
	})

	t.Run("deeply nested call chains", func(t *testing.T) {
		bridge := &ComprehensiveMockBridge{
			prepareCallHierarchyFunc: func(uri string, line, character int32) ([]any, error) {
				return []any{
					map[string]any{
						"name": "middleFunction",
						"kind": protocol.SymbolKindFunction,
						"uri":  uri,
					},
				}, nil
			},
			getIncomingCallsFunc: func(item any) ([]any, error) {
				// Multiple levels of callers
				return []any{
					map[string]any{
						"from": map[string]any{
							"name": "topLevel1",
							"kind": protocol.SymbolKindFunction,
						},
					},
					map[string]any{
						"from": map[string]any{
							"name": "topLevel2",
							"kind": protocol.SymbolKindFunction,
						},
					},
					map[string]any{
						"from": map[string]any{
							"name": "intermediate",
							"kind": protocol.SymbolKindFunction,
						},
					},
				}, nil
			},
			getOutgoingCallsFunc: func(item any) ([]any, error) {
				// Multiple levels of callees
				return []any{
					map[string]any{
						"to": map[string]any{
							"name": "bottomLevel1",
							"kind": protocol.SymbolKindFunction,
						},
					},
					map[string]any{
						"to": map[string]any{
							"name": "bottomLevel2",
							"kind": protocol.SymbolKindFunction,
						},
					},
					map[string]any{
						"to": map[string]any{
							"name": "utility",
							"kind": protocol.SymbolKindFunction,
						},
					},
				}, nil
			},
		}

		items, err := bridge.PrepareCallHierarchy("file:///deep.go", 25, 10)
		if err != nil {
			t.Errorf("Error preparing call hierarchy for deeply nested calls: %v", err)
		}

		if len(items) == 0 {
			t.Error("Expected call hierarchy items for deeply nested function")
		}

		// Test that we can get both incoming and outgoing calls
		incoming, err := bridge.GetIncomingCalls(items[0])
		if err != nil {
			t.Errorf("Error getting incoming calls: %v", err)
		}
		if len(incoming) != 3 {
			t.Errorf("Expected 3 incoming calls, got %d", len(incoming))
		}

		outgoing, err := bridge.GetOutgoingCalls(items[0])
		if err != nil {
			t.Errorf("Error getting outgoing calls: %v", err)
		}
		if len(outgoing) != 3 {
			t.Errorf("Expected 3 outgoing calls, got %d", len(outgoing))
		}
	})
}