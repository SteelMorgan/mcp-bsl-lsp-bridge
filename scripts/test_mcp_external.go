package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// MCPTestClient represents an external MCP client for testing
type MCPTestClient struct {
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	requestID    int
	capabilities *mcp.ServerCapabilities
}

// TestResult represents the result of a test
type TestResult struct {
	TestName string
	Success  bool
	Error    string
	Duration time.Duration
	Response any
}

// NewMCPTestClient creates a new external MCP test client
func NewMCPTestClient() (*MCPTestClient, error) {
	// Build the project first
	buildCmd := exec.Command("go", "build", "-o", "mcp-lsp-bridge", ".")
	buildCmd.Dir = ".."
	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to build project: %v", err)
	}

	// Start the MCP server process
	cmd := exec.Command("../mcp-lsp-bridge")
	cmd.Dir = "."

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start MCP server: %v", err)
	}

	client := &MCPTestClient{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		requestID: 1,
	}

	// Initialize the client
	if err := client.initialize(); err != nil {
		closeErr := client.Close()
		if closeErr != nil {
			log.Printf("failed to close client: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to initialize client: %v", err)
	}

	return client, nil
}

// Close closes the MCP client and terminates the server process
func (c *MCPTestClient) Close() error {
	if c.stdin != nil {
		err := c.stdin.Close()
		if err != nil {
			return fmt.Errorf("failed to close stdin: %v", err)
		}
	}
	if c.stdout != nil {
		err := c.stdout.Close()
		if err != nil {
			return fmt.Errorf("failed to close stdout: %v", err)
		}
	}
	if c.stderr != nil {
		err := c.stderr.Close()
		if err != nil {
			return fmt.Errorf("failed to close stderr: %v", err)
		}
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// sendRequest sends a JSON-RPC request to the MCP server
func (c *MCPTestClient) sendRequest(method string, params any) (map[string]any, error) {
	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.requestID,
		"method":  method,
		"params":  params,
	}
	c.requestID++

	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send request
	if _, err := c.stdin.Write(append(requestData, '\n')); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	// Read response
	response := make([]byte, 4096)
	n, err := c.stdout.Read(response)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(response[:n], &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if errorVal, exists := result["error"]; exists {
		return nil, fmt.Errorf("server returned error: %v", errorVal)
	}

	return result, nil
}

// initialize initializes the MCP client
func (c *MCPTestClient) initialize() error {
	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"clientInfo": map[string]any{
			"name":    "mcp-test-client",
			"version": "1.0.0",
		},
	}

	response, err := c.sendRequest("initialize", initParams)
	if err != nil {
		return fmt.Errorf("initialize failed: %v", err)
	}

	// Store server capabilities
	if result, ok := response["result"].(map[string]interface{}); ok {
		if caps, ok := result["capabilities"].(map[string]interface{}); ok {
			capData, _ := json.Marshal(caps)
			_ = json.Unmarshal(capData, &c.capabilities)
		}
	}

	// Send initialized notification
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	notificationData, _ := json.Marshal(notification)
	_, _ = c.stdin.Write(append(notificationData, '\n'))

	return nil
}

// testToolCall tests calling a specific MCP tool
func (c *MCPTestClient) testToolCall(toolName string, arguments map[string]interface{}) (*TestResult, error) {
	start := time.Now()

	params := map[string]interface{}{
		"name":      toolName,
		"arguments": arguments,
	}

	response, err := c.sendRequest("tools/call", params)
	duration := time.Since(start)

	result := &TestResult{
		TestName: "Tool: " + toolName,
		Duration: duration,
		Response: response,
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}

	result.Success = true
	return result, nil
}

// testListTools tests listing available tools
func (c *MCPTestClient) testListTools() (*TestResult, error) {
	start := time.Now()

	response, err := c.sendRequest("tools/list", map[string]interface{}{})
	duration := time.Since(start)

	result := &TestResult{
		TestName: "List Tools",
		Duration: duration,
		Response: response,
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}

	result.Success = true
	return result, nil
}

// runAllTests runs a comprehensive test suite
func (c *MCPTestClient) runAllTests() []*TestResult {
	var results []*TestResult

	// Test 1: List Tools
	fmt.Println("🔍 Testing: List Tools")
	if result, err := c.testListTools(); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		results = append(results, result)
	} else {
		fmt.Printf("✅ Success: Found tools\n")
		results = append(results, result)
	}

	// Test 2: Infer Language Tool
	fmt.Println("🔍 Testing: Infer Language Tool")
	inferArgs := map[string]interface{}{
		"file_path": "/test/example.go",
	}
	if result, err := c.testToolCall("infer_language", inferArgs); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		results = append(results, result)
	} else {
		fmt.Printf("✅ Success: Language inferred\n")
		results = append(results, result)
	}

	// Test 3: LSP Connect Tool
	fmt.Println("🔍 Testing: LSP Connect Tool")
	connectArgs := map[string]interface{}{
		"language": "go",
	}
	if result, err := c.testToolCall("lsp_connect", connectArgs); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		results = append(results, result)
	} else {
		fmt.Printf("✅ Success: LSP connected\n")
		results = append(results, result)
	}

	// Test 4: Analyze Code Tool
	fmt.Println("🔍 Testing: Analyze Code Tool")
	analyzeArgs := map[string]interface{}{
		"uri":       "file:///test/example.go",
		"line":      10,
		"character": 5,
	}
	if result, err := c.testToolCall("analyze_code", analyzeArgs); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		results = append(results, result)
	} else {
		fmt.Printf("✅ Success: Code analyzed\n")
		results = append(results, result)
	}

	// Test 5: LSP Disconnect Tool
	fmt.Println("🔍 Testing: LSP Disconnect Tool")
	disconnectArgs := map[string]interface{}{}
	if result, err := c.testToolCall("lsp_disconnect", disconnectArgs); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		results = append(results, result)
	} else {
		fmt.Printf("✅ Success: LSP disconnected\n")
		results = append(results, result)
	}

	return results
}

