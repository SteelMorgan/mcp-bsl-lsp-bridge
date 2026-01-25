# Security Policy

## Reporting a vulnerability

If you believe you have found a security vulnerability, please **do not** open a public issue.

Instead, open a GitHub Security Advisory (preferred) or contact the maintainer privately.

## Security model (what the bridge tries to protect)

This project exposes powerful “code intelligence” capabilities to an MCP client. The main security risks are:

- **Unauthorized filesystem access** (reading/writing outside the intended workspace)
- **Command execution** (via LSP `workspace/executeCommand` or tool wrappers)
- **Confused deputy** scenarios (agent is tricked into operating on unintended paths)

## Built-in mitigations

- **Path allowlisting**: file operations must be inside allowed directories (see `security/path_validation.go` and bridge checks).
- **Argument sanitization**: LSP process arguments are validated to avoid obvious shell metacharacters.
- **Tool exposure is curated**: potentially dangerous/low-value tools (e.g. `execute_command`) are implemented but **not exposed by default**.

## Operational recommendations

- Run the bridge against a **dedicated workspace** (ideally a container volume) with minimum necessary access.
- Prefer **read-only** mounts when you only need analysis.
- Treat any agent tool call as untrusted input; avoid exposing the bridge to untrusted users.

