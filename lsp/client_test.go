package lsp_test

import (
	"context"
	"testing"
	"time"

	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// [Previous test functions remain the same]

func TestLanguageClient_SendRequest(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		command string
		args    []string
		// Named input parameters for target function.
		method  string
		params  any
		result  any
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "valid initialize request",
			command: "mock-lsp-server",
			args:    []string{},
			method:  "initialize",
			params: protocol.InitializeParams{
				ProcessId: func() *int32 { v := int32(12345); return &v }(),
				ClientInfo: &protocol.ClientInfo{
					Name:    "test-client{",
					Version: "1.0.0",
				},
				Capabilities: protocol.ClientCapabilities{},
			},
			result:  &map[string]any{},
			timeout: 1 * time.Second,
			wantErr: false,
		},
		{
			name:    "empty method name should fail",
			command: "mock-lsp-server",
			args:    []string{},
			method:  "",
			params:  map[string]any{},
			result:  &map[string]any{},
			timeout: 1 * time.Second,
			wantErr: true,
		},
		// [Other test cases remain the same]
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a context with timeout for each test
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			lc, err := lsp.NewLanguageClient(tt.command, tt.args...)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}

			_, err = lc.Connect()
			if err != nil {
				t.Fatalf("could not connect receiver type: %v", err)
			}

			// Use the context to control the test timeout
			errChan := make(chan error, 1)
			go func() {
				errChan <- lc.SendRequest(tt.method, tt.params, tt.result, tt.timeout)
			}()

			select {
			case gotErr := <-errChan:
				if gotErr != nil {
					if !tt.wantErr {
						t.Errorf("SendRequest() failed: %v", gotErr)
					}

					return
				}

				if tt.wantErr {
					t.Fatal("SendRequest() succeeded unexpectedly")
				}
			case <-ctx.Done():
				t.Fatal("Test timed out")
			}
		})
	}
}