// generateReport generates a test report
func generateReport(results []*TestResult) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📋 MCP EXTERNAL TEST REPORT")
	fmt.Println(strings.Repeat("=", 60))

	successful := 0
	failed := 0
	totalDuration := time.Duration(0)

	for _, result := range results {
		totalDuration += result.Duration

		status := "✅ PASS"
		if !result.Success {
			status = "❌ FAIL"
			failed++
		} else {
			successful++
		}

		fmt.Printf("%-30s %s (%v)\n", result.TestName, status, result.Duration)
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.Error)
		}
	}

	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("📊 Summary: %d passed, %d failed (Total: %v)\n",
		successful, failed, totalDuration)

	if failed == 0 {
		fmt.Println("🎉 All tests passed!")
	} else {
		fmt.Printf("⚠️  %d test(s) failed\n", failed)
	}

	// Save detailed report to file
	saveDetailedReport(results)
}

// saveDetailedReport saves a detailed JSON report
func saveDetailedReport(results []*TestResult) {
	reportData := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"summary": map[string]any{
			"total": len(results),
			"successful": func() int {
				count := 0
				for _, r := range results {
					if r.Success {
						count++
					}
				}
				return count
			}(),
			"failed": func() int {
				count := 0
				for _, r := range results {
					if !r.Success {
						count++
					}
				}
				return count
			}(),
		},
		"tests": results,
	}

	reportJSON, _ := json.MarshalIndent(reportData, "", "  ")

	reportFile := filepath.Join("..", "mcp_test_report.json")
	if err := os.WriteFile(reportFile, reportJSON, 0644); err != nil {
		fmt.Printf("⚠️  Failed to save detailed report: %v\n", err)
	} else {
		fmt.Printf("📄 Detailed report saved to: %s\n", reportFile)
	}
}

func main() {
	fmt.Println("🚀 Starting MCP External Testing")
	fmt.Println("=" + strings.Repeat("=", 50))

	// Ensure we're in the right directory
	if err := os.Chdir("scripts"); err != nil {
		if err := os.MkdirAll("scripts", 0755); err != nil {
			log.Fatalf("Failed to create scripts directory: %v", err)
		}
		if err := os.Chdir("scripts"); err != nil {
			log.Fatalf("Failed to change to scripts directory: %v", err)
		}
	}

	client, err := NewMCPTestClient()
	if err != nil {
		log.Fatalf("Failed to create MCP test client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("failed to close client: %v", err)
		}
	}()

	fmt.Println("🔗 MCP Client connected successfully")

	// Run all tests
	results := client.runAllTests()

	// Generate report
	generateReport(results)

	fmt.Println("\n🏁 Testing completed")
}
