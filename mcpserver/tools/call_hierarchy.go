package tools

import (
	"context"
	"fmt"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/utils"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// RegisterCallHierarchyTool registers the call hierarchy tool
func RegisterCallHierarchyTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(CallHierarchyTool(bridge))
}

func CallHierarchyTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("call_hierarchy",
			mcp.WithDescription("Show call hierarchy (callers and callees) for a symbol. INVALUABLE for understanding code flow - reveals function relationships that static analysis alone cannot provide."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file")),
			mcp.WithNumber("line", mcp.Description("Line number (0-based)")),
			mcp.WithNumber("character", mcp.Description("Character position (0-based)")),
			mcp.WithString("direction", mcp.Description("Direction: 'incoming', 'outgoing', or 'both' (default: 'both')")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse and validate parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("call_hierarchy: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				logger.Error("call_hierarchy: Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				logger.Error("call_hierarchy: Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Determine call hierarchy direction
			direction := "both" // Default to both directions
			if dirParam, err := request.RequireString("direction"); err == nil {
				// Validate direction parameter
				switch strings.ToLower(dirParam) {
				case "incoming", "outgoing", "both":
					direction = strings.ToLower(dirParam)
				default:
					logger.Error("call_hierarchy: Invalid direction parameter", fmt.Errorf("invalid direction: %s", dirParam))
					return mcp.NewToolResultError("Invalid direction. Must be 'incoming', 'outgoing', or 'both', got: " + dirParam), nil
				}
			}

			// Validate parameters
			lineUint32, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			characterUint32, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}

			// Normalize URI to ensure proper file:// scheme
			normalizedURI := utils.NormalizeURI(uri)

			// Infer language from the specific file URI
			language, err := bridge.InferLanguage(normalizedURI)
			if err != nil {
				logger.Error("call_hierarchy: language inference failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to infer language from URI: %v", err)), nil
			}

			// Use bridge to prepare call hierarchy (consistent with IncomingCalls/OutgoingCalls)
			allPrepItems, err := bridge.PrepareCallHierarchy(normalizedURI, lineUint32, characterUint32)
			if err != nil {
				logger.Error("call_hierarchy: prepare call hierarchy failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to prepare call hierarchy: %v", err)), nil
			}

			// Set successful language for output formatting
			successfulLanguage := string(*language)
			var prepErrors []error

			if len(allPrepItems) == 0 {
				var errorMsg strings.Builder
				fmt.Fprintf(&errorMsg, "No call hierarchy items found on line %d, character %d for language %s\n", line, character, *language)
				if len(prepErrors) > 0 {
					fmt.Fprintf(&errorMsg, "\nError: %v\n", prepErrors[0])
				}
				return mcp.NewToolResultText(errorMsg.String()), nil
			}

			// Collect call details based on direction
			var incomingCalls []protocol.CallHierarchyIncomingCall
			var outgoingCalls []protocol.CallHierarchyOutgoingCall
			var callErrors []error

			for _, item := range allPrepItems {
				switch direction {
				case "incoming":
					calls, err := bridge.IncomingCalls(item)
					if err != nil {
						callErrors = append(callErrors, fmt.Errorf("incoming calls for %s: %w", item.Name, err))
					} else {
						incomingCalls = append(incomingCalls, calls...)
					}
				case "outgoing":
					calls, err := bridge.OutgoingCalls(item)
					if err != nil {
						callErrors = append(callErrors, fmt.Errorf("outgoing calls for %s: %w", item.Name, err))
					} else {
						outgoingCalls = append(outgoingCalls, calls...)
					}
				case "both":
					inCalls, inErr := bridge.IncomingCalls(item)
					outCalls, outErr := bridge.OutgoingCalls(item)

					if inErr != nil {
						callErrors = append(callErrors, fmt.Errorf("incoming calls for %s: %w", item.Name, inErr))
					} else {
						incomingCalls = append(incomingCalls, inCalls...)
					}

					if outErr != nil {
						callErrors = append(callErrors, fmt.Errorf("outgoing calls for %s: %w", item.Name, outErr))
					} else {
						outgoingCalls = append(outgoingCalls, outCalls...)
					}
				}
			}

			// Format results
			result := formatCallHierarchyResults(
				allPrepItems,
				successfulLanguage,
				append(prepErrors, callErrors...),
				uri,
				line,
				character,
				direction,
				incomingCalls,
				outgoingCalls,
			)

			return mcp.NewToolResultText(result), nil
		}
}

