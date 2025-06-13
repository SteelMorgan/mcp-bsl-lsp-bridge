package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

// ClientHandler handles incoming messages from the language server
type ClientHandler struct {
	client *LanguageClient
}

func (h *ClientHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	fmt.Printf("Received from server - Method: %s, ID: %v\n", req.Method, req.ID)

	switch req.Method {
	case "textDocument/publishDiagnostics":
		// Handle diagnostics
		var params any
		if err := json.Unmarshal(*req.Params, &params); err == nil {
			fmt.Printf("Diagnostics: %+v\n", params)
		}

	case "window/showMessage":
		// Handle show message
		var params any
		if err := json.Unmarshal(*req.Params, &params); err == nil {
			fmt.Printf("Server message: %+v\n", params)
		}

	case "window/logMessage":
		// Handle log message
		var params any
		if err := json.Unmarshal(*req.Params, &params); err == nil {
			fmt.Printf("Server log: %+v\n", params)
		}

	case "client/registerCapability":
		// Handle capability registration - reply with success
		if err := conn.Reply(ctx, req.ID, map[string]any{}); err != nil {
			fmt.Printf("Failed to reply to registerCapability: %v\n", err)
		}

	case "workspace/configuration":
		// Handle configuration request - reply with empty config
		if err := conn.Reply(ctx, req.ID, []any{}); err != nil {
			fmt.Printf("Failed to reply to configuration: %v\n", err)
		}

	default:
		fmt.Printf("Unhandled method: %s with params: %s\n", req.Method, string(*req.Params))

		err := &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: "Method not found",
		}
		if replyErr := conn.ReplyWithError(ctx, req.ID, err); replyErr != nil {
			fmt.Printf("Failed to reply with error: %v\n", replyErr)
		}
	}
}