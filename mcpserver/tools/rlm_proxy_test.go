package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// captureRLM поднимает фейковый upstream rlm-tools-bsl и возвращает
// аргументы последнего tools/call.
func captureRLM(t *testing.T) (*map[string]interface{}, *string) {
	t.Helper()
	var gotArgs map[string]interface{}
	var gotName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotArgs = req.Params.Arguments
		gotName = req.Params.Name
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"ok\":true}"}]}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RLM_MCP_URL", srv.URL)
	return &gotArgs, &gotName
}

func callReq(args map[string]interface{}) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func domainsOf(t *testing.T, v interface{}) []string {
	t.Helper()
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("domains is not an array: %#v", v)
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		out = append(out, x.(string))
	}
	return out
}

func TestRLMStartDomainsForwarding(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		env  *string
		want []string
	}{
		{"array", []interface{}{"документ", "код"}, nil, []string{"документ", "код"}},
		{"empty array is kept", []interface{}{}, nil, []string{}},
		{"json string", `["связи"]`, nil, []string{"связи"}},
		{"empty json string", `[]`, nil, []string{}},
		{"comma string", "поиск, структура", nil, []string{"поиск", "структура"}},
		{"omitted uses builtin default", nil, nil, []string{"поиск", "код"}},
		{"omitted uses env default", nil, strPtr("связи,документ"), []string{"связи", "документ"}},
		{"omitted with empty env -> core only", nil, strPtr(""), []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, name := captureRLM(t)
			if tc.env != nil {
				t.Setenv("RLM_DEFAULT_DOMAINS", *tc.env)
			} else {
				t.Setenv("RLM_DEFAULT_DOMAINS", "unset-marker")
				unsetEnv(t, "RLM_DEFAULT_DOMAINS")
			}
			args := map[string]interface{}{"query": "q", "path": "/abs/src"}
			if tc.in != nil {
				args["domains"] = tc.in
			}
			_, handler := RLMStartTool(nil)
			res, err := handler(context.Background(), callReq(args))
			if err != nil || res.IsError {
				t.Fatalf("unexpected error: %v %#v", err, res)
			}
			if *name != "rlm_start" {
				t.Fatalf("tool name = %q", *name)
			}
			d, present := (*got)["domains"]
			if !present {
				t.Fatalf("domains must always be forwarded, args=%v", *got)
			}
			if gotD := domainsOf(t, d); !reflect.DeepEqual(gotD, tc.want) {
				t.Fatalf("domains = %#v, want %#v", gotD, tc.want)
			}
		})
	}
}

func TestRLMExecuteDomainsForwarding(t *testing.T) {
	got, _ := captureRLM(t)
	_, handler := RLMExecuteTool(nil)
	if _, err := handler(context.Background(), callReq(map[string]interface{}{
		"session_id": "s1", "code": "print(1)", "domains": []interface{}{"связи"},
	})); err != nil {
		t.Fatal(err)
	}
	if d := domainsOf(t, (*got)["domains"]); !reflect.DeepEqual(d, []string{"связи"}) {
		t.Fatalf("domains = %#v", d)
	}

	_, _ = handler(context.Background(), callReq(map[string]interface{}{"session_id": "s1", "code": "print(1)"}))
	if _, present := (*got)["domains"]; present {
		t.Fatalf("rlm_execute must not send domains when omitted: %v", *got)
	}
}

func TestRLMHelpDomainForwarding(t *testing.T) {
	got, _ := captureRLM(t)
	_, handler := RLMHelpTool(nil)
	if _, err := handler(context.Background(), callReq(map[string]interface{}{"domain": "поиск"})); err != nil {
		t.Fatal(err)
	}
	if d := domainsOf(t, (*got)["domain"]); !reflect.DeepEqual(d, []string{"поиск"}) {
		t.Fatalf("domain = %#v", d)
	}
}

func TestRLMToolSchemasDeclareDomains(t *testing.T) {
	for _, tc := range []struct {
		tool  mcp.Tool
		param string
	}{
		{mustTool(RLMStartTool(nil)), "domains"},
		{mustTool(RLMExecuteTool(nil)), "domains"},
		{mustTool(RLMHelpTool(nil)), "domain"},
	} {
		prop, ok := tc.tool.InputSchema.Properties[tc.param].(map[string]interface{})
		if !ok {
			t.Fatalf("%s: no %s property", tc.tool.Name, tc.param)
		}
		if prop["type"] != "array" {
			t.Fatalf("%s.%s type = %v", tc.tool.Name, tc.param, prop["type"])
		}
		for _, r := range tc.tool.InputSchema.Required {
			if r == tc.param {
				t.Fatalf("%s.%s must stay optional (bridge supplies a default)", tc.tool.Name, tc.param)
			}
		}
	}
}

func mustTool(tool mcp.Tool, _ interface{}) mcp.Tool { return tool }

func strPtr(s string) *string { return &s }

// unsetEnv снимает переменную окружения на время теста (предшествующий
// t.Setenv регистрирует восстановление исходного значения).
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}
