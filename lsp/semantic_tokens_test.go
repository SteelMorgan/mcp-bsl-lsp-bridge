package lsp

import (
	"testing"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSemanticTokenParser(t *testing.T) {
	tokenTypes := []string{"function", "variable", "type"}
	tokenModifiers := []string{"declaration", "definition", "readonly"}

	parser := NewSemanticTokenParser(tokenTypes, tokenModifiers)

	require.NotNil(t, parser)
	assert.Equal(t, tokenTypes, parser.TokenTypes())
	assert.Equal(t, tokenModifiers, parser.TokenModifiers())
}

func TestNewSemanticTokenParser_EmptySlices(t *testing.T) {
	parser := NewSemanticTokenParser([]string{}, []string{})

	require.NotNil(t, parser)
	assert.Empty(t, parser.TokenTypes())
	assert.Empty(t, parser.TokenModifiers())
}

func TestNewSemanticTokenParser_NilSlices(t *testing.T) {
	parser := NewSemanticTokenParser(nil, nil)

	require.NotNil(t, parser)
	assert.Nil(t, parser.TokenTypes())
	assert.Nil(t, parser.TokenModifiers())
}

func TestFindTokensByType_BasicFunctionality(t *testing.T) {
	tokenTypes := []string{"function", "variable", "type", "parameter"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	// Create test semantic tokens data
	// Format: [deltaLine, deltaStart, length, tokenTypeIndex, tokenModifiers]
	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{
			// First token: function at line 0, char 0, length 5
			0, 0, 5, 0, 0, // "function" type (index 0)
			// Second token: variable at line 0, char 10, length 3
			0, 5, 3, 1, 0, // "variable" type (index 1)
			// Third token: function at line 1, char 0, length 4
			1, 0, 4, 0, 0, // "function" type (index 0)
		},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	// Test finding functions only
	results, err := parser.FindTokensByType(semanticTokens, []string{"function"}, baseRange)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Check first function token
	assert.Equal(t, "function", results[0].TokenType)
	assert.Equal(t, uint32(0), results[0].Range.Start.Line)
	assert.Equal(t, uint32(0), results[0].Range.Start.Character)
	assert.Equal(t, uint32(5), results[0].Range.End.Character)

	// Check second function token
	assert.Equal(t, "function", results[1].TokenType)
	assert.Equal(t, uint32(1), results[1].Range.Start.Line)
	assert.Equal(t, uint32(0), results[1].Range.Start.Character)
	assert.Equal(t, uint32(4), results[1].Range.End.Character)
}

func TestFindTokensByType_MultipleTypes(t *testing.T) {
	tokenTypes := []string{"function", "variable", "type", "parameter"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{
			0, 0, 5, 0, 0, // function
			0, 6, 3, 1, 0, // variable
			0, 4, 4, 2, 0, // type
		},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	// Test finding both functions and variables
	results, err := parser.FindTokensByType(semanticTokens, []string{"function", "variable"}, baseRange)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	assert.Equal(t, "function", results[0].TokenType)
	assert.Equal(t, "variable", results[1].TokenType)
}

func TestFindTokensByType_NoMatches(t *testing.T) {
	tokenTypes := []string{"function", "variable", "type"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{
			0, 0, 5, 0, 0, // function
			0, 6, 3, 1, 0, // variable
		},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	// Test finding types that don't exist in the data
	results, err := parser.FindTokensByType(semanticTokens, []string{"class", "interface"}, baseRange)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestFindTokensByType_InvalidTokenIndex(t *testing.T) {
	tokenTypes := []string{"function", "variable"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{
			0, 0, 5, 0, 0, // valid function
			0, 6, 3, 5, 0, // invalid token type index (5 >= len(tokenTypes))
			0, 4, 4, 1, 0, // valid variable
		},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	// Should skip invalid tokens and return valid ones
	results, err := parser.FindTokensByType(semanticTokens, []string{"function", "variable"}, baseRange)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "function", results[0].TokenType)
	assert.Equal(t, "variable", results[1].TokenType)
}

func TestFindTokensByType_EmptyData(t *testing.T) {
	tokenTypes := []string{"function", "variable"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	results, err := parser.FindTokensByType(semanticTokens, []string{"function"}, baseRange)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestFindTokensByType_ComplexDeltas(t *testing.T) {
	tokenTypes := []string{"function", "variable", "type"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	// Test complex delta calculations across multiple lines
	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{
			// Token at line 0, character 5
			0, 5, 8, 0, 0, // function
			// Token at line 2 (deltaLine=2), character 10
			2, 10, 4, 1, 0, // variable
			// Token at same line (deltaLine=0), character 20 (10 + 10)
			0, 10, 6, 2, 0, // type
			// Token at line 5 (2 + 3), character 0
			3, 0, 5, 0, 0, // function
		},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	results, err := parser.FindTokensByType(semanticTokens, []string{"function", "variable", "type"}, baseRange)
	require.NoError(t, err)
	assert.Len(t, results, 4)

	// Check positions are calculated correctly
	assert.Equal(t, uint32(0), results[0].Range.Start.Line)
	assert.Equal(t, uint32(5), results[0].Range.Start.Character)

	assert.Equal(t, uint32(2), results[1].Range.Start.Line)
	assert.Equal(t, uint32(10), results[1].Range.Start.Character)

	assert.Equal(t, uint32(2), results[2].Range.Start.Line)
	assert.Equal(t, uint32(20), results[2].Range.Start.Character)

	assert.Equal(t, uint32(5), results[3].Range.Start.Line)
	assert.Equal(t, uint32(0), results[3].Range.Start.Character)
}

func TestFindFunctionNames(t *testing.T) {
	tokenTypes := []string{"function", "method", "variable"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{
			0, 0, 5, 0, 0, // function
			0, 6, 6, 1, 0, // method
			0, 7, 3, 2, 0, // variable (should not be included)
		},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	results, err := parser.FindFunctionNames(semanticTokens, baseRange)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "function", results[0].TokenType)
	assert.Equal(t, "method", results[1].TokenType)
}

func TestFindParameters(t *testing.T) {
	tokenTypes := []string{"parameter", "variable", "function"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{
			0, 0, 5, 0, 0, // parameter
			0, 6, 3, 1, 0, // variable (should not be included)
			0, 4, 4, 2, 0, // function (should not be included)
		},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	results, err := parser.FindParameters(semanticTokens, baseRange)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "parameter", results[0].TokenType)
}

func TestFindVariables(t *testing.T) {
	tokenTypes := []string{"variable", "parameter", "function"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{
			0, 0, 5, 0, 0, // variable
			0, 6, 3, 1, 0, // parameter (should not be included)
			0, 4, 4, 2, 0, // function (should not be included)
		},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	results, err := parser.FindVariables(semanticTokens, baseRange)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "variable", results[0].TokenType)
}

func TestFindTypes(t *testing.T) {
	tokenTypes := []string{"type", "class", "interface", "struct", "variable"}
	parser := NewSemanticTokenParser(tokenTypes, []string{})

	semanticTokens := &protocol.SemanticTokens{
		Data: []uint32{
			0, 0, 5, 0, 0, // type
			0, 6, 5, 1, 0, // class
			0, 6, 9, 2, 0, // interface
			0, 10, 6, 3, 0, // struct
			0, 7, 3, 4, 0, // variable (should not be included)
		},
	}

	baseRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 10, Character: 0},
	}

	results, err := parser.FindTypes(semanticTokens, baseRange)
	require.NoError(t, err)
	assert.Len(t, results, 4)
	assert.Equal(t, "type", results[0].TokenType)
	assert.Equal(t, "class", results[1].TokenType)
	assert.Equal(t, "interface", results[2].TokenType)
	assert.Equal(t, "struct", results[3].TokenType)
}

func TestGetTokenTypeFromServerCapabilities_SemanticTokensOptions(t *testing.T) {
	tokenTypes := []string{"function", "variable", "type"}
	tokenModifiers := []string{"declaration", "definition"}

	capabilities := &protocol.ServerCapabilities{
		SemanticTokensProvider: &protocol.Or2[protocol.SemanticTokensOptions, protocol.SemanticTokensRegistrationOptions]{
			Value: protocol.SemanticTokensOptions{
				Legend: protocol.SemanticTokensLegend{
					TokenTypes:     tokenTypes,
					TokenModifiers: tokenModifiers,
				},
			},
		},
	}

	resultTypes, resultModifiers, err := GetTokenTypeFromServerCapabilities(capabilities)
	require.NoError(t, err)
	assert.Equal(t, tokenTypes, resultTypes)
	assert.Equal(t, tokenModifiers, resultModifiers)
}

func TestGetTokenTypeFromServerCapabilities_SemanticTokensRegistrationOptions(t *testing.T) {
	tokenTypes := []string{"function", "variable", "type"}
	tokenModifiers := []string{"declaration", "definition"}

	capabilities := &protocol.ServerCapabilities{
		SemanticTokensProvider: &protocol.Or2[protocol.SemanticTokensOptions, protocol.SemanticTokensRegistrationOptions]{
			Value: protocol.SemanticTokensRegistrationOptions{
				Legend: protocol.SemanticTokensLegend{
					TokenTypes:     tokenTypes,
					TokenModifiers: tokenModifiers,
				},
			},
		},
	}

	resultTypes, resultModifiers, err := GetTokenTypeFromServerCapabilities(capabilities)
	require.NoError(t, err)
	assert.Equal(t, tokenTypes, resultTypes)
	assert.Equal(t, tokenModifiers, resultModifiers)
}

func TestGetTokenTypeFromServerCapabilities_NoSemanticTokens(t *testing.T) {
	capabilities := &protocol.ServerCapabilities{
		SemanticTokensProvider: nil,
	}

	resultTypes, resultModifiers, err := GetTokenTypeFromServerCapabilities(capabilities)
	require.Error(t, err)
	assert.Equal(t, "server does not support semantic tokens", err.Error())
	assert.Nil(t, resultTypes)
	assert.Nil(t, resultModifiers)
}

func TestGetTokenTypeFromServerCapabilities_EmptyTokenTypes(t *testing.T) {
	capabilities := &protocol.ServerCapabilities{
		SemanticTokensProvider: &protocol.Or2[protocol.SemanticTokensOptions, protocol.SemanticTokensRegistrationOptions]{
			Value: protocol.SemanticTokensOptions{
				Legend: protocol.SemanticTokensLegend{
					TokenTypes:     []string{}, // Empty token types
					TokenModifiers: []string{"declaration"},
				},
			},
		},
	}

	resultTypes, resultModifiers, err := GetTokenTypeFromServerCapabilities(capabilities)
	require.Error(t, err)
	assert.Equal(t, "no token types provided by server", err.Error())
	assert.Nil(t, resultTypes)
	assert.Nil(t, resultModifiers)
}

func TestGetTokenTypeFromServerCapabilities_UnsupportedType(t *testing.T) {
	capabilities := &protocol.ServerCapabilities{
		SemanticTokensProvider: &protocol.Or2[protocol.SemanticTokensOptions, protocol.SemanticTokensRegistrationOptions]{
			Value: "unsupported_type", // This would cause a type assertion failure
		},
	}

	resultTypes, resultModifiers, err := GetTokenTypeFromServerCapabilities(capabilities)
	require.Error(t, err)
	assert.Equal(t, "unsupported semantic tokens provider type", err.Error())
	assert.Nil(t, resultTypes)
	assert.Nil(t, resultModifiers)
}
