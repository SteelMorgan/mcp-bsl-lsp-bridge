// lsp-proxy: stdio-to-TCP proxy for LSP servers
//
// This daemon:
// 1. Starts an LSP server (e.g., BSL LS) in stdio mode
// 2. Listens on a TCP port
// 3. Forwards LSP messages between TCP clients and the LSP server
//
// This allows the LSP server to be started once at container startup,
// index the workspace, and be ready to serve requests immediately.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var (
	port    = flag.Int("port", 9999, "TCP port to listen on")
	command = flag.String("command", "", "LSP server command to run")
)

func main() {
	flag.Parse()

	if *command == "" {
		log.Fatal("--command is required")
	}

	// All remaining arguments after flags are passed to the command
	cmdArgs := flag.Args()

	log.Printf("Starting lsp-proxy on port %d", *port)
	log.Printf("LSP command: %s %v", *command, cmdArgs)

	// Start LSP server process
	cmd := exec.Command(*command, cmdArgs...)

	// Get stdin/stdout pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to get stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get stdout pipe: %v", err)
	}

	// Forward stderr to our stderr
	cmd.Stderr = os.Stderr

	// Start the LSP server
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start LSP server: %v", err)
	}
	log.Printf("LSP server started with PID %d", cmd.Process.Pid)

	// Create the proxy
	proxy := NewLSPProxy(stdin, stdout)

	// Start TCP listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}
	defer listener.Close()
	log.Printf("Listening on port %d", *port)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		listener.Close()
		cmd.Process.Kill()
		os.Exit(0)
	}()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		log.Printf("Client connected: %s", conn.RemoteAddr())

		go proxy.HandleClient(conn)
	}
}

// LSPProxy manages communication between TCP clients and an LSP server
type LSPProxy struct {
	stdin  io.WriteCloser
	stdout io.Reader

	mu            sync.Mutex
	activeClient  net.Conn
	responseReady chan struct{}

	// Initialize state caching - LSP servers should only be initialized once
	initMu          sync.RWMutex
	initialized     bool
	initializeResp  []byte // Cached initialize response
}

// NewLSPProxy creates a new LSP proxy
func NewLSPProxy(stdin io.WriteCloser, stdout io.Reader) *LSPProxy {
	proxy := &LSPProxy{
		stdin:         stdin,
		stdout:        stdout,
		responseReady: make(chan struct{}, 1),
	}

	// Start reading responses from LSP server
	go proxy.readResponses()

	return proxy
}

// HandleClient handles a single TCP client connection
func (p *LSPProxy) HandleClient(conn net.Conn) {
	defer conn.Close()

	log.Printf("HandleClient: setting up for %s", conn.RemoteAddr())

	p.mu.Lock()
	p.activeClient = conn
	p.mu.Unlock()

	reader := bufio.NewReader(conn)

	for {
		log.Printf("HandleClient: waiting for LSP message...")
		// Read LSP message from client
		msg, err := readLSPMessage(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("Client read error: %v", err)
			} else {
				log.Printf("Client closed connection (EOF)")
			}
			break
		}

		// Extract body for analysis
		bodyStart := strings.Index(string(msg), "\r\n\r\n")
		var body string
		if bodyStart != -1 && len(msg) > bodyStart+4 {
			body = string(msg[bodyStart+4:])
		}

		// Log message content for debugging
		logBody := body
		if len(logBody) > 200 {
			logBody = logBody[:200] + "..."
		}
		log.Printf("-> LSP request: %d bytes, content: %s", len(msg), logBody)

		// Check if this is an initialize request
		if p.isInitializeRequest(body) {
			if p.handleInitializeRequest(conn, msg, body) {
				continue // Handled from cache, don't forward
			}
		}

		// Check if this is an initialized notification (just log it)
		if p.isInitializedNotification(body) {
			log.Printf("-> 'initialized' notification received")
			// Forward it - BSL LS expects this
		}

		// Forward to LSP server
		p.mu.Lock()
		_, err = p.stdin.Write(msg)
		p.mu.Unlock()

		if err != nil {
			log.Printf("LSP server write error: %v", err)
			break
		}
	}

	p.mu.Lock()
	if p.activeClient == conn {
		p.activeClient = nil
	}
	p.mu.Unlock()

	log.Printf("Client disconnected: %s", conn.RemoteAddr())
}

// isInitializeRequest checks if the message is an "initialize" request
func (p *LSPProxy) isInitializeRequest(body string) bool {
	var msg struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal([]byte(body), &msg); err != nil {
		return false
	}
	return msg.Method == "initialize"
}