// formatCallHierarchyResults formats call hierarchy results for user-friendly output
func formatCallHierarchyResults(items []protocol.CallHierarchyItem, successfulLanguage string, errors []error, uri string, line, character int, direction string, incomingCalls []protocol.CallHierarchyIncomingCall, outgoingCalls []protocol.CallHierarchyOutgoingCall) string {
	var result strings.Builder

	// Header with summary
	fmt.Fprintf(&result, "CALL HIERARCHY:\n")
	fmt.Fprintf(&result, "Position: %s:%d:%d\n", uri, line, character)
	fmt.Fprintf(&result, "Language: %s\n", successfulLanguage)
	fmt.Fprintf(&result, "Items found: %d\n", len(items))
	if len(errors) > 0 {
		fmt.Fprintf(&result, "Errors: %d\n", len(errors))
	}
	fmt.Fprintf(&result, "\n")

	// Show errors if any
	if len(errors) > 0 {
		fmt.Fprintf(&result, "ERRORS:\n")
		for i, err := range errors {
			fmt.Fprintf(&result, "%d. %v\n", i+1, err)
		}
		fmt.Fprintf(&result, "\n")
	}

	// Show call hierarchy items
	if len(items) > 0 {
		fmt.Fprintf(&result, "CALL HIERARCHY ITEMS:\n")
		for i, item := range items {
			fmt.Fprintf(&result, "%d. %s\n", i+1, item.Name)
			fmt.Fprintf(&result, "   Kind: %s\n", symbolKindToString(item.Kind))
			fmt.Fprintf(&result, "   URI: %s\n", item.Uri)
			fmt.Fprintf(&result, "   Range: %d:%d-%d:%d\n",
				item.Range.Start.Line, item.Range.Start.Character,
				item.Range.End.Line, item.Range.End.Character)
			fmt.Fprintf(&result, "   Selection Range: %d:%d-%d:%d\n",
				item.SelectionRange.Start.Line, item.SelectionRange.Start.Character,
				item.SelectionRange.End.Line, item.SelectionRange.End.Character)
			if item.Detail != "" {
				fmt.Fprintf(&result, "   Detail: %s\n", item.Detail)
			}
			if len(item.Tags) > 0 {
				fmt.Fprintf(&result, "   Tags: %v\n", item.Tags)
			}
			fmt.Fprintf(&result, "\n")
		}
	}

	// Display call details based on direction
	switch direction {
	case "incoming":
		fmt.Fprintf(&result, "INCOMING CALLS (%d):\n", len(incomingCalls))
		for _, call := range incomingCalls {
			fmt.Fprintf(&result, "Caller: %s\n", call.From.Name)
			fmt.Fprintf(&result, "   From: %s\n", call.From.Uri)
			fmt.Fprintf(&result, "   Call Ranges: %d\n", len(call.FromRanges))
			for i, callRange := range call.FromRanges {
				if i >= 3 { // Limit to first 3 ranges to avoid overwhelming output
					fmt.Fprintf(&result, "   ... and %d more ranges\n", len(call.FromRanges)-3)
					break
				}
				fmt.Fprintf(&result, "     - Line %d:%d-%d:%d\n",
					callRange.Start.Line+1, callRange.Start.Character+1,
					callRange.End.Line+1, callRange.End.Character+1)
			}
		}
	case "outgoing":
		fmt.Fprintf(&result, "OUTGOING CALLS (%d):\n", len(outgoingCalls))
		for _, call := range outgoingCalls {
			fmt.Fprintf(&result, "Callee: %s\n", call.To.Name)
			fmt.Fprintf(&result, "   To: %s\n", call.To.Uri)
			fmt.Fprintf(&result, "   Call Ranges: %d\n", len(call.FromRanges))
			for i, callRange := range call.FromRanges {
				if i >= 3 { // Limit to first 3 ranges to avoid overwhelming output
					fmt.Fprintf(&result, "   ... and %d more ranges\n", len(call.FromRanges)-3)
					break
				}
				fmt.Fprintf(&result, "     - Line %d:%d-%d:%d\n",
					callRange.Start.Line+1, callRange.Start.Character+1,
					callRange.End.Line+1, callRange.End.Character+1)
			}
		}
	case "both":
		fmt.Fprintf(&result, "INCOMING CALLS (%d):\n", len(incomingCalls))
		for _, call := range incomingCalls {
			fmt.Fprintf(&result, "Caller: %s\n", call.From.Name)
			fmt.Fprintf(&result, "   From: %s\n", call.From.Uri)
			fmt.Fprintf(&result, "   Call Ranges: %d\n", len(call.FromRanges))
			for i, callRange := range call.FromRanges {
				if i >= 3 { // Limit to first 3 ranges to avoid overwhelming output
					fmt.Fprintf(&result, "   ... and %d more ranges\n", len(call.FromRanges)-3)
					break
				}
				fmt.Fprintf(&result, "     - Line %d:%d-%d:%d\n",
					callRange.Start.Line+1, callRange.Start.Character+1,
					callRange.End.Line+1, callRange.End.Character+1)
			}
		}
		fmt.Fprintf(&result, "\nOUTGOING CALLS (%d):\n", len(outgoingCalls))
		for _, call := range outgoingCalls {
			fmt.Fprintf(&result, "Callee: %s\n", call.To.Name)
			fmt.Fprintf(&result, "   To: %s\n", call.To.Uri)
			fmt.Fprintf(&result, "   Call Ranges: %d\n", len(call.FromRanges))
			for i, callRange := range call.FromRanges {
				if i >= 3 { // Limit to first 3 ranges to avoid overwhelming output
					fmt.Fprintf(&result, "   ... and %d more ranges\n", len(call.FromRanges)-3)
					break
				}
				fmt.Fprintf(&result, "     - Line %d:%d-%d:%d\n",
					callRange.Start.Line+1, callRange.Start.Character+1,
					callRange.End.Line+1, callRange.End.Character+1)
			}
		}
	}

	return result.String()
}
