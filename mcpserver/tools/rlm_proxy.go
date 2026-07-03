package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/utils"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var rlmRequestID atomic.Int64

type rlmRPCResponse struct {
	Result *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func rlmMCPURL() string {
	if v := os.Getenv("RLM_MCP_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:9000/mcp"
}

func callRLMTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	id := rlmRequestID.Add(1)
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rlmMCPURL(), bytes.NewReader(body))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rlm-tools-bsl is unavailable at %s: %v", rlmMCPURL(), err)), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)
		return mcp.NewToolResultError(fmt.Sprintf("rlm-tools-bsl returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(b.String()))), nil
	}

	raw, err := readRLMHTTPResponse(resp)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var rpc rlmRPCResponse
	if err := json.Unmarshal(raw, &rpc); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse rlm-tools-bsl response: %v: %s", err, string(raw))), nil
	}
	if rpc.Error != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rlm-tools-bsl JSON-RPC error %d: %s", rpc.Error.Code, rpc.Error.Message)), nil
	}
	if rpc.Result == nil {
		return mcp.NewToolResultError("rlm-tools-bsl response has no result"), nil
	}

	texts := make([]string, 0, len(rpc.Result.Content))
	for _, c := range rpc.Result.Content {
		if c.Type == "" || c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	if len(texts) == 0 {
		texts = append(texts, "{}")
	}
	if rpc.Result.IsError {
		return mcp.NewToolResultError(strings.Join(texts, "\n")), nil
	}
	return mcp.NewToolResultText(strings.Join(texts, "\n")), nil
}

func readRLMHTTPResponse(resp *http.Response) ([]byte, error) {
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		var b bytes.Buffer
		_, err := b.ReadFrom(resp.Body)
		return b.Bytes(), err
	}

	scanner := bufio.NewScanner(resp.Body)
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if data.Len() == 0 {
		return nil, fmt.Errorf("rlm-tools-bsl returned empty SSE response")
	}
	return []byte(data.String()), nil
}

func addIfString(args map[string]interface{}, request mcp.CallToolRequest, name string) {
	if v := request.GetString(name, ""); v != "" {
		args[name] = v
	}
}

func addIfProjectPath(args map[string]interface{}, request mcp.CallToolRequest, name string, bridge interfaces.BridgeInterface) error {
	if v := request.GetString(name, ""); v != "" {
		mapped, err := mapRLMProjectPath(v, bridge)
		if err != nil {
			return fmt.Errorf("%s path mapping failed: %w", name, err)
		}
		args[name] = mapped
	}
	return nil
}

func mapRLMProjectPath(projectPath string, bridge interfaces.BridgeInterface) (string, error) {
	filePath := utils.URIToFilePath(projectPath)
	normalizedPath := strings.ReplaceAll(filePath, "\\", "/")
	isRelative := !strings.HasPrefix(normalizedPath, "/") && !utils.IsWindowsAbsPath(normalizedPath)

	if bridge == nil || !bridge.HasPathMapper() || bridge.GetPathMapper() == nil {
		if isRelative {
			if root := rlmContainerRoot(); root != "" {
				return path.Clean(path.Join(root, normalizedPath)), nil
			}
		}
		return projectPath, nil
	}

	mapper := bridge.GetPathMapper()
	if isRelative {
		return path.Clean(path.Join(mapper.ContainerRoot(), normalizedPath)), nil
	}

	containerRoot := strings.TrimSuffix(mapper.ContainerRoot(), "/")
	cleanPath := path.Clean(normalizedPath)
	if cleanPath == containerRoot || strings.HasPrefix(cleanPath, containerRoot+"/") {
		return cleanPath, nil
	}

	return mapper.HostToContainer(filePath)
}

func rlmContainerRoot() string {
	if root := os.Getenv("PROJECTS_ROOT"); root != "" {
		return root
	}
	return os.Getenv("WORKSPACE_ROOT")
}

func addIfInt(args map[string]interface{}, request mcp.CallToolRequest, name string) {
	if v := request.GetInt(name, 0); v != 0 {
		args[name] = v
	}
}

