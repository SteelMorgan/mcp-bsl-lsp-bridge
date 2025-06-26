package lsp

import (
	"fmt"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Standard LSP semantic token types (you should get these from server capabilities)
// tokenTypes := []string{
// 	"namespace", "type", "class", "enum", "interface", "struct",
// 	"typeParameter", "parameter", "variable", "property", "enumMember",
// 	"event", "function", "method", "macro", "keyword", "modifier",
// 	"comment", "string", "number", "regexp", "operator",
// }
//
// tokenModifiers := []string{
// 	"declaration", "definition", "readonly", "static", "deprecated",
// 	"abstract", "async", "modification", "documentation", "defaultLibrary",
// }

// TokenPosition represents a found token with its position and type
type TokenPosition struct {
	TokenType string
	Text      string // The actual text if available
	Range     protocol.Range
}

// SemanticTokenParser handles parsing of semantic tokens
type SemanticTokenParser struct {
	tokenTypes     []string
	tokenModifiers []string
}

// NewSemanticTokenParser creates a new parser with the given token types and modifiers
func NewSemanticTokenParser(tokenTypes, tokenModifiers []string) *SemanticTokenParser {
	return &SemanticTokenParser{
		tokenTypes:     tokenTypes,
		tokenModifiers: tokenModifiers,
	}
}

// FindTokensByType finds all tokens of specified types from a semantic tokens response
func (p *SemanticTokenParser) FindTokensByType(
	tokens *protocol.SemanticTokens,
	targetTypes []string,
	baseRange protocol.Range,
) ([]TokenPosition, error) {
	var results []TokenPosition
	targetTypeSet := make(map[string]bool)
	for _, t := range targetTypes {
		targetTypeSet[t] = true
	}

	currentLine := baseRange.Start.Line
	currentChar := baseRange.Start.Character

	data := tokens.Data

	for i := 0; i < len(data); i += 5 {
		deltaLine := data[i]
		deltaStart := data[i+1]
		tokenLength := data[i+2]
		tokenTypeIndex := data[i+3]
		// tokenModifiers := int(data[i+4]) // Not used in this example

		// Update position based on deltas
		if deltaLine > 0 {
			currentLine += deltaLine
			currentChar = deltaStart
		} else {
			currentChar += deltaStart
		}

		// Get token type name
		if tokenTypeIndex >= uint32(len(p.tokenTypes)) {
			continue // Skip invalid token types
		}
		tokenType := p.tokenTypes[tokenTypeIndex]

		// Check if this is a token type we're looking for
		if targetTypeSet[tokenType] {
			tokenRange := protocol.Range{
				Start: protocol.Position{
					Line:      currentLine,
					Character: currentChar,
				},
				End: protocol.Position{
					Line:      currentLine,
					Character: currentChar + tokenLength,
				},
			}

			results = append(results, TokenPosition{
				TokenType: tokenType,
				Range:     tokenRange,
			})
		}
	}

	return results, nil
}

// FindFunctionNames finds all function/method names in the semantic tokens
func (p *SemanticTokenParser) FindFunctionNames(
	tokens *protocol.SemanticTokens,
	baseRange protocol.Range,
) ([]TokenPosition, error) {
	return p.FindTokensByType(tokens, []string{"function", "method"}, baseRange)
}

// FindParameters finds all parameters in the semantic tokens
func (p *SemanticTokenParser) FindParameters(
	tokens *protocol.SemanticTokens,
	baseRange protocol.Range,
) ([]TokenPosition, error) {
	return p.FindTokensByType(tokens, []string{"parameter"}, baseRange)
}

// FindVariables finds all variables in the semantic tokens
func (p *SemanticTokenParser) FindVariables(
	tokens *protocol.SemanticTokens,
	baseRange protocol.Range,
) ([]TokenPosition, error) {
	return p.FindTokensByType(tokens, []string{"variable"}, baseRange)
}

// FindTypes finds all type references in the semantic tokens
func (p *SemanticTokenParser) FindTypes(
	tokens *protocol.SemanticTokens,
	baseRange protocol.Range,
) ([]TokenPosition, error) {
	return p.FindTokensByType(tokens, []string{"type", "class", "interface", "struct"}, baseRange)
}

// GetTokenTypeFromServerCapabilities extracts token types from server capabilities
func GetTokenTypeFromServerCapabilities(capabilities *protocol.ServerCapabilities) ([]string, []string, error) {
	if capabilities.SemanticTokensProvider == nil {
		return nil, nil, fmt.Errorf("server does not support semantic tokens")
	}

	var tokenTypes, tokenModifiers []string

	// Handle different ways the server might provide semantic token info
	switch provider := capabilities.SemanticTokensProvider.Value.(type) {
	case *protocol.SemanticTokensOptions:
	case *protocol.SemanticTokensRegistrationOptions:
		tokenTypes = provider.Legend.TokenTypes
		tokenModifiers = provider.Legend.TokenModifiers
	default:
		return nil, nil, fmt.Errorf("unsupported semantic tokens provider type")
	}

	if len(tokenTypes) == 0 {
		return nil, nil, fmt.Errorf("no token types provided by server")
	}

	return tokenTypes, tokenModifiers, nil
}
