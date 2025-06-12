package mcp_lsp_bridge

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func PrettyPrint(v any) string {
	if data, err := json.MarshalIndent(v, "", "   "); err != nil {
		return fmt.Sprintf("%+v", v) // fallback to default formatting
	} else {
		return string(data)
	}
}

func SafePrettyPrint(v any) string {
	// Create a copy without circular references
	simplified := simplifyForJSON(v, make(map[uintptr]bool))
	if data, err := json.MarshalIndent(simplified, "", "   "); err != nil {
		return fmt.Sprintf("%+v", v)
	} else {
		return string(data)
	}
}

func simplifyForJSON(v any, visited map[uintptr]bool) any {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)

	// Handle pointers and check for cycles
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		ptr := val.Pointer()
		if visited[ptr] {
			return "<circular reference>"
		}
		visited[ptr] = true
		defer delete(visited, ptr)
		return simplifyForJSON(val.Elem().Interface(), visited)
	}

	// For structs, process each field
	if val.Kind() == reflect.Struct {
		result := make(map[string]any)
		typ := val.Type()
		for i := range val.NumField() {
			field := typ.Field(i)
			if field.IsExported() {
				fieldVal := val.Field(i)
				if fieldVal.CanInterface() {
					result[field.Name] = simplifyForJSON(fieldVal.Interface(), visited)
				}
			}
		}
		return result
	}

	return v
}
