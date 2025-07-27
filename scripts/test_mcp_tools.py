#!/usr/bin/env python3
import sys
import json
import re
import subprocess
import logging
import uuid

# Configure logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s: %(message)s"
)
logger = logging.getLogger(__name__)


class MCPToolRunner:
    def __init__(self, custom_cmd=None):
        """
        Initialize MCP server process

        Args:
            custom_cmd (list, optional): Custom command to start the MCP server. Defaults to None.
        """
        self.mcp_process = None
        self.custom_cmd = custom_cmd

    def start_mcp_server(self, custom_cmd=None):
        """
        Start the MCP server
        """
        logger.info("Starting MCP server...")

        # Build command to start MCP server
        cmd = custom_cmd or ["go", "run", "/home/rockerboo/code/mcp-lsp-bridge/main.go"]

        # Start the server process
        self.mcp_process = subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stdin=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        logger.info("MCP server started")

    def stop_mcp_server(self):
        """
        Stop the MCP server process
        """
        if self.mcp_process:
            logger.info("Stopping MCP server...")
            self.mcp_process.terminate()
            try:
                self.mcp_process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.mcp_process.kill()

            # Capture and log any output
            stdout, stderr = self.mcp_process.communicate()
            if stdout:
                logger.info(f"Server STDOUT: {stdout}")
            if stderr:
                logger.error(f"Server STDERR: {stderr}")

            self.mcp_process = None

    def parse_mcp_command(self, command_str):
        """
        Parse MCP command string into components.
        This is a more robust parser designed to handle various formats.
        Example input:
        ●? lsp:signature_help (MCP)(uri="file:///path/to/project", line="10", character='5')
        lsp:hover(uri="file:///path/to/file.go", line=10, character=5)
        mcp__lsp__diagnostics (MCP)(report_type="all")
        """
        command_str = command_str.strip()

        # Flexible regex to capture various tool name formats and optional (MCP)
        match = re.match(
            r"^(?:●\s*)?([a-zA-Z0-9_]+(?::[a-zA-Z0-9_]+)*)\s*(?:\(MCP\))?\s*\((.*)\)$",
            command_str,
        )

        if not match:
            logger.error(f"Failed to match command structure: '{command_str}'")
            raise ValueError(f"Invalid MCP command format: {command_str}")

        # tool_name is the first group
        tool_name = match.group(1)
        params_str = match.group(2)
        logger.info(f"params string: {params_str}")

        params = {}
        if params_str:
            # More flexible regex to handle various parameter types including lists
            # Uses ast.literal_eval for safe parsing of complex types
            import ast

            # Split params by commas, but handle nested structures
            def split_complex_params(s):
                params = []
                current = []
                bracket_level = 0
                for char in s:
                    if char in '[{(' and bracket_level == 0:
                        bracket_level += 1
                        current.append(char)
                    elif char in ']})' and bracket_level > 0:
                        bracket_level -= 1
                        current.append(char)
                        if bracket_level == 0:
                            params.append(''.join(current))
                            current = []
                    elif char == ',' and bracket_level == 0:
                        if current:
                            params.append(''.join(current).strip())
                            current = []
                    else:
                        current.append(char)
                if current:
                    params.append(''.join(current).strip())
                return params

            # Parse each parameter
            try:
                for param in split_complex_params(params_str):
                    key, value = param.split('=', 1)
                    key = key.strip()
                    value = value.strip()

                    # Use ast.literal_eval for safe parsing of lists, dicts, etc.
                    try:
                        parsed_value = ast.literal_eval(value)
                    except (ValueError, SyntaxError):
                        # If literal_eval fails, keep as string
                        parsed_value = value.strip('"\'')

                    params[key] = parsed_value
            except Exception as e:
                logger.warning(f"Could not parse parameters: {e}")
                logger.warning(f"Raw params string: {params_str}")
        logger.info(f"Parsed command: Tool='{tool_name}', Params={params}")
        return tool_name, params

    def run_mcp_tool(self, tool_name, params):
        """
        Run MCP tool by sending JSON-RPC request via stdio
        """
        # Generate a unique request ID
        request_id = str(uuid.uuid4())

        # Construct the JSON-RPC request
        request = {
            "jsonrpc": "2.0",
            "method": "tools/call",
            "params": {"name": tool_name, "arguments": params},
            "id": request_id,
        }

        # Serialize the request
        request_json = json.dumps(request)

        logger.info(f"Running MCP Tool: {tool_name}")
        logger.info(f"Parameters: {params}")
        logger.info(f"Request: {request_json}")

        try:
            # Communicate with the running MCP server process
            if not self.mcp_process:
                logger.error("MCP server process is not running")
                return False

            # Write request to stdin
            self.mcp_process.stdin.write(request_json + "\n")
            self.mcp_process.stdin.flush()

            # Read response from stdout
            response_data = self.mcp_process.stdout.readline().strip()

            logger.info(f"Raw Response: {response_data}")

            # Print process stdout and stderr
            stdout, stderr = self.mcp_process.communicate()
            if stdout:
                logger.info(f"Process STDOUT: {stdout}")
            if stderr:
                logger.error(f"Process STDERR: {stderr}")

            # Validate response is a non-empty JSON
            if not response_data:
                logger.error("Received empty response")
                return False

            # Parse response
            try:
                response = json.loads(response_data)
            except json.JSONDecodeError as e:
                logger.error(f"Failed to parse response JSON: {e}")
                logger.error(f"Response data: {response_data}")
                return False

            # Check for success or error
            if "error" in response:
                logger.error(f"MCP Tool Error: {response['error']}")
                return False

            # Print result if exists
            if "result" in response:
                result = response["result"]
                print(json.dumps(result, indent=2))

                # Try to find call hierarchy details
                if isinstance(result, str):
                    call_details_match = re.search(r'(Incoming|Outgoing) CALLS', result, re.IGNORECASE)
                    if call_details_match:
                        print(f"Call hierarchy details found: {result}")
                    else:
                        print(f"Call hierarchy full result: {result}")

            return True

        except Exception as e:
            logger.error(f"Error running MCP tool: {e}")
            return False

    def __enter__(self):
        self.start_mcp_server(self.custom_cmd)
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.stop_mcp_server()


def main():
    import argparse

    parser = argparse.ArgumentParser(description='Test MCP tools with optional custom server startup',
        epilog='''Examples:\n\n  # Run with default MCP server (Go main.go)\n  %(prog)s "project_analysis (MCP)(analysis_type=\"document_symbols\", query=\"mcpserver/tools.go\")"\n\n  # Run with a custom MCP server startup command\n  %(prog)s "lsp:hover (MCP)(uri=\"file:///project/tools.go\", line=10, character=5)" --cmd "python3 alternative_server.py"\n''')
    parser.add_argument('command', help='MCP tool command to run (in the format: "lsp:tool_name (MCP)(param1=value1, param2=value2)")')
    parser.add_argument('--cmd', help='Custom command to start the MCP server. If not provided, defaults to running main.go with Go', default=None)

    args = parser.parse_args()

    try:
        # Handle custom command, preserving full command string
        custom_cmd = None
        if args.cmd and isinstance(args.cmd, str) and args.cmd.strip():
            # Replace newlines with spaces to create a single command line
            custom_cmd = args.cmd.replace('\n', ' ').split()
        with MCPToolRunner(custom_cmd=custom_cmd) as runner:
            tool_name, params = runner.parse_mcp_command(args.command)
            success = runner.run_mcp_tool(tool_name, params)
            sys.exit(0 if success else 1)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