// isInitializedNotification checks if the message is an "initialized" notification
func (p *LSPProxy) isInitializedNotification(body string) bool {
	var msg struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal([]byte(body), &msg); err != nil {
		return false
	}
	return msg.Method == "initialized"
}

// handleInitializeRequest handles initialize requests with caching
// Returns true if handled from cache (don't forward), false to forward normally
func (p *LSPProxy) handleInitializeRequest(conn net.Conn, msg []byte, body string) bool {
	p.initMu.RLock()
	initialized := p.initialized
	cachedResp := p.initializeResp
	p.initMu.RUnlock()

	if initialized && cachedResp != nil {
		// Return cached response
		log.Printf("-> CACHED: Returning cached initialize response (%d bytes)", len(cachedResp))
		
		// Extract request ID to match in response
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal([]byte(body), &req); err == nil {
			// Update the cached response with the new request ID
			var resp map[string]interface{}
			
			// Parse cached response body
			respBodyStart := strings.Index(string(cachedResp), "\r\n\r\n")
			if respBodyStart != -1 {
				respBody := cachedResp[respBodyStart+4:]
				if err := json.Unmarshal(respBody, &resp); err == nil {
					// Update ID
					resp["id"] = req.ID
					
					// Re-serialize
					newRespBody, err := json.Marshal(resp)
					if err == nil {
						newResp := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(newRespBody), newRespBody)
						conn.Write([]byte(newResp))
						log.Printf("<- CACHED: Sent cached response with updated ID")
						return true
					}
				}
			}
		}
		
		// Fallback: send cached response as-is
		conn.Write(cachedResp)
		return true
	}

	// First initialize - let it through, will be cached in readResponses
	log.Printf("-> FIRST initialize request, forwarding to LSP server")
	return false
}

// readResponses reads responses from LSP server and forwards to active client
func (p *LSPProxy) readResponses() {
	reader := bufio.NewReader(p.stdout)

	for {
		log.Printf("readResponses: waiting for LSP server response...")
		// Read LSP message from server
		msg, err := readLSPMessage(reader)
		if err != nil {
			log.Printf("LSP server read error: %v", err)
			return
		}

		// Extract body for logging and analysis
		bodyStart := strings.Index(string(msg), "\r\n\r\n")
		var body string
		var bodyPreview string
		if bodyStart != -1 && len(msg) > bodyStart+4 {
			body = string(msg[bodyStart+4:])
			if len(body) > 200 {
				bodyPreview = body[:200] + "..."
			} else {
				bodyPreview = body
			}
		}
		log.Printf("<- LSP response: %d bytes, content: %s", len(msg), bodyPreview)

		// Check if this is an initialize response (has "capabilities" in result)
		p.cacheInitializeResponseIfNeeded(msg, body)

		// Forward to active client
		p.mu.Lock()
		client := p.activeClient
		p.mu.Unlock()

		if client != nil {
			log.Printf("readResponses: forwarding to client %s", client.RemoteAddr())
			n, err := client.Write(msg)
			if err != nil {
				log.Printf("Client write error: %v", err)
			} else {
				log.Printf("readResponses: wrote %d bytes to client", n)
			}
		} else {
			log.Printf("readResponses: no active client to forward to!")
		}
	}
}

// cacheInitializeResponseIfNeeded checks if this is an initialize response and caches it
func (p *LSPProxy) cacheInitializeResponseIfNeeded(msg []byte, body string) {
	p.initMu.RLock()
	alreadyInitialized := p.initialized
	p.initMu.RUnlock()

	if alreadyInitialized {
		return
	}

	// Check if response has "result" with "capabilities" - this is an initialize response
	var resp struct {
		ID     interface{} `json:"id"`
		Result struct {
			Capabilities interface{} `json:"capabilities"`
		} `json:"result"`
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return
	}

	// If it has capabilities, it's an initialize response
	if resp.Result.Capabilities != nil {
		p.initMu.Lock()
		p.initialized = true
		p.initializeResp = make([]byte, len(msg))
		copy(p.initializeResp, msg)
		p.initMu.Unlock()
		log.Printf("CACHED: Initialize response cached (%d bytes), LSP server is now initialized", len(msg))
	}
}

// readLSPMessage reads a complete LSP message (with Content-Length header)
func readLSPMessage(reader *bufio.Reader) ([]byte, error) {
	// Read headers
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			// Empty line = end of headers
			break
		}

		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %v", err)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	// Reconstruct full message
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", contentLength)
	return append([]byte(header), body...), nil
}
