// Command mock-lsp-server is a minimal LSP server fixture used by the lsp
// package tests. It speaks JSON-RPC 2.0 over stdio (VSCode object codec, like a
// real language server) and implements just enough behavior for the tests:
//
//   - "initialize"        -> replies with an empty-capabilities result (success)
//   - "shutdown"          -> replies with null (success)
//   - notifications       -> ignored (no response)
//   - any other method    -> replies with a JSON-RPC "method not found" error
//
// It deliberately has no external dependencies beyond what the main module
// already vendors, so `go test ./lsp/` can build and run it on the fly without
// installing the external rockerBOO/mock-lsp-server tool.
package main

import (
	"context"
	"io"
	"os"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

type stdioReadWriteCloser struct {
	io.Reader
	io.Writer
}

func (stdioReadWriteCloser) Close() error { return nil }

type mockHandler struct{}

func (mockHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// Notifications (initialized, exit, textDocument/did*, ...) carry no id and
	// must not be answered.
	if req.Notif {
		return
	}

	switch req.Method {
	case "initialize":
		_ = conn.Reply(ctx, req.ID, protocol.InitializeResult{
			Capabilities: protocol.ServerCapabilities{},
		})
	case "shutdown":
		_ = conn.Reply(ctx, req.ID, nil)
	default:
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    -32601, // MethodNotFound
			Message: "method not found: " + req.Method,
		})
	}
}

func main() {
	stream := jsonrpc2.NewBufferedStream(
		stdioReadWriteCloser{Reader: os.Stdin, Writer: os.Stdout},
		jsonrpc2.VSCodeObjectCodec{},
	)

	conn := jsonrpc2.NewConn(context.Background(), stream, jsonrpc2.AsyncHandler(mockHandler{}))
	<-conn.DisconnectNotify()
}
