package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	bridgepkg "rockerboo/mcp-lsp-bridge/bridge"
	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/mark3labs/mcp-go/mcp"
)

type LSPActivity struct {
	Server      string  `json:"server"`
	Token       string  `json:"token"`
	Kind        string  `json:"kind"`
	Title       string  `json:"title,omitempty"`
	Message     string  `json:"message,omitempty"`
	Percentage  *uint32 `json:"percentage,omitempty"`
	Cancellable *bool   `json:"cancellable,omitempty"`
}

type LSPClientStatus struct {
	Server         string `json:"server"`
	Command        string `json:"command,omitempty"`
	Connected      bool   `json:"connected"`
	Status         string `json:"status"`
	LastError      string `json:"last_error,omitempty"`
	ActiveProgress int    `json:"active_progress"`
}

type IndexingProgress struct {
	State          string `json:"state"` // "idle" | "indexing" | "complete"
	Current        int    `json:"current"`
	Total          int    `json:"total"`
	ETASeconds     int    `json:"eta_seconds,omitempty"`
	ElapsedSeconds int    `json:"elapsed_seconds,omitempty"`
	Message        string `json:"message,omitempty"`
}

type LSPStatus struct {
	Ready    bool              `json:"ready"`
	State    string            `json:"state"`
	Activity []LSPActivity     `json:"activity"`
	Clients  []LSPClientStatus `json:"clients,omitempty"`
	Indexing *IndexingProgress `json:"indexing,omitempty"`
}

type LSPStatusResponse struct {
	LSPStatus
	RetryAfterMs int `json:"retry_after_ms,omitempty"`
}

func BuildLSPStatus(bridge interfaces.BridgeInterface) (LSPStatus, error) {
	b, ok := bridge.(*bridgepkg.MCPLSPBridge)
	if !ok {
		return LSPStatus{}, fmt.Errorf("bridge does not support status introspection")
	}

	clients := b.ListConnectedClients()
	status := LSPStatus{
		Ready:    false,
		State:    "starting",
		Activity: []LSPActivity{},
		Clients:  []LSPClientStatus{},
	}

	if len(clients) == 0 {
		return status, nil
	}

	servers := make([]string, 0, len(clients))
	for srv := range clients {
		servers = append(servers, string(srv))
	}
	sort.Strings(servers)

	connectedCount := 0
	anyError := false
	anyStarting := false
	anyBusy := false

	for _, srv := range servers {
		client := clients[types.LanguageServer(srv)]
		metrics := client.GetMetrics()

		statusStr := lsp.ClientStatus(metrics.GetStatus()).String()
		lastError := metrics.GetLastError()
		connected := metrics.IsConnected()
		if connected {
			connectedCount++
		}

		// IMPORTANT:
		// The LSP client status "error" means "last request failed" (e.g. server busy/indexing),
		// not necessarily that the underlying transport is broken. Do NOT block all tools on that.
		// We only treat *connection-level* problems as readiness blockers.
		isConnError := statusStr == "disconnected" || !connected ||
			strings.Contains(lastError, "connection is closed") ||
			strings.Contains(lastError, "already disconnected") ||
			strings.Contains(lastError, "EOF")
		if isConnError {
			anyError = true
		}
		if statusStr == "connecting" || statusStr == "uninitialized" || statusStr == "restarting" {
			anyStarting = true
		}

		activeCount := 0
		if ps, ok := client.(interface{ ProgressSnapshot() lsp.ProgressSnapshot }); ok {
			snap := ps.ProgressSnapshot()
			activeCount = len(snap.Active)
			if activeCount > 0 {
				anyBusy = true
				for _, ev := range snap.Active {
					status.Activity = append(status.Activity, LSPActivity{
						Server:      srv,
						Token:       ev.TokenKey,
						Kind:        ev.Kind,
						Title:       ev.Title,
						Message:     ev.Message,
						Percentage:  ev.Percentage,
						Cancellable: ev.Cancellable,
					})
				}
			}
		}

		status.Clients = append(status.Clients, LSPClientStatus{
			Server:         srv,
			Command:        metrics.GetCommand(),
			Connected:      connected,
			Status:         statusStr,
			LastError:      lastError,
			ActiveProgress: activeCount,
		})

		// Try to get indexing status from SessionAdapter
		if status.Indexing == nil {
			if sa, ok := client.(*lsp.SessionAdapter); ok {
				if idxStatus := sa.GetIndexingStatus(); idxStatus != nil {
					status.Indexing = &IndexingProgress{
						State:          idxStatus.State,
						Current:        idxStatus.Current,
						Total:          idxStatus.Total,
						ETASeconds:     idxStatus.ETASeconds,
						ElapsedSeconds: idxStatus.ElapsedSeconds,
						Message:        idxStatus.Message,
					}
					// If indexing is active, mark as busy
					if idxStatus.State == "indexing" {
						anyBusy = true
					}
				}
			}
		}
	}

	switch {
	case anyError:
		status.State = "error"
	case anyBusy:
		status.State = "busy"
	case anyStarting || connectedCount == 0:
		status.State = "starting"
	default:
		status.State = "ready"
	}

	// IMPORTANT: "busy" (indexing/progress) should not block tool usage.
	// We consider the system ready as soon as at least one client is connected
	// and there are no connection/errors.
	status.Ready = connectedCount > 0 && !anyError

	return status, nil
}

