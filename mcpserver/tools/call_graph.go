package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Constants for call graph
const (
	DefaultDepthUp   = 5
	DefaultDepthDown = 5
	DefaultMaxNodes  = 100
	HardLimitNodes   = 500
	TimeoutSeconds   = 60
)

// Known BSL entry points (event handlers, commands, etc.)
var bslEntryPoints = map[string]bool{
	// Document events
	"ПриЗаписи":                   true,
	"ПриПроведении":               true,
	"ПриОтменеПроведения":         true,
	"ПередЗаписью":                true,
	"ПередУдалением":              true,
	"ПриУстановкеНовогоНомера":    true,
	"ПриКопировании":              true,
	"ОбработкаЗаполнения":         true,
	"ОбработкаПроверкиЗаполнения": true,
	// Form events
	"ПриСозданииНаСервере":         true,
	"ПриОткрытии":                  true,
	"ПриЗакрытии":                  true,
	"ПередЗаписьюНаСервере":        true,
	"ПриЗаписиНаСервере":           true,
	"ПослеЗаписиНаСервере":         true,
	"ПриЧтенииНаСервере":           true,
	"ОбработкаОповещения":          true,
	"ОбработкаНавигационнойСсылки": true,
	// Commands
	"ОбработкаКоманды": true,
	"ПриВыполнении":    true,
	// Session events
	"ПриНачалеРаботыСистемы":        true,
	"ПриЗавершенииРаботыСистемы":    true,
	"ПередНачаломРаботыСистемы":     true,
	"ПередЗавершениемРаботыСистемы": true,
	// Scheduled jobs
	"ОбработчикРегламентногоЗадания": true,
	// HTTP services
	"ОбработкаВызоваHTTPСервиса": true,
	// Web services
	"ОбработкаВызоваWebСервиса": true,
	// English equivalents
	"OnWrite":          true,
	"Posting":          true,
	"OnOpen":           true,
	"OnCreateAtServer": true,
	"BeforeWrite":      true,
	"OnClose":          true,
}

// CallGraphNode represents a node in the call graph
type CallGraphNode struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Kind         string           `json:"kind"`
	URI          string           `json:"uri"`
	Line         uint32           `json:"line"`
	Character    uint32           `json:"character"`
	IsEntryPoint bool             `json:"is_entry_point,omitempty"`
	IsCycle      bool             `json:"is_cycle,omitempty"`
	Depth        int              `json:"depth"`
	Direction    string           `json:"direction"` // "up", "down", "root"
	Children     []*CallGraphNode `json:"children,omitempty"`
}

// CallGraphResult is the complete result of call graph analysis
type CallGraphResult struct {
	Root           *CallGraphNode `json:"root"`
	IncomingTree   *CallGraphNode `json:"incoming_tree,omitempty"`
	OutgoingTree   *CallGraphNode `json:"outgoing_tree,omitempty"`
	TotalNodes     int            `json:"total_nodes"`
	MaxDepthUp     int            `json:"max_depth_up_reached"`
	MaxDepthDown   int            `json:"max_depth_down_reached"`
	Truncated      bool           `json:"truncated"`
	TruncateReason string         `json:"truncate_reason,omitempty"`
	CyclesFound    int            `json:"cycles_found"`
	EntryPoints    []string       `json:"entry_points_found,omitempty"`
	ElapsedMs      int64          `json:"elapsed_ms"`
}

// callGraphBuilder manages the recursive graph building
type callGraphBuilder struct {
	bridge         interfaces.BridgeInterface
	visited        map[string]bool
	visitedMu      sync.RWMutex
	nodeCount      int
	nodeCountMu    sync.Mutex
	maxNodes       int
	depthUp        int
	depthDown      int
	cyclesFound    int
	cyclesMu       sync.Mutex
	entryPoints    []string
	entryMu        sync.Mutex
	maxDepthUp     int
	maxDepthDown   int
	depthMu        sync.Mutex
	ctx            context.Context
	truncated      bool
	truncateReason string
}

// RegisterCallGraphTool registers the call graph tool
func RegisterCallGraphTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(CallGraphTool(bridge))
}

func CallGraphTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("call_graph",
			mcp.WithDescription(`Build a complete call graph by recursively traversing call hierarchy.
Returns JSON with incoming (callers) and outgoing (callees) trees.

EXCELLENT for:
- Understanding code flow and dependencies
- Impact analysis before refactoring
- Finding entry points (event handlers, commands)
- Tracing execution paths

Parameters:
- depth_up: How deep to trace callers (default: 5, 0 = unlimited up to hard limit)
- depth_down: How deep to trace callees (default: 5, 0 = unlimited up to hard limit)
- max_nodes: Maximum nodes to collect (default: 100, 0 = unlimited up to 500)

Output includes:
- Complete call trees (incoming/outgoing)
- Entry point detection (BSL events like ПриЗаписи, ПриОткрытии)
- Cycle detection with markers
- Truncation info if limits reached`),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithNumber("line", mcp.Description("Line number (0-based)"), mcp.Required()),
			mcp.WithNumber("character", mcp.Description("Character position (0-based)"), mcp.Required()),
			mcp.WithNumber("depth_up", mcp.Description("Max depth for incoming calls (default: 5, 0 = unlimited)")),
			mcp.WithNumber("depth_down", mcp.Description("Max depth for outgoing calls (default: 5, 0 = unlimited)")),
			mcp.WithNumber("max_nodes", mcp.Description("Max total nodes (default: 100, 0 = unlimited, hard limit: 500)")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			startTime := time.Now()

			// Parse required parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				return mcp.NewToolResultError("uri is required"), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				return mcp.NewToolResultError("line is required"), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				return mcp.NewToolResultError("character is required"), nil
			}

			// Parse optional parameters with defaults
			depthUp := DefaultDepthUp
			if val, err := request.RequireInt("depth_up"); err == nil {
				depthUp = val
				if depthUp == 0 {
					depthUp = HardLimitNodes // 0 means unlimited, but respect hard limit through node count
				}
			}

			depthDown := DefaultDepthDown
			if val, err := request.RequireInt("depth_down"); err == nil {
				depthDown = val
				if depthDown == 0 {
					depthDown = HardLimitNodes
				}
			}

			maxNodes := DefaultMaxNodes
			if val, err := request.RequireInt("max_nodes"); err == nil {
				maxNodes = val
				if maxNodes == 0 || maxNodes > HardLimitNodes {
					maxNodes = HardLimitNodes
				}
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			// Create timeout context
			timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutSeconds*time.Second)
			defer cancel()

			// Validate parameters
			lineUint32, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			characterUint32, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}

			// Normalize URI
			normalizedURI := bridge.NormalizeURIForLSP(uri)

			// Prepare call hierarchy to get the root item
			prepItems, err := bridge.PrepareCallHierarchy(normalizedURI, lineUint32, characterUint32)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to prepare call hierarchy: %v", err)), nil
			}

			if len(prepItems) == 0 {
				return mcp.NewToolResultText(`{"error": "No call hierarchy item found at this position"}`), nil
			}

			// Use first item as root
			rootItem := prepItems[0]

			// Create builder
			builder := &callGraphBuilder{
				bridge:    bridge,
				visited:   make(map[string]bool),
				maxNodes:  maxNodes,
				depthUp:   depthUp,
				depthDown: depthDown,
				ctx:       timeoutCtx,
			}

			// Build root node
			rootNode := builder.itemToNode(&rootItem, 0, "root")

			// Check if root is an entry point
			if isEntryPoint(rootItem.Name) {
				rootNode.IsEntryPoint = true
				builder.addEntryPoint(rootItem.Name)
			}

			// Build incoming tree (callers) - parallel
			var incomingTree *CallGraphNode
			var outgoingTree *CallGraphNode
			var wg sync.WaitGroup

			wg.Add(2)

			go func() {
				defer wg.Done()
				incomingTree = builder.buildIncomingTree(&rootItem, 1)
			}()

			go func() {
				defer wg.Done()
				outgoingTree = builder.buildOutgoingTree(&rootItem, 1)
			}()

			wg.Wait()

			// Build result
			result := &CallGraphResult{
				Root:           rootNode,
				IncomingTree:   incomingTree,
				OutgoingTree:   outgoingTree,
				TotalNodes:     builder.nodeCount,
				MaxDepthUp:     builder.maxDepthUp,
				MaxDepthDown:   builder.maxDepthDown,
				Truncated:      builder.truncated,
				TruncateReason: builder.truncateReason,
				CyclesFound:    builder.cyclesFound,
				EntryPoints:    builder.entryPoints,
				ElapsedMs:      time.Since(startTime).Milliseconds(),
			}

			// Marshal to JSON
			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
			}

			return mcp.NewToolResultText(string(jsonBytes)), nil
		}
}

