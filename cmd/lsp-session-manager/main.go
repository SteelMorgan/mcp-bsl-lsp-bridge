// LSP Session Manager - Persistent LSP session daemon
//
// This daemon:
// 1. Starts BSL Language Server once at container startup
// 2. Initializes LSP session and waits for indexing to complete
// 3. Keeps the session alive and ready for requests
// 4. Provides a simple JSON-RPC API for mcp-lsp-bridge to call LSP methods
//
// This solves the problem of repeated initialization - BSL LS indexes once,
// and all subsequent requests use the same initialized session.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"rockerboo/mcp-lsp-bridge/utils"
)

var (
	port         = flag.Int("port", 9999, "TCP port to listen on")
	command      = flag.String("command", "", "LSP server command to run")
	workspaceDir = flag.String("workspace", "/projects", "Workspace directory for LSP")
	statePath    = flag.String("state", "/var/lib/mcp-lsp-bridge/state.json", "Path to multi-project state.json (multi-project mode only)")
)

// JSONRPCID handles JSON-RPC 2.0 id field which can be string, number, or null
// Per spec: "An identifier established by the Client that MUST contain a String, Number, or NULL value"
type JSONRPCID struct {
	intValue *int64
	strValue *string
}

func (id *JSONRPCID) UnmarshalJSON(data []byte) error {
	// Try null first
	if string(data) == "null" {
		return nil
	}

	// Try int
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		id.intValue = &i
		return nil
	}

	// Try string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		id.strValue = &s
		// Also try to parse string as int (some servers send "123" instead of 123)
		if parsed, err := strconv.ParseInt(s, 10, 64); err == nil {
			id.intValue = &parsed
		}
		return nil
	}

	return fmt.Errorf("id must be string, number, or null, got: %s", string(data))
}

func (id JSONRPCID) MarshalJSON() ([]byte, error) {
	if id.intValue != nil {
		return json.Marshal(*id.intValue)
	}
	if id.strValue != nil {
		return json.Marshal(*id.strValue)
	}
	return []byte("null"), nil
}

// AsInt64 returns the id as int64 for map lookups (our internal pending map uses int64 keys)
func (id *JSONRPCID) AsInt64() int64 {
	if id.intValue != nil {
		return *id.intValue
	}
	return 0
}

// IsSet returns true if the id has a value
func (id *JSONRPCID) IsSet() bool {
	return id.intValue != nil || id.strValue != nil
}

func main() {
	flag.Parse()

	if *command == "" {
		log.Fatal("--command is required")
	}

	cmdArgs := flag.Args()

	log.Printf("Starting LSP Session Manager on port %d", *port)
	log.Printf("Workspace: %s", *workspaceDir)
	log.Printf("LSP command: %s %v", *command, cmdArgs)

	// Create session manager
	sm := NewSessionManager(*command, cmdArgs, *workspaceDir)

	// Multi-project mode: accumulative warm projects behind one BSL LS process.
	if os.Getenv("MULTI_PROJECT") == "1" {
		if err := sm.EnableMultiProject(*statePath); err != nil {
			log.Printf("Warning: failed to load multi-project state: %v", err)
		}
		log.Printf("Multi-project mode ENABLED (state: %s)", *statePath)
	}

	// Start LSP server and initialize session
	if err := sm.Start(); err != nil {
		log.Fatalf("Failed to start LSP session: %v", err)
	}

	// In multi-project mode, warm only the last-used project (lazy for the rest).
	if sm.multiProject {
		go sm.warmupBoot()
	}

	// Start TCP listener for API requests
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}
	defer listener.Close()
	log.Printf("API listening on port %d", *port)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		sm.Stop()
		listener.Close()
		os.Exit(0)
	}()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go sm.HandleClient(conn)
	}
}

// SessionManager manages a persistent LSP session
type SessionManager struct {
	command      string
	args         []string
	workspaceDir string

	// configRoot is the directory actually handed to BSL LS as the workspace
	// root. It equals workspaceDir unless auto-detection (single-project mode)
	// found the real 1C configuration root deeper inside the passed path — see
	// detectConfigurationRoot. Use rootDir() to read it with the fallback.
	configRoot string

	mu     sync.RWMutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader

	initialized  bool
	initResult   json.RawMessage
	capabilities json.RawMessage

	// serverProviders holds the set of *Provider keys the BSL LS advertised in
	// its initialize result (e.g. "callHierarchyProvider", "hoverProvider").
	// Used to gate request forwarding: a method whose provider the server did
	// NOT advertise must not be sent to the LSP process. BSL LS answers an
	// unadvertised method (textDocument/completion, textDocument/signatureHelp)
	// by throwing UnsupportedOperationException from the lsp4j default method,
	// which kills its single StreamMessageProducer.listen loop and poisons the
	// whole session — every subsequent request then hangs. Honouring the
	// advertised capabilities keeps the session alive for the methods that work.
	serverProviders map[string]bool

	// Request/response handling
	requestID int64
	pending   map[int64]chan lspResponse
	pendingMu sync.Mutex

	// Document tracking
	openDocs   map[string]bool
	openDocsMu sync.Mutex

	// Indexing progress tracking
	indexingMu             sync.RWMutex
	indexingActive         bool
	indexingTitle          string
	indexingMessage        string
	indexingCurrent        int
	indexingTotal          int
	indexingPercentage     int
	indexingStartedAt      time.Time // Start of current phase
	indexingFirstStartedAt time.Time // Start of first phase (for total elapsed time)
	indexingLastUpdate     time.Time
	indexingSpeed          float64 // files per second (rolling average)

	// File watcher for automatic didChangeWatchedFiles
	watcher        *fsnotify.Watcher
	watcherStop    chan struct{}
	pollingWatcher *PollingWatcher
	watcherMode    FileWatcherMode

	// Multi-project mode (MULTI_PROJECT=1). When disabled, pm is nil and the
	// daemon keeps its original single-workspace behavior.
	multiProject bool
	pm           *ProjectManager
	warmupMu     sync.Mutex // serializes project warm-ups so $/progress is attributable
	warmingMu    sync.RWMutex
	warming      string // root of the project currently being warmed (diagnostics)
}

