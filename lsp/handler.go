package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/sourcegraph/jsonrpc2"
	"rockerboo/mcp-lsp-bridge/logger"
)

// ClientHandler handles incoming messages from the language server
type ClientHandler struct {
	progress *ProgressTracker
}

func (h *ClientHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	switch req.Method {
	case "$/progress", "#/progress":
		// LSP progress notification (server-initiated workDone progress).
		// NOTE: LSP spec defines $/progress. We also accept "#/progress" just in case.
		if req.Params == nil {
			return
		}
		var params protocol.ProgressParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			logger.Debug(fmt.Sprintf("Failed to unmarshal progress params: %v\n", err))
			return
		}
		if h.progress != nil {
			h.progress.Update(params)
		}
		return

	case "window/workDoneProgress/create":
		// Server asks client to create a token for progress reporting.
		// We just acknowledge; the token is provided by server and used in subsequent $/progress.
		if req.Params != nil {
			var params protocol.WorkDoneProgressCreateParams
			if err := json.Unmarshal(*req.Params, &params); err == nil {
				if h.progress != nil {
					h.progress.RegisterToken(params.Token)
				}
			}
		}
		if err := conn.Reply(ctx, req.ID, map[string]any{}); err != nil {
			logger.Debug(fmt.Sprintf("Failed to reply to workDoneProgress/create: %v\n", err))
		}
		return

	case "textDocument/publishDiagnostics":
		// Handle diagnostics
		var params any
		if err := json.Unmarshal(*req.Params, &params); err == nil {
			logger.Debug(fmt.Sprintf("Diagnostics: %+v\n", params))
		}

	case "window/showMessage":
		// Handle show message
		var params any
		if err := json.Unmarshal(*req.Params, &params); err == nil {
			logger.Debug(fmt.Sprintf("Server message: %+v\n", params))
		}

	case "window/logMessage":
		// Handle log message
		var params any
		if err := json.Unmarshal(*req.Params, &params); err == nil {
			logger.Info(fmt.Sprintf("Server log: %+v\n", params))
		}

	case "client/registerCapability":
		// Handle capability registration - reply with success
		if err := conn.Reply(ctx, req.ID, map[string]any{}); err != nil {
			logger.Debug(fmt.Sprintf("Failed to reply to registerCapability: %v\n", err))
		}

	case "workspace/configuration":
		// Handle configuration request - reply with empty config
		if err := conn.Reply(ctx, req.ID, []any{}); err != nil {
			logger.Debug(fmt.Sprintf("Failed to reply to configuration: %v\n", err))
		}

	default:
		// IMPORTANT:
		// - For notifications, we MUST NOT reply with an error (it can break some servers).
		// - For unknown requests, reply with method-not-found.
		if req.Notif {
			logUnhandledNotification(req.Method, req.Params)
			return
		}

		if req.Params != nil {
			logger.Error(fmt.Sprintf("Unhandled request method: %s with params: %s", req.Method, string(*req.Params)))
		} else {
			logger.Error(fmt.Sprintf("Unhandled request method: %s (no params)", req.Method))
		}

		err := &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound, Message: "Method not found"}
		if replyErr := conn.ReplyWithError(ctx, req.ID, err); replyErr != nil {
			logger.Error(fmt.Sprintf("Failed to reply with error: %v", replyErr))
		}
	}
}