// itemToNode converts a CallHierarchyItem to a CallGraphNode
func (b *callGraphBuilder) itemToNode(item *protocol.CallHierarchyItem, depth int, direction string) *CallGraphNode {
	b.nodeCountMu.Lock()
	b.nodeCount++
	b.nodeCountMu.Unlock()

	return &CallGraphNode{
		ID:        fmt.Sprintf("%s:%d:%d", item.Uri, item.Range.Start.Line, item.Range.Start.Character),
		Name:      item.Name,
		Kind:      symbolKindToString(item.Kind),
		URI:       string(item.Uri),
		Line:      item.Range.Start.Line,
		Character: item.Range.Start.Character,
		Depth:     depth,
		Direction: direction,
	}
}

// buildIncomingTree recursively builds the incoming calls tree
func (b *callGraphBuilder) buildIncomingTree(item *protocol.CallHierarchyItem, depth int) *CallGraphNode {
	// Check context cancellation (timeout)
	select {
	case <-b.ctx.Done():
		b.setTruncated("timeout after 60 seconds")
		return nil
	default:
	}

	// Check depth limit
	if depth > b.depthUp {
		return nil
	}

	// Check node limit
	b.nodeCountMu.Lock()
	if b.nodeCount >= b.maxNodes {
		b.nodeCountMu.Unlock()
		b.setTruncated(fmt.Sprintf("max_nodes limit reached (%d)", b.maxNodes))
		return nil
	}
	b.nodeCountMu.Unlock()

	// Update max depth reached
	b.depthMu.Lock()
	if depth > b.maxDepthUp {
		b.maxDepthUp = depth
	}
	b.depthMu.Unlock()

	// Get incoming calls from LSP
	calls, err := b.bridge.IncomingCalls(*item)
	if err != nil {
		logger.Error("call_graph: failed to get incoming calls", err)
		return nil
	}

	if len(calls) == 0 {
		return nil
	}

	// Create container node for incoming calls
	containerNode := &CallGraphNode{
		ID:        fmt.Sprintf("incoming-%s:%d", item.Uri, item.Range.Start.Line),
		Name:      fmt.Sprintf("Callers of %s", item.Name),
		Direction: "up",
		Depth:     depth,
		Children:  make([]*CallGraphNode, 0, len(calls)),
	}

	// Process calls in parallel with limiting
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 5) // Limit concurrent LSP calls

	for _, call := range calls {
		// Check limits before spawning goroutine
		b.nodeCountMu.Lock()
		if b.nodeCount >= b.maxNodes {
			b.nodeCountMu.Unlock()
			b.setTruncated(fmt.Sprintf("max_nodes limit reached (%d)", b.maxNodes))
			break
		}
		b.nodeCountMu.Unlock()

		select {
		case <-b.ctx.Done():
			b.setTruncated("timeout after 60 seconds")
			break
		default:
		}

		wg.Add(1)
		callCopy := call // Capture for goroutine

		go func() {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			callerItem := callCopy.From
			nodeKey := fmt.Sprintf("%s:%d:%d", callerItem.Uri, callerItem.Range.Start.Line, callerItem.Range.Start.Character)

			// Check for cycle
			b.visitedMu.RLock()
			isCycle := b.visited[nodeKey]
			b.visitedMu.RUnlock()

			node := b.itemToNode(&callerItem, depth, "up")

			if isCycle {
				node.IsCycle = true
				b.cyclesMu.Lock()
				b.cyclesFound++
				b.cyclesMu.Unlock()

				mu.Lock()
				containerNode.Children = append(containerNode.Children, node)
				mu.Unlock()
				return
			}

			// Mark as visited
			b.visitedMu.Lock()
			b.visited[nodeKey] = true
			b.visitedMu.Unlock()

			// Check if entry point
			if isEntryPoint(callerItem.Name) {
				node.IsEntryPoint = true
				b.addEntryPoint(callerItem.Name)
			}

			// Recurse for incoming calls
			childTree := b.buildIncomingTree(&callerItem, depth+1)
			if childTree != nil && len(childTree.Children) > 0 {
				node.Children = childTree.Children
			}

			mu.Lock()
			containerNode.Children = append(containerNode.Children, node)
			mu.Unlock()
		}()
	}

	wg.Wait()

	if len(containerNode.Children) == 0 {
		return nil
	}

	return containerNode
}