type lspResponse struct {
	Result json.RawMessage
	Err    *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
}

// NewSessionManager creates a new session manager
func NewSessionManager(command string, args []string, workspaceDir string) *SessionManager {
	return &SessionManager{
		command:      command,
		args:         args,
		workspaceDir: workspaceDir,
		pending:      make(map[int64]chan lspResponse),
		openDocs:     make(map[string]bool),
	}
}

// rootDir returns the directory BSL LS treats as the workspace root: the
// auto-detected 1C configuration root when available, otherwise the raw
// workspace passed on the command line.
func (sm *SessionManager) rootDir() string {
	if sm.configRoot != "" {
		return sm.configRoot
	}
	return sm.workspaceDir
}

// detectConfigurationRoot finds the 1C configuration root beneath workspaceDir.
//
// The workspace is often passed as a PROJECT root that holds the real source
// tree deeper down (e.g. <project>/src/xml/Configuration.xml) next to extension
// sub-configurations (<project>/src/exts/*/Configuration.xml) and scratch copies
// under tasks/. BSL LS, pointed at such a parent, discovers every
// Configuration.xml and registers the extensions' common modules while the MAIN
// configuration's modules drop out of the type index — breaking cross-module
// resolution (signature help, call hierarchy) and producing false
// QueryToMissingMetadata on objects that do exist. We therefore descend to the
// single MAIN configuration root and hand that to BSL LS instead of the parent.
//
// A main configuration is a root Configuration.xml WITHOUT the
// <ConfigurationExtensionPurpose> marker that every extension carries near the
// top of its file. Returns (root, true) when the detected root differs from
// workspaceDir; on any ambiguity (no main config found, or more than one at the
// shallowest depth is handled by a deterministic tie-break) it falls back to
// workspaceDir so the original behavior is preserved.
func detectConfigurationRoot(workspaceDir string) (string, bool) {
	base := filepath.Clean(workspaceDir)

	// Already a main configuration root? keep it (handles a correctly-pointed
	// workspace, e.g. .../src/xml).
	if isMainConfigurationFile(filepath.Join(base, "Configuration.xml")) {
		return base, false
	}

	const maxDepth = 5 // <project>/src/xml/Configuration.xml is depth 2; keep slack
	var candidates []string
	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			if path == base {
				return nil
			}
			name := info.Name()
			// Hidden, dependency and scratch directories are never config roots;
			// pruning them also keeps the scratch Configuration.xml copies under
			// tasks/ and .review-sandboxes/ out of the candidate set.
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" ||
				name == "tasks" || name == "build" || name == "out" || name == "bin" {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(base, path)
			if strings.Count(rel, string(os.PathSeparator))+1 > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "Configuration.xml" && isMainConfigurationFile(path) {
			candidates = append(candidates, filepath.Dir(path))
		}
		return nil
	})

	if len(candidates) == 0 {
		return base, false
	}

	// Prefer the shallowest candidate — the base configuration sits above any
	// nested copy. Deterministic lexical tie-break for stability.
	best := candidates[0]
	bestDepth := strings.Count(best, string(os.PathSeparator))
	for _, c := range candidates[1:] {
		d := strings.Count(c, string(os.PathSeparator))
		if d < bestDepth || (d == bestDepth && c < best) {
			best, bestDepth = c, d
		}
	}
	return best, best != base
}

// isMainConfigurationFile reports whether path is a root Configuration.xml of a
// MAIN 1C configuration (not an extension). Only the head of the file is read:
// the distinguishing <ConfigurationExtensionPurpose> property lives in the
// Properties block near the top, well before the large ChildObjects list.
func isMainConfigurationFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 128*1024)
	n, _ := io.ReadFull(f, buf) // short read on small files is fine; n is exact
	head := string(buf[:n])

	if !strings.Contains(head, "<Configuration ") && !strings.Contains(head, "<Configuration>") {
		return false // not a configuration root descriptor
	}
	// Extensions carry this marker; the base configuration never does.
	return !strings.Contains(head, "ConfigurationExtensionPurpose")
}

