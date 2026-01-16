package lsp

import (
	"os"
	"strings"

	"rockerboo/mcp-lsp-bridge/types"
)

// ApplyEnvOverrides mutates loaded config based on environment variables.
//
// This exists so MCP users can tune runtime parameters (e.g. Java heap) "from outside"
// via Cursor MCP env, depending on project size.
//
// Supported env vars:
// - MCP_LSP_BSL_JAVA_XMX: overrides -Xmx for the BSL language server (e.g. "6g", "6144m")
// - MCP_LSP_JAVA_XMX:     fallback override for any Java-based language server
// - WORKSPACE_ROOT:       substitutes ${WORKSPACE_ROOT} in args (e.g. --workspace=${WORKSPACE_ROOT})
// - PROJECTS_ROOT:        substitutes ${PROJECTS_ROOT} in args
// - Any env var:          ${VAR_NAME} syntax is expanded in all args
func ApplyEnvOverrides(cfg *LSPServerConfig) {
	if cfg == nil || cfg.LanguageServers == nil {
		return
	}

	// Prefer per-language override.
	bslXmx := strings.TrimSpace(os.Getenv("MCP_LSP_BSL_JAVA_XMX"))
	globalXmx := strings.TrimSpace(os.Getenv("MCP_LSP_JAVA_XMX"))

	for serverName, serverCfg := range cfg.LanguageServers {
		// First, expand environment variables in args (e.g. ${WORKSPACE_ROOT})
		serverCfg.Args = expandEnvVarsInArgs(serverCfg.Args)

		// Then apply Java-specific overrides
		if serverCfg.Command == "java" {
			xmx := globalXmx
			if serverName == types.LanguageServer("bsl-language-server") && bslXmx != "" {
				xmx = bslXmx
			}
			if strings.TrimSpace(xmx) != "" {
				serverCfg.Args = setJavaXmx(serverCfg.Args, xmx)
			}
		}

		cfg.LanguageServers[serverName] = serverCfg
	}
}

// expandEnvVarsInArgs replaces ${VAR_NAME} placeholders in args with environment variable values.
// If a variable is not set, the placeholder is left unchanged.
func expandEnvVarsInArgs(args []string) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		result[i] = os.Expand(arg, func(key string) string {
			if val, exists := os.LookupEnv(key); exists {
				return val
			}
			// Return original placeholder if env var not set
			return "${" + key + "}"
		})
	}
	return result
}

func setJavaXmx(args []string, xmx string) []string {
	xmx = strings.TrimSpace(xmx)
	if xmx == "" {
		return args
	}
	if !strings.HasPrefix(xmx, "-Xmx") {
		xmx = "-Xmx" + xmx
	}

	// Remove existing -Xmx... entries.
	clean := make([]string, 0, len(args)+1)
	for _, a := range args {
		if strings.HasPrefix(a, "-Xmx") {
			continue
		}
		clean = append(clean, a)
	}

	// Insert before -jar if present (JVM options must come before -jar).
	for i, a := range clean {
		if a == "-jar" {
			out := make([]string, 0, len(clean)+1)
			out = append(out, clean[:i]...)
			out = append(out, xmx)
			out = append(out, clean[i:]...)
			return out
		}
	}

	// Otherwise prepend.
	return append([]string{xmx}, clean...)
}