func addBool(args map[string]interface{}, request mcp.CallToolRequest, name string, def bool) {
	args[name] = request.GetBool(name, def)
}

func parseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []string
	if strings.HasPrefix(raw, "[") && json.Unmarshal([]byte(raw), &arr) == nil {
		return arr
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func RLMStartTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("rlm_start",
			mcp.WithDescription("Open an upstream rlm-tools-bsl search/navigation session over a 1C/BSL codebase. Use this whenever you need to discover project-wide 1C context for the task, even if the user did not explicitly ask to search. Prefer rlm_start before Claude Code Grep/Glob/broad Read, shell rg/grep/find, or manual recursive file reads for BSL modules, metadata objects, methods, references/usages, call paths, forms, rights, queries, XML/MDO content, extensions, business mechanisms and full-text search."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("query", mcp.Description("What you want to find or analyze in the BSL codebase."), mcp.Required()),
			mcp.WithString("path", mcp.Description("Path to a 1C configuration root or parent directory. Host paths and paths relative to the mounted project root are converted to container paths before starting RLM.")),
			mcp.WithString("project", mcp.Description("Project name from the RLM registry.")),
			mcp.WithString("effort", mcp.Description("Analysis depth: auto, low, medium, high, max.")),
			mcp.WithNumber("max_output_chars", mcp.Description("Max characters per execute output.")),
			mcp.WithNumber("max_llm_calls", mcp.Description("Override max llm_query calls.")),
			mcp.WithNumber("max_execute_calls", mcp.Description("Override max rlm_execute calls.")),
			mcp.WithNumber("execution_timeout_seconds", mcp.Description("Per-rlm_execute timeout in seconds.")),
			mcp.WithBoolean("include_metadata", mcp.Description("Include file counts/types in response.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := request.RequireString("query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			args := map[string]interface{}{"query": query}
			if err := addIfProjectPath(args, request, "path", bridge); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			addIfString(args, request, "project")
			addIfString(args, request, "effort")
			addIfInt(args, request, "max_output_chars")
			addIfInt(args, request, "max_llm_calls")
			addIfInt(args, request, "max_execute_calls")
			addIfInt(args, request, "execution_timeout_seconds")
			addBool(args, request, "include_metadata", false)
			return callRLMTool(ctx, "rlm_start", args)
		}
}

func RLMExecuteTool(_ interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("rlm_execute",
			mcp.WithDescription("Execute Python helper code inside an RLM session. Batch related discovery work here instead of making many Grep/Glob/Read or rg/grep/find calls across a 1C project. Use helper functions returned by rlm_start/rlm_help and print compact summaries."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id", mcp.Description("Session ID from rlm_start."), mcp.Required()),
			mcp.WithString("code", mcp.Description("Python code to execute in the RLM sandbox."), mcp.Required()),
			mcp.WithString("detail_level", mcp.Description("compact, usage, or full.")),
			mcp.WithNumber("max_new_variables", mcp.Description("When detail_level=full, cap returned new_variables list.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			sessionID, err := request.RequireString("session_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			code, err := request.RequireString("code")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			args := map[string]interface{}{"session_id": sessionID, "code": code}
			addIfString(args, request, "detail_level")
			addIfInt(args, request, "max_new_variables")
			return callRLMTool(ctx, "rlm_execute", args)
		}
}

func RLMEndTool(_ interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("rlm_end",
			mcp.WithDescription("End an RLM exploration session and free resources."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id", mcp.Description("Session ID to end."), mcp.Required()),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			sessionID, err := request.RequireString("session_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return callRLMTool(ctx, "rlm_end", map[string]interface{}{"session_id": sessionID})
		}
}

func RLMHelpTool(_ interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("rlm_help",
			mcp.WithDescription("Get upstream rlm-tools-bsl recipes, helper details, categories and strategy sections."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("topic", mcp.Description("Business domain or alias to fetch a recipe for.")),
			mcp.WithString("helpers", mcp.Description("Comma-separated helper names or JSON string array.")),
			mcp.WithString("category", mcp.Description("discovery, code, xml, composite, business, extension, navigation.")),
			mcp.WithString("section", mcp.Description("workflow, disambiguation, performance, batching, io, critical.")),
			mcp.WithString("format", mcp.Description("compact or full.")),
			mcp.WithBoolean("include_code", mcp.Description("Include code_hint snippets.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := map[string]interface{}{}
			addIfString(args, request, "topic")
			if helpers := parseStringList(request.GetString("helpers", "")); len(helpers) > 0 {
				args["helpers"] = helpers
			}
			addIfString(args, request, "category")
			addIfString(args, request, "section")
			addIfString(args, request, "format")
			addBool(args, request, "include_code", true)
			return callRLMTool(ctx, "rlm_help", args)
		}
}

func RLMProjectsTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("rlm_projects",
			mcp.WithDescription("Manage the upstream RLM project registry: list/add/remove/rename/update."),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("action", mcp.Description("list, add, remove, rename, or update."), mcp.Required()),
			mcp.WithString("name", mcp.Description("Project name.")),
			mcp.WithString("path", mcp.Description("Path to a 1C configuration root or parent. Host paths and paths relative to the mounted project root are converted to container paths for add/update actions.")),
			mcp.WithString("description", mcp.Description("Optional project description.")),
			mcp.WithString("new_name", mcp.Description("New name for rename action.")),
			mcp.WithString("password", mcp.Description("Project password for mutating actions.")),
			mcp.WithBoolean("clear_password", mcp.Description("Remove project password.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			action, err := request.RequireString("action")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			args := map[string]interface{}{"action": action}
			addIfString(args, request, "name")
			if action == "add" || action == "update" {
				if err := addIfProjectPath(args, request, "path", bridge); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			} else {
				addIfString(args, request, "path")
			}
			addIfString(args, request, "description")
			addIfString(args, request, "new_name")
			addIfString(args, request, "password")
			addBool(args, request, "clear_password", false)
			return callRLMTool(ctx, "rlm_projects", args)
		}
}

func RLMIndexTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("rlm_index",
			mcp.WithDescription("Manage the upstream RLM BSL index: build, update, info, drop."),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("action", mcp.Description("build, update, info, or drop."), mcp.Required()),
			mcp.WithString("path", mcp.Description("Path to a 1C configuration root or parent directory. Host paths and paths relative to the mounted project root are converted to container paths for path-based index actions.")),
			mcp.WithString("project", mcp.Description("Project name from the RLM registry.")),
			mcp.WithBoolean("no_calls", mcp.Description("Skip call graph on build.")),
			mcp.WithBoolean("no_metadata", mcp.Description("Skip L2 metadata on build.")),
			mcp.WithBoolean("no_fts", mcp.Description("Skip FTS5 full-text index on build.")),
			mcp.WithBoolean("no_synonyms", mcp.Description("Skip object synonyms on build.")),
			mcp.WithString("confirm", mcp.Description("Project password for build/update/drop confirmation.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			action, err := request.RequireString("action")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			args := map[string]interface{}{"action": action}
			if err := addIfProjectPath(args, request, "path", bridge); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			addIfString(args, request, "project")
			addBool(args, request, "no_calls", false)
			addBool(args, request, "no_metadata", false)
			addBool(args, request, "no_fts", false)
			addBool(args, request, "no_synonyms", false)
			addIfString(args, request, "confirm")
			return callRLMTool(ctx, "rlm_index", args)
		}
}

func RegisterRLMTools(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	if v, err := strconv.ParseBool(os.Getenv("RLM_MCP_TOOLS_ENABLED")); err == nil && !v {
		return
	}
	mcpServer.AddTool(RLMStartTool(bridge))
	mcpServer.AddTool(RLMExecuteTool(bridge))
	mcpServer.AddTool(RLMEndTool(bridge))
	mcpServer.AddTool(RLMHelpTool(bridge))
	mcpServer.AddTool(RLMProjectsTool(bridge))
	mcpServer.AddTool(RLMIndexTool(bridge))
}