// Start starts the LSP server and initializes the session
func (sm *SessionManager) Start() error {
	log.Println("Starting LSP server...")

	// Resolve the real configuration root under the passed workspace (single
	// project only — multi-project manages roots per project via ProjectManager).
	sm.configRoot = sm.workspaceDir
	if !sm.multiProject {
		if root, changed := detectConfigurationRoot(sm.workspaceDir); changed {
			log.Printf("Detected 1C configuration root: %s (passed workspace: %s)", root, sm.workspaceDir)
			sm.configRoot = root
		}
	}

	sm.cmd = exec.Command(sm.command, sm.args...)

	var err error
	sm.stdin, err = sm.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	sm.stdout, err = sm.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	sm.cmd.Stderr = os.Stderr

	if err := sm.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start LSP server: %w", err)
	}
	log.Printf("LSP server started with PID %d", sm.cmd.Process.Pid)

	// Start response reader
	go sm.readResponses()

	// Initialize LSP session
	if err := sm.initialize(); err != nil {
		return fmt.Errorf("failed to initialize LSP session: %w", err)
	}

	// Start file watcher AFTER indexing completes to avoid resource contention
	go sm.startFileWatcherAfterIndexing()

	return nil
}

// startFileWatcherAfterIndexing waits for indexing to complete, then starts file watcher
func (sm *SessionManager) startFileWatcherAfterIndexing() {
	// Wait for indexing to complete (check every 5 seconds)
	for {
		time.Sleep(5 * time.Second)

		sm.indexingMu.RLock()
		isActive := sm.indexingActive
		sm.indexingMu.RUnlock()

		if !isActive {
			break
		}
		log.Println("File watcher: waiting for indexing to complete...")
	}

	log.Println("File watcher: indexing complete, starting watcher")

	// Start file watcher for automatic didChangeWatchedFiles
	if err := sm.startFileWatcher(); err != nil {
		log.Printf("Warning: failed to start file watcher: %v", err)
		// Non-fatal - continue without file watching
	}
}

// Stop stops the LSP server
func (sm *SessionManager) Stop() {
	// Stop file watchers
	if sm.pollingWatcher != nil {
		sm.pollingWatcher.Stop()
	}
	if sm.watcherStop != nil {
		close(sm.watcherStop)
	}
	if sm.watcher != nil {
		sm.watcher.Close()
	}

	if sm.cmd != nil && sm.cmd.Process != nil {
		sm.sendNotification("exit", nil)
		sm.cmd.Process.Kill()
	}
}

// startFileWatcher starts watching workspace for .bsl and .os file changes
func (sm *SessionManager) startFileWatcher() error {
	sm.watcherMode = GetFileWatcherMode()
	log.Printf("File watcher mode: %s", sm.watcherMode)

	switch sm.watcherMode {
	case WatcherModeOff:
		log.Println("File watcher disabled - use did_change_watched_files tool manually")
		return nil

	case WatcherModePolling:
		return sm.startPollingWatcher()

	case WatcherModeFsnotify:
		return sm.startFsnotifyWatcher()

	case WatcherModeAuto:
		// Try fsnotify first, fallback to polling if it doesn't detect changes
		// For now, on Docker/Windows, fsnotify won't work, so we detect and use polling
		if err := sm.startFsnotifyWatcher(); err != nil {
			log.Printf("fsnotify failed (%v), falling back to polling", err)
			return sm.startPollingWatcher()
		}
		return nil

	default:
		log.Printf("Unknown watcher mode '%s', using polling", sm.watcherMode)
		return sm.startPollingWatcher()
	}
}

// startPollingWatcher запускает polling-based file watcher
func (sm *SessionManager) startPollingWatcher() error {
	interval := GetPollingInterval()
	workers := GetPollingWorkers()

	sm.pollingWatcher = NewPollingWatcher(
		sm.rootDir(),
		interval,
		workers,
		func(changes []FileChange) error {
			// Convert to LSP format and send notification
			lspChanges := make([]map[string]interface{}, len(changes))
			for i, c := range changes {
				lspChanges[i] = map[string]interface{}{
					"uri":  c.URI,
					"type": c.Type,
				}
			}
			params := map[string]interface{}{
				"changes": lspChanges,
			}
			return sm.sendNotification("workspace/didChangeWatchedFiles", params)
		},
		sm.IsIndexing, // Pass indexing check function
	)

	return sm.pollingWatcher.Start()
}

// IsIndexing returns true if LSP is currently indexing
func (sm *SessionManager) IsIndexing() bool {
	sm.indexingMu.RLock()
	defer sm.indexingMu.RUnlock()
	return sm.indexingActive
}

