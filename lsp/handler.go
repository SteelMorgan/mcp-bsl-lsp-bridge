package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
	"rockerboo/mcp-lsp-bridge/logger"
)

// ClientHandler handles incoming messages from the language server
type ClientHandler struct {
	// No client reference needed - handler is stateless
}

func (h *ClientHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	switch req.Method {
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
		logger.Error(fmt.Sprintf("Unhandled method: %s with params: %s", req.Method, string(*req.Params)))

		err := &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: "Method not found",
		}
		if replyErr := conn.ReplyWithError(ctx, req.ID, err); replyErr != nil {
			logger.Error(fmt.Sprintf("Failed to reply with error: %v", replyErr))
		}
	}
}