// buildOutgoingTree recursively builds the outgoing calls tree
func (b *callGraphBuilder) buildOutgoingTree(item *protocol.CallHierarchyItem, depth int) *CallGraphNode {
	// Check context cancellation (timeout)
	select {
	case <-b.ctx.Done():
		b.setTruncated("timeout after 60 seconds")
		return nil
	default:
	}

	// Check depth limit
	if depth > b.depthDown {
		return nil
	}

	// Check node limit
	b.nodeCountMu.Lock()
	if b.nodeCount >= b.maxNodes {
		b.nodeCountMu.Unlock()
		b.setTruncated(fmt.Sprintf("max_nodes limit reached (%d)", b.maxNodes))
		return nil
	}
	b.nodeCountMu.Unlock()

	// Update max depth reached
	b.depthMu.Lock()
	if depth > b.maxDepthDown {
		b.maxDepthDown = depth
	}
	b.depthMu.Unlock()

	// Get outgoing calls from LSP
	calls, err := b.bridge.OutgoingCalls(*item)
	if err != nil {
		logger.Error("call_graph: failed to get outgoing calls", err)
		return nil
	}

	if len(calls) == 0 {
		return nil
	}

	// Create container node for outgoing calls
	containerNode := &CallGraphNode{
		ID:        fmt.Sprintf("outgoing-%s:%d", item.Uri, item.Range.Start.Line),
		Name:      fmt.Sprintf("Calls from %s", item.Name),
		Direction: "down",
		Depth:     depth,
		Children:  make([]*CallGraphNode, 0, len(calls)),
	}

	// Process calls in parallel with limiting
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 5) // Limit concurrent LSP calls

	for _, call := range calls {
		// Check limits before spawning goroutine
		b.nodeCountMu.Lock()
		if b.nodeCount >= b.maxNodes {
			b.nodeCountMu.Unlock()
			b.setTruncated(fmt.Sprintf("max_nodes limit reached (%d)", b.maxNodes))
			break
		}
		b.nodeCountMu.Unlock()

		select {
		case <-b.ctx.Done():
			b.setTruncated("timeout after 60 seconds")
			break
		default:
		}

		wg.Add(1)
		callCopy := call // Capture for goroutine

		go func() {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			calleeItem := callCopy.To
			nodeKey := fmt.Sprintf("%s:%d:%d", calleeItem.Uri, calleeItem.Range.Start.Line, calleeItem.Range.Start.Character)

			// Check for cycle
			b.visitedMu.RLock()
			isCycle := b.visited[nodeKey]
			b.visitedMu.RUnlock()

			node := b.itemToNode(&calleeItem, depth, "down")

			if isCycle {
				node.IsCycle = true
				b.cyclesMu.Lock()
				b.cyclesFound++
				b.cyclesMu.Unlock()

				mu.Lock()
				containerNode.Children = append(containerNode.Children, node)
				mu.Unlock()
				return
			}

			// Mark as visited
			b.visitedMu.Lock()
			b.visited[nodeKey] = true
			b.visitedMu.Unlock()

			// Recurse for outgoing calls
			childTree := b.buildOutgoingTree(&calleeItem, depth+1)
			if childTree != nil && len(childTree.Children) > 0 {
				node.Children = childTree.Children
			}

			mu.Lock()
			containerNode.Children = append(containerNode.Children, node)
			mu.Unlock()
		}()
	}

	wg.Wait()

	if len(containerNode.Children) == 0 {
		return nil
	}

	return containerNode
}

// isEntryPoint checks if a method name is a known BSL entry point
func isEntryPoint(name string) bool {
	// Check exact match first
	if bslEntryPoints[name] {
		return true
	}

	// Check if name contains known entry point (for cases like "Форма_ПриОткрытии")
	for ep := range bslEntryPoints {
		if strings.Contains(name, ep) {
			return true
		}
	}

	return false
}

// addEntryPoint safely adds an entry point to the list
func (b *callGraphBuilder) addEntryPoint(name string) {
	b.entryMu.Lock()
	defer b.entryMu.Unlock()

	// Check for duplicates
	for _, ep := range b.entryPoints {
		if ep == name {
			return
		}
	}
	b.entryPoints = append(b.entryPoints, name)
}

// setTruncated safely sets truncation status
func (b *callGraphBuilder) setTruncated(reason string) {
	b.nodeCountMu.Lock()
	defer b.nodeCountMu.Unlock()

	if !b.truncated {
		b.truncated = true
		b.truncateReason = reason
	}
}