// startFsnotifyWatcher запускает fsnotify-based file watcher
func (sm *SessionManager) startFsnotifyWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	sm.watcher = watcher
	sm.watcherStop = make(chan struct{})

	// Add workspace directory recursively
	err = filepath.Walk(sm.rootDir(), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			// Skip hidden directories and common non-source directories
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			if err := watcher.Add(path); err != nil {
				log.Printf("Warning: failed to watch directory %s: %v", path, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk workspace: %w", err)
	}

	log.Printf("fsnotify watcher started for workspace: %s", sm.rootDir())

	// Start watcher goroutine
	go sm.runFsnotifyWatcher()

	return nil
}

// runFsnotifyWatcher processes file system events from fsnotify
func (sm *SessionManager) runFsnotifyWatcher() {
	// Debounce events - collect changes over 500ms before sending
	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}
	pendingChanges := make(map[string]int) // uri -> FileChangeType (1=Created, 2=Changed, 3=Deleted)
	var pendingMu sync.Mutex

	for {
		select {
		case <-sm.watcherStop:
			log.Println("fsnotify watcher stopped")
			return

		case event, ok := <-sm.watcher.Events:
			if !ok {
				return
			}

			// Only process .bsl and .os files
			ext := strings.ToLower(filepath.Ext(event.Name))
			if ext != ".bsl" && ext != ".os" {
				// Check if it's a new directory to watch
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						name := info.Name()
						if !strings.HasPrefix(name, ".") && name != "node_modules" && name != "vendor" {
							sm.watcher.Add(event.Name)
							log.Printf("Added new directory to watch: %s", event.Name)
						}
					}
				}
				continue
			}

			// Convert to file URI
			uri := "file://" + filepath.ToSlash(event.Name)

			pendingMu.Lock()
			// Map fsnotify events to LSP FileChangeType
			switch {
			case event.Has(fsnotify.Create):
				pendingChanges[uri] = 1 // Created
				log.Printf("File created: %s", event.Name)
			case event.Has(fsnotify.Write):
				// Only mark as changed if not already marked as created
				if _, exists := pendingChanges[uri]; !exists {
					pendingChanges[uri] = 2 // Changed
				}
			case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
				pendingChanges[uri] = 3 // Deleted
				log.Printf("File deleted/renamed: %s", event.Name)
			}
			pendingMu.Unlock()

			// Reset debounce timer
			debounceTimer.Reset(500 * time.Millisecond)

		case <-debounceTimer.C:
			pendingMu.Lock()
			if len(pendingChanges) > 0 {
				// Build FileEvent array
				changes := make([]map[string]interface{}, 0, len(pendingChanges))
				for uri, changeType := range pendingChanges {
					changes = append(changes, map[string]interface{}{
						"uri":  uri,
						"type": changeType,
					})
				}
				pendingChanges = make(map[string]int) // Clear pending
				pendingMu.Unlock()

				// Send didChangeWatchedFiles notification
				params := map[string]interface{}{
					"changes": changes,
				}
				if err := sm.sendNotification("workspace/didChangeWatchedFiles", params); err != nil {
					log.Printf("Error sending didChangeWatchedFiles: %v", err)
				} else {
					log.Printf("Sent didChangeWatchedFiles with %d changes", len(changes))
				}
			} else {
				pendingMu.Unlock()
			}

		case err, ok := <-sm.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("fsnotify watcher error: %v", err)
		}
	}
}

