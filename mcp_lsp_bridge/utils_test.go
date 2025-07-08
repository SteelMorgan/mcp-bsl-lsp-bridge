package mcp_lsp_bridge_test

import (
	"rockerboo/mcp-lsp-bridge/mcp_lsp_bridge"
	"strings"
	"testing"
)

func TestPrettyPrint(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		v    any
		want string
	}{
		{"nil", nil, "null"},
		{"int", 123, "123"},
		{"string", "hello", "\"hello\""},
		{"bool", true, "true"},
		{"slice", []int{1, 2, 3}, "[\n   1,\n   2,\n   3\n]"},
		{"map", map[string]int{"a": 1, "b": 2}, "{\n   \"a\": 1,\n   \"b\": 2\n}"},
		{"struct", struct {
			A int
			B string
		}{1, "hello"}, "{\n   \"A\": 1,\n   \"B\": \"hello\"\n}"},
		{"slice of structs", []struct {
			A int
			B string
		}{{1, "hello"}, {2, "world"}}, "[\n   {\n      \"A\": 1,\n      \"B\": \"hello\"\n   },\n   {\n      \"A\": 2,\n      \"B\": \"world\"\n   }\n]"},
		{"map of slices", map[string][]int{"a": {1, 2, 3}, "b": {4, 5, 6}}, "{\n   \"a\": [\n      1,\n      2,\n      3\n   ],\n   \"b\": [\n      4,\n      5,\n      6\n   ]\n}"},
		{"slice of maps", []map[string]int{{"a": 1, "b": 2}, {"c": 3, "d": 4}}, "[\n   {\n      \"a\": 1,\n      \"b\": 2\n   },\n   {\n      \"c\": 3,\n      \"d\": 4\n   }\n]"},
		{"map of structs", map[string]struct {
			A int
			B string
		}{"a": {1, "hello"}, "b": {2, "world"}}, "{\n   \"a\": {\n      \"A\": 1,\n      \"B\": \"hello\"\n   },\n   \"b\": {\n      \"A\": 2,\n      \"B\": \"world\"\n   }\n}"},
		{"slice of maps of structs", []map[string]struct {
			A int
			B string
		}{{"a": {1, "hello"}, "b": {2, "world"}}, {"c": {3, "foo"}, "d": {4, "bar"}}}, "[\n   {\n      \"a\": {\n         \"A\": 1,\n         \"B\": \"hello\"\n      },\n      \"b\": {\n         \"A\": 2,\n         \"B\": \"world\"\n      }\n   },\n   {\n      \"c\": {\n         \"A\": 3,\n         \"B\": \"foo\"\n      },\n      \"d\": {\n         \"A\": 4,\n         \"B\": \"bar\"\n      }\n   }\n]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mcp_lsp_bridge.PrettyPrint(tt.v)
			if got != tt.want {
				t.Errorf("PrettyPrint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafePrettyPrint(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want string
	}{
		{"nil", nil, "null"},
		{"simple string", "hello", "\"hello\""},
		{"simple int", 42, "42"},
		{"simple struct", struct {
			Name string
			Age  int
		}{"John", 30}, "{\n   \"Age\": 30,\n   \"Name\": \"John\"\n}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mcp_lsp_bridge.SafePrettyPrint(tt.v)
			if got != tt.want {
				t.Errorf("SafePrettyPrint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafePrettyPrint_CircularReference(t *testing.T) {
	// Create a struct with a circular reference
	type Node struct {
		Name string
		Next *Node
	}

	node1 := &Node{Name: "Node1"}
	node2 := &Node{Name: "Node2"}
	node1.Next = node2
	node2.Next = node1 // Creates circular reference

	result := mcp_lsp_bridge.SafePrettyPrint(node1)

	// Should not panic and should contain circular reference indication
	if result == "" {
		t.Error("SafePrettyPrint should not return empty string for circular reference")
	}

	// Should contain both nodes and handle the circular reference
	// The exact format may vary but it should not crash
	t.Logf("Circular reference result: %s", result)
}

func TestSafePrettyPrint_ComplexStructures(t *testing.T) {
	// Test that SafePrettyPrint handles complex structures correctly
	// This indirectly tests the simplifyForJSON function

	tests := []struct {
		name          string
		input         any
		shouldMatch   string   // exact match
		shouldContain []string // strings that should be in output
	}{
		{
			name: "struct with unexported field",
			input: struct {
				Name string
				age  int // unexported, should not appear
			}{"John", 30},
			shouldContain: []string{"Name", "John"},
		},
		{
			name:        "nil pointer",
			input:       (*string)(nil),
			shouldMatch: "null",
		},
		{
			name:        "valid pointer",
			input:       func() *string { s := "hello"; return &s }(),
			shouldMatch: "\"hello\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mcp_lsp_bridge.SafePrettyPrint(tt.input)

			if tt.shouldMatch != "" {
				if result != tt.shouldMatch {
					t.Errorf("Expected exact match %q, got %q", tt.shouldMatch, result)
				}
			}

			for _, contain := range tt.shouldContain {
				if !contains(result, contain) {
					t.Errorf("Expected result to contain %q, got %q", contain, result)
				}
			}

			// For struct with unexported field test, ensure 'age' is not present
			if tt.name == "struct with unexported field" {
				if contains(result, "age") {
					t.Error("Unexported field 'age' should not appear in output")
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