func FormatLSPStatus(status LSPStatus) (string, error) {
	raw, err := json.Marshal(status)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func FormatLSPStatusResponse(status LSPStatus, retryAfterMs int) (string, error) {
	resp := LSPStatusResponse{
		LSPStatus:    status,
		RetryAfterMs: retryAfterMs,
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func CheckReadyOrReturn(bridge interfaces.BridgeInterface) (*mcp.CallToolResult, bool) {
	// If we're running with the concrete bridge, trigger auto-connect as needed.
	// This removes the need for an explicit lsp_connect tool call.
	if b, ok := bridge.(*bridgepkg.MCPLSPBridge); ok {
		status, err := BuildLSPStatus(bridge)
		if err == nil && !status.Ready {
			// If there are no connected clients (or we are still starting), attempt (re)connect.
			connected := 0
			for _, c := range status.Clients {
				if c.Connected {
					connected++
				}
			}
			if connected == 0 && (status.State == "starting" || status.State == "error") {
				b.StartAutoConnect()
			}
		} else if err != nil && len(b.ListConnectedClients()) == 0 {
			b.StartAutoConnect()
		}

		// Give the background connect a small head-start to avoid returning "starting"
		// on the very first tool call in normal cases.
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			s, e := BuildLSPStatus(bridge)
			if e != nil {
				break
			}
			if s.Ready {
				// In session mode, LSP Session Manager handles warmup - skip our gate.
				if b.AllClientsInSessionMode() {
					return nil, true
				}
				// Hard gate: warm-up must be finished before tools run (variant A).
				running, done, werr, _, _ := b.WarmupStatus()
				if done && werr == "" {
					return nil, true
				}
				// Ensure warm-up is started.
				if !running && !done {
					b.StartWarmup()
				}
				status.State = "warming"
				retryAfterMs := 2000
				payload, _ := FormatLSPStatusResponse(status, retryAfterMs)
				return mcp.NewToolResultText(payload), false
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	status, err := BuildLSPStatus(bridge)
	if err != nil {
		// In unit tests and in alternative bridge implementations we may not support
		// status introspection. In that case, don't block tool execution.
		if strings.Contains(err.Error(), "does not support status introspection") {
			return nil, true
		}
		return mcp.NewToolResultError(err.Error()), false
	}
	if status.Ready {
		// Hard gate: warm-up must be finished before tools run (variant A).
		if b, ok := bridge.(*bridgepkg.MCPLSPBridge); ok {
			// In session mode, LSP Session Manager handles warmup - skip our gate.
			if b.AllClientsInSessionMode() {
				return nil, true
			}
			running, done, werr, _, _ := b.WarmupStatus()
			if done && werr == "" {
				return nil, true
			}
			if !running && !done {
				b.StartWarmup()
			}
			status.State = "warming"
			retryAfterMs := 2000
			payload, _ := FormatLSPStatusResponse(status, retryAfterMs)
			return mcp.NewToolResultText(payload), false
		}
		return nil, true
	}
	retryAfterMs := 0
	if status.State == "busy" || status.State == "starting" {
		retryAfterMs = 2000
	}
	payload, err := FormatLSPStatusResponse(status, retryAfterMs)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), false
	}
	return mcp.NewToolResultText(payload), false
}