// initialize sends initialize request and waits for response
func (sm *SessionManager) initialize() error {
	log.Println("Initializing LSP session...")

	rootURI := utils.FilePathToURI(sm.rootDir())

	// Build workspace folders.
	//
	// In multi-project mode we start with NO workspace folders and a null
	// rootUri so BSL LS does not eagerly index the mounted parent directory
	// (`/projects`) as a single configuration. Projects are added later, one at
	// a time, via workspace/didChangeWorkspaceFolders. Guards invariant #2 from
	// the design (avoid the BSLLanguageServer rootUri/rootPath fallback).
	workspaceFolders := []map[string]string{
		{
			//"uri":  "file://" + sm.workspaceDir,
			"uri":  rootURI,
			"name": "workspace",
		},
	}
	if sm.multiProject {
		workspaceFolders = []map[string]string{}
		rootURI = ""
	}

	params := map[string]interface{}{
		"processId": nil, // Don't monitor parent process
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"hover": map[string]interface{}{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"definition": map[string]interface{}{
					"linkSupport": true,
				},
				"references":     map[string]interface{}{},
				"callHierarchy":  map[string]interface{}{},
				"documentSymbol": map[string]interface{}{},
				"diagnostic":     map[string]interface{}{},
				"signatureHelp": map[string]interface{}{
					"contextSupport": true,
				},
				"completion": map[string]interface{}{
					"completionItem": map[string]interface{}{
						"snippetSupport":      false,
						"documentationFormat": []string{"markdown", "plaintext"},
						"labelDetailsSupport": true,
					},
				},
				"selectionRange": map[string]interface{}{},
				"inlayHint":      map[string]interface{}{},
				"semanticTokens": map[string]interface{}{
					"requests": map[string]interface{}{
						"range": true,
						"full":  true,
					},
					"tokenTypes": []string{
						"namespace", "type", "class", "enum", "interface", "struct",
						"typeParameter", "parameter", "variable", "property", "enumMember",
						"event", "function", "method", "macro", "keyword", "modifier",
						"comment", "string", "number", "regexp", "operator",
					},
					"tokenModifiers": []string{
						"declaration", "definition", "readonly", "static", "deprecated",
						"abstract", "async", "modification", "documentation", "defaultLibrary",
					},
					"formats": []string{"relative"},
				},
			},
			"workspace": map[string]interface{}{
				"workspaceFolders": true,
			},
			"window": map[string]interface{}{
				"workDoneProgress": true, // Enable $/progress notifications
			},
		},
		//"rootUri":          "file://" + sm.workspaceDir,
		"rootUri":          rootURI,
		"workspaceFolders": workspaceFolders,
	}
	if rootURI == "" {
		// Explicit null rootUri (multi-project): nothing to index at init time.
		params["rootUri"] = nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := sm.sendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	sm.mu.Lock()
	sm.initResult = result
	sm.initialized = true
	sm.mu.Unlock()

	// Extract capabilities
	var initResp struct {
		Capabilities json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(result, &initResp); err == nil {
		sm.mu.Lock()
		sm.capabilities = initResp.Capabilities
		// Record which *Provider keys the server advertised, so handleAPIRequest
		// can refuse unadvertised methods instead of forwarding them and
		// poisoning the session (see serverProviders doc on the struct field).
		providers := map[string]bool{}
		var capsMap map[string]json.RawMessage
		if json.Unmarshal(initResp.Capabilities, &capsMap) == nil {
			for key, val := range capsMap {
				if !strings.HasSuffix(key, "Provider") {
					continue
				}
				// A provider counts as present unless it is explicitly false/null.
				if s := strings.TrimSpace(string(val)); s != "false" && s != "null" {
					providers[key] = true
				}
			}
		}
		sm.serverProviders = providers
		sm.mu.Unlock()
		log.Printf("Server advertised %d providers: %v", len(providers), providersList(providers))
	}

	log.Println("LSP session initialized successfully")

	// Send initialized notification
	if err := sm.sendNotification("initialized", map[string]interface{}{}); err != nil {
		log.Printf("Warning: failed to send initialized notification: %v", err)
	}

	log.Println("Waiting for indexing to complete...")
	// Give BSL LS time to index - we'll track progress via $/progress notifications
	time.Sleep(5 * time.Second)

	return nil
}

// sendRequest sends an LSP request and waits for response
func (sm *SessionManager) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	sm.pendingMu.Lock()
	sm.requestID++
	id := sm.requestID
	respCh := make(chan lspResponse, 1)
	sm.pending[id] = respCh
	sm.pendingMu.Unlock()

	defer func() {
		sm.pendingMu.Lock()
		delete(sm.pending, id)
		sm.pendingMu.Unlock()
	}()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}

	if err := sm.writeMessage(req); err != nil {
		return nil, err
	}

	select {
	case resp := <-respCh:
		if resp.Err != nil {
			return nil, fmt.Errorf("lsp error %d: %s", resp.Err.Code, resp.Err.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sendNotification sends an LSP notification (no response expected)
func (sm *SessionManager) sendNotification(method string, params interface{}) error {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	return sm.writeMessage(req)
}

// writeMessage writes an LSP message to the server
func (sm *SessionManager) writeMessage(msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.stdin == nil {
		return fmt.Errorf("LSP stdin is not connected")
	}

	if _, err := sm.stdin.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := sm.stdin.Write(body); err != nil {
		return err
	}
	return nil
}

// readResponses reads responses from LSP server
func (sm *SessionManager) readResponses() {
	reader := bufio.NewReader(sm.stdout)

	for {
		msg, err := readLSPMessage(reader)
		if err != nil {
			log.Printf("LSP read error: %v", err)
			return
		}

		// Parse message
		var baseMsg struct {
			ID     *JSONRPCID      `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal(msg, &baseMsg); err != nil {
			log.Printf("Failed to parse LSP message: %v (raw: %s)", err, string(msg)[:min(200, len(msg))])
			continue
		}

		// Handle response (has id, no method)
		if baseMsg.ID != nil && baseMsg.ID.IsSet() && baseMsg.Method == "" {
			idInt := baseMsg.ID.AsInt64()
			sm.pendingMu.Lock()
			if ch, ok := sm.pending[idInt]; ok {
				ch <- lspResponse{
					Result: baseMsg.Result,
					Err:    baseMsg.Error,
				}
			}
			sm.pendingMu.Unlock()
			continue
		}

		// Handle notification (no id)
		if baseMsg.Method != "" {
			sm.handleNotification(baseMsg.Method, msg)
		}
	}
}

// handleNotification handles LSP notifications from server
func (sm *SessionManager) handleNotification(method string, msg []byte) {
	switch method {
	case "$/progress":
		// Log and track progress updates
		var progress struct {
			Params struct {
				Token string `json:"token"`
				Value struct {
					Kind       string `json:"kind"`
					Title      string `json:"title"`
					Message    string `json:"message"`
					Percentage int    `json:"percentage"`
				} `json:"value"`
			} `json:"params"`
		}
		if json.Unmarshal(msg, &progress) == nil {
			kind := progress.Params.Value.Kind
			title := progress.Params.Value.Title
			message := progress.Params.Value.Message
			percentage := progress.Params.Value.Percentage

			if kind != "" {
				log.Printf("Progress [%s]: %s %s (%d%%)", kind, title, message, percentage)
			}

			// Update indexing state
			sm.indexingMu.Lock()
			now := time.Now()

			switch kind {
			case "begin":
				sm.indexingActive = true
				sm.indexingTitle = title
				sm.indexingMessage = message
				sm.indexingPercentage = percentage
				sm.indexingStartedAt = now
				// Reset counters only for new indexing cycle (> 30s since last update)
				// This preserves progress across multiple phases of the same indexing
				isNewCycle := sm.indexingFirstStartedAt.IsZero() || now.Sub(sm.indexingLastUpdate) > 30*time.Second
				if isNewCycle {
					sm.indexingFirstStartedAt = now
					sm.indexingCurrent = 0
					sm.indexingTotal = 0
					sm.indexingSpeed = 0
				}
				sm.indexingLastUpdate = now

			case "report":
				sm.indexingMessage = message
				sm.indexingPercentage = percentage

				// Parse "N/M файлов" format
				current, total := parseProgressMessage(message)
				if total > 0 {
					// Calculate speed (files per second) with rolling average
					if sm.indexingCurrent > 0 && current > sm.indexingCurrent {
						elapsed := now.Sub(sm.indexingLastUpdate).Seconds()
						if elapsed > 0 {
							instantSpeed := float64(current-sm.indexingCurrent) / elapsed
							if sm.indexingSpeed > 0 {
								// Rolling average: 70% old + 30% new
								sm.indexingSpeed = sm.indexingSpeed*0.7 + instantSpeed*0.3
							} else {
								sm.indexingSpeed = instantSpeed
							}
						}
					}
					sm.indexingCurrent = current
					sm.indexingTotal = total
				}
				sm.indexingLastUpdate = now

			case "end":
				sm.indexingActive = false
				sm.indexingMessage = message
				if message != "" {
					sm.indexingTitle = message // "Наполнение контекста завершено."
				}
				sm.indexingPercentage = 100
				if sm.indexingTotal > 0 {
					sm.indexingCurrent = sm.indexingTotal
				}
				sm.indexingLastUpdate = now
				// Keep indexingFirstStartedAt for displaying total elapsed time
			}
			sm.indexingMu.Unlock()
		}

	case "textDocument/publishDiagnostics":
		// Could cache diagnostics here

	case "window/logMessage":
		// Log server messages
		var logMsg struct {
			Params struct {
				Type    int    `json:"type"`
				Message string `json:"message"`
			} `json:"params"`
		}
		if json.Unmarshal(msg, &logMsg) == nil {
			log.Printf("LSP Log [type=%d]: %s", logMsg.Params.Type, logMsg.Params.Message)
		}

	default:
		log.Printf("Notification: %s", method)
	}
}

// parseProgressMessage extracts current/total from "N/M файлов" format
func parseProgressMessage(msg string) (current, total int) {
	// Try to parse "123/456 файлов" or similar formats
	var c, t int
	// Match patterns like "123/456" anywhere in the message
	for i := 0; i < len(msg); i++ {
		if msg[i] >= '0' && msg[i] <= '9' {
			// Found start of number, try to parse "N/M"
			n, err := fmt.Sscanf(msg[i:], "%d/%d", &c, &t)
			if err == nil && n == 2 && t > 0 {
				return c, t
			}
		}
	}
	return 0, 0
}

// HandleClient handles an API client connection
func (sm *SessionManager) HandleClient(conn net.Conn) {
	defer conn.Close()
	log.Printf("API client connected: %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)

	for {
		// Read JSON-RPC request (newline-delimited)
		log.Printf("Waiting for request from %s...", conn.RemoteAddr())
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Client read error: %v", err)
			} else {
				log.Printf("Client %s closed connection (EOF)", conn.RemoteAddr())
			}
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		log.Printf("Received request from %s: %s", conn.RemoteAddr(), line)

		// Parse request
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int64           `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}

		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("Parse error for request: %v", err)
			sm.sendAPIError(conn, 0, -32700, "Parse error")
			continue
		}

		log.Printf("Handling method: %s (id=%d)", req.Method, req.ID)

		// Handle request
		result, err := sm.handleAPIRequest(req.Method, req.Params)
		if err != nil {
			log.Printf("Error handling %s: %v", req.Method, err)
			sm.sendAPIError(conn, req.ID, -32603, err.Error())
			continue
		}

		log.Printf("Method %s completed successfully", req.Method)

		// Send response
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}
		respJSON, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error marshaling response: %v", err)
			continue
		}
		n, err := conn.Write(append(respJSON, '\n'))
		if err != nil {
			log.Printf("Error writing response: %v", err)
		} else {
			log.Printf("Sent response to %s: %d bytes (id=%d)", conn.RemoteAddr(), n, req.ID)
		}
	}

	log.Printf("API client disconnected: %s", conn.RemoteAddr())
}

// sendAPIError sends an error response to API client
func (sm *SessionManager) sendAPIError(conn net.Conn, id int64, code int, message string) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	respJSON, _ := json.Marshal(resp)
	conn.Write(append(respJSON, '\n'))
}

// methodProvider maps an LSP method to the serverCapabilities *Provider key that
// must be advertised for the method to be safe to forward to BSL LS. An empty
// string means "do not gate" (session-local, notification-style, or workspace
// methods). Forwarding a method whose provider is absent makes BSL LS throw
// UnsupportedOperationException and poison the session, so these are refused.
func methodProvider(method string) string {
	switch method {
	case "textDocument/completion":
		return "completionProvider"
	case "textDocument/signatureHelp":
		return "signatureHelpProvider"
	case "textDocument/hover":
		return "hoverProvider"
	case "textDocument/definition":
		return "definitionProvider"
	case "textDocument/references":
		return "referencesProvider"
	case "textDocument/documentSymbol":
		return "documentSymbolProvider"
	case "textDocument/implementation":
		return "implementationProvider"
	case "textDocument/codeAction":
		return "codeActionProvider"
	case "textDocument/codeLens", "codeLens/resolve":
		return "codeLensProvider"
	case "textDocument/formatting":
		return "documentFormattingProvider"
	case "textDocument/rangeFormatting":
		return "documentRangeFormattingProvider"
	case "textDocument/rename", "textDocument/prepareRename":
		return "renameProvider"
	case "textDocument/prepareCallHierarchy",
		"callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
		return "callHierarchyProvider"
	case "textDocument/selectionRange":
		return "selectionRangeProvider"
	case "textDocument/semanticTokens/range", "textDocument/semanticTokens/full":
		return "semanticTokensProvider"
	case "textDocument/inlayHint":
		return "inlayHintProvider"
	case "textDocument/diagnostic":
		return "diagnosticProvider"
	}
	return ""
}

// providersList returns the provider keys as a slice (for logging).
func providersList(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// handleAPIRequest handles an API request from mcp-lsp-bridge
func (sm *SessionManager) handleAPIRequest(method string, params json.RawMessage) (interface{}, error) {
	timeout := 90 * time.Second
	switch method {
	// Long operations
	case "workspace/diagnostic":
		timeout = 10 * time.Minute
	case "textDocument/diagnostic", "textDocument/formatting":
		timeout = 5 * time.Minute
	case "textDocument/rename", "textDocument/prepareRename":
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Capability gate: refuse any method whose provider the server did not
	// advertise, instead of forwarding it and poisoning the session with an
	// UnsupportedOperationException (see SessionManager.serverProviders). Gate
	// only when we actually parsed providers (fail-open if the set is empty/nil).
	if prov := methodProvider(method); prov != "" {
		sm.mu.RLock()
		gated := len(sm.serverProviders) > 0 && !sm.serverProviders[prov]
		sm.mu.RUnlock()
		if gated {
			return nil, fmt.Errorf("method %s is not supported by the language server (no %s advertised)", method, prov)
		}
	}

	switch method {
	case "session/status":
		return sm.getStatus(), nil

	case "project/add", "project/close", "project/list", "project/status":
		return sm.handleProjectAPI(method, params)

	case "session/capabilities":
		sm.mu.RLock()
		caps := sm.capabilities
		sm.mu.RUnlock()
		return caps, nil

	case "textDocument/didOpen":
		return sm.handleDidOpen(params)

	case "textDocument/didClose":
		return sm.handleDidClose(params)

	case "textDocument/hover",
		"textDocument/definition",
		"textDocument/references",
		"textDocument/documentSymbol",
		"textDocument/diagnostic",
		"textDocument/implementation",
		"textDocument/codeAction",
		"textDocument/formatting",
		"textDocument/rename",
		"textDocument/prepareRename",
		"textDocument/prepareCallHierarchy",
		// Platform-aware features enabled with BSL LS type-system v2 / bsl-context.
		"textDocument/signatureHelp",
		"textDocument/completion",
		"textDocument/selectionRange",
		"textDocument/inlayHint",
		// CodeLens for complexity metrics (cyclomatic/cognitive). Two-step:
		// textDocument/codeLens returns range+data, codeLens/resolve fills command.title.
		"textDocument/codeLens",
		"codeLens/resolve",
		"textDocument/semanticTokens/range",
		"textDocument/semanticTokens/full":
		// Forward directly to LSP server
		var p interface{}
		json.Unmarshal(params, &p)
		start := time.Now()
		res, err := sm.sendRequest(ctx, method, p)
		log.Printf("Method %s finished in %s (err=%v)", method, time.Since(start), err)
		return res, err

	case "callHierarchy/incomingCalls",
		"callHierarchy/outgoingCalls":
		var p interface{}
		json.Unmarshal(params, &p)
		return sm.sendRequest(ctx, method, p)

	case "workspace/didChangeWatchedFiles":
		// Notification (no result) in LSP, but our API is request/response.
		// Forward as notification to the underlying LSP server and return an "ok" ack.
		var p interface{}
		json.Unmarshal(params, &p)
		start := time.Now()
		err := sm.sendNotification(method, p)
		log.Printf("Notification %s sent in %s (err=%v)", method, time.Since(start), err)
		return map[string]interface{}{"ok": err == nil}, err

	case "workspace/symbol":
		var p interface{}
		json.Unmarshal(params, &p)
		start := time.Now()
		res, err := sm.sendRequest(ctx, method, p)
		log.Printf("Method %s finished in %s (err=%v)", method, time.Since(start), err)
		return res, err

	case "workspace/diagnostic":
		var p interface{}
		json.Unmarshal(params, &p)
		start := time.Now()
		res, err := sm.sendRequest(ctx, method, p)
		log.Printf("Method %s finished in %s (err=%v)", method, time.Since(start), err)
		return res, err

	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

// getStatus returns current session status
func (sm *SessionManager) getStatus() map[string]interface{} {
	sm.mu.RLock()
	initialized := sm.initialized
	sm.mu.RUnlock()

	sm.openDocsMu.Lock()
	openDocsCount := len(sm.openDocs)
	sm.openDocsMu.Unlock()

	// Get indexing progress (minimal structure)
	sm.indexingMu.RLock()

	// Determine state: "idle" | "indexing" | "complete"
	isActive := sm.indexingActive || (sm.indexingTotal > 0 && sm.indexingCurrent < sm.indexingTotal)
	isComplete := sm.indexingTotal > 0 && sm.indexingCurrent >= sm.indexingTotal

	state := "idle"
	if isActive {
		state = "indexing"
	} else if isComplete {
		state = "complete"
	}

	indexing := map[string]interface{}{
		"state":   state,
		"current": sm.indexingCurrent,
		"total":   sm.indexingTotal,
		"message": sm.indexingMessage,
	}

	// Add ETA only during indexing
	if isActive && sm.indexingSpeed > 0 && sm.indexingTotal > sm.indexingCurrent {
		remaining := sm.indexingTotal - sm.indexingCurrent
		indexing["eta_seconds"] = int(float64(remaining) / sm.indexingSpeed)
	}

	// Add elapsed time (using first start time for total duration across all phases)
	startTime := sm.indexingFirstStartedAt
	if startTime.IsZero() {
		startTime = sm.indexingStartedAt
	}
	if !startTime.IsZero() {
		if isActive {
			indexing["elapsed_seconds"] = int(time.Since(startTime).Seconds())
		} else if isComplete {
			indexing["elapsed_seconds"] = int(sm.indexingLastUpdate.Sub(startTime).Seconds())
		}
	}
	sm.indexingMu.RUnlock()

	status := map[string]interface{}{
		"initialized":   initialized,
		"openDocuments": openDocsCount,
		"pid":           sm.cmd.Process.Pid,
		"indexing":      indexing,
		"multiProject":  sm.multiProject,
	}

	// In multi-project mode, attach the per-project registry so the bridge can
	// gate readiness per project instead of relying on the single global state.
	if sm.pm != nil {
		projects := sm.pm.List()
		projList := make([]map[string]interface{}, 0, len(projects))
		for _, p := range projects {
			projList = append(projList, map[string]interface{}{
				"root":      p.Root,
				"state":     string(p.State),
				"last_used": p.LastUsed,
			})
		}
		status["projects"] = projList
		status["last_project"] = sm.pm.LastProject()
	}

	return status
}

// handleDidOpen handles textDocument/didOpen
func (sm *SessionManager) handleDidOpen(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI        string `json:"uri"`
			LanguageID string `json:"languageId"`
			Version    int    `json:"version"`
			Text       string `json:"text"`
		} `json:"textDocument"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	sm.openDocsMu.Lock()
	alreadyOpen := sm.openDocs[p.TextDocument.URI]
	sm.openDocs[p.TextDocument.URI] = true
	sm.openDocsMu.Unlock()

	if alreadyOpen {
		// Document already open - close and reopen to refresh content
		closeParams := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": p.TextDocument.URI,
			},
		}
		if err := sm.sendNotification("textDocument/didClose", closeParams); err != nil {
			return nil, fmt.Errorf("failed to close document for refresh: %w", err)
		}
	}

	// Send to LSP server (either first open or reopen after close)
	return nil, sm.sendNotification("textDocument/didOpen", p)
}

// handleDidClose handles textDocument/didClose
func (sm *SessionManager) handleDidClose(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	sm.openDocsMu.Lock()
	delete(sm.openDocs, p.TextDocument.URI)
	sm.openDocsMu.Unlock()

	return nil, sm.sendNotification("textDocument/didClose", p)
}

// readLSPMessage reads a complete LSP message
func readLSPMessage(reader *bufio.Reader) ([]byte, error) {
	var contentLength int

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
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

	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
