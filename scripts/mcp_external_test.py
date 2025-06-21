#!/usr/bin/env python3
"""
Programmatic MCP External Testing Tool

This module allows for flexible, dynamic testing of MCP tools by:
1. Parsing complex tool command strings
2. Extracting parameters automatically
3. Executing MCP tool commands
4. Supporting sequential testing
5. Validating and reporting results

Key Features:
- Parse tool commands from command-line inputs
- Support for complex MCP tool command formats
- Automatic parameter extraction
- Robust error handling
- Colored console output
- JSON-formatted result reporting
- Sequential command execution
- Context and variable interpolation
- File-based and command-line input support

Usage Examples:
```bash
# Single tool test
python mcp_external_test.py "● lsp:project_analysis (MCP)(analysis_type: \"document_symbols\", query: \"mcpserver/tools.go\", workspace_uri: \"file:///project\")"

# Multiple sequential tests
python mcp_external_test.py \
    "● lsp:project_analysis (MCP)(analysis_type: \"document_symbols\", query: \"mcpserver/tools.go\", workspace_uri: \"file:///project\")" \
    "● lsp:hover (MCP)(uri: \"file:///project/tools.go\", line: 10, character: 5)"

# Dynamic variable interpolation
python mcp_external_test.py \
    "● lsp:project_analysis (MCP)(analysis_type: \"document_symbols\", query: \"mcpserver/tools.go\", workspace_uri: \"file:///project\")" \
    "● lsp:hover (MCP)(uri: \"file:///project/tools.go\", line: ${last_result}, character: 5)"

# Test from file
python mcp_external_test.py test_commands.txt
```

Command Format:
- Prefix with ● (optional)
- Start with lsp:tool_name
- Wrap parameters in (MCP)(...)
- Use key: "value" format for parameters
- Supports ${last_result} and custom variables

Input Methods:
1. Command-line arguments
2. Text file with command list
3. Sequential command execution
4. Context and variable preservation

Available Tools:
- project_analysis
- hover
- implementation
- signature_help
- code_actions
- rename
- and more...

Advanced Features:
- Persist context between commands
- Interpolate results dynamically
- Stop on first error in sequence
- Flexible error handling

When to Use:
- Quickly test MCP tool interactions
- Validate complex tool calls
- Explore LSP server capabilities
- Debugging and development
- Create complex, multi-step test scenarios

Contribute:
Report issues or feature requests at:
https://github.com/rockerboo/mcp-lsp-bridge/issues
"""

import re
import json
import subprocess
import os
import sys
import traceback
from typing import Dict, Any, Optional, List
from dataclasses import dataclass, field
from pathlib import Path

class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    BLUE = '\033[0;34m'
    MAGENTA = '\033[0;35m'
    CYAN = '\033[0;36m'
    NC = '\033[0m'  # No Color

@dataclass
class MCPToolCommand:
    """Represents a parsed MCP tool command"""
    tool_name: str
    parameters: Dict[str, Any]

class MCPExternalTestRunner:
    def __init__(self, project_dir: Path):
        self.project_dir = project_dir
        self.build_output = project_dir / "build" / "mcp-lsp-bridge"
        self.server_process = None
        self.request_id = 1

    def _print_colored(self, message: str, color: str = Colors.NC):
        """Print a colored message"""
        print(f"{color}{message}{Colors.NC}")

    def _start_server(self) -> bool:
        """Start the MCP server process"""
        try:
            self.server_process = subprocess.Popen(
                [str(self.build_output)],
                cwd=self.project_dir,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True
            )
            return True
        except Exception as e:
            self._print_colored(f"❌ Failed to start server: {e}", Colors.RED)
            return False

    def _send_request(self, method: str, params: Optional[Dict] = None) -> Dict:
        """Send a JSON-RPC request to the MCP server"""
        if not self.server_process:
            self._start_server()

        request = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method,
            "params": params or {}
        }
        self.request_id += 1

        try:
            request_json = json.dumps(request) + "\n"
            self.server_process.stdin.write(request_json)
            self.server_process.stdin.flush()

            response_line = self.server_process.stdout.readline()
            return json.loads(response_line.strip())
        except Exception as e:
            self._print_colored(f"❌ Request failed: {e}", Colors.RED)
            return {"error": str(e)}

    def call_tool(self, tool_name: str, arguments: Dict) -> Dict:
        """Call an MCP tool"""
        return self._send_request("tools/call", {
            "name": tool_name,
            "arguments": arguments
        })

    @dataclass
    class CommandContext:
        """Store and pass context between sequential commands"""
        last_result: Optional[Dict] = None
        variables: Dict[str, Any] = field(default_factory=dict)

    def parse_command(self, command_string: str) -> Optional[MCPToolCommand]:
        """Parse a complex MCP tool command string"""
        # Remove the ● symbol and any trailing/leading whitespace
        command_string = command_string.replace('●', '').strip()

        # Extract tool name using regex
        tool_match = re.match(r'lsp:(\w+)\s*\(MCP\)\((.+)\)', command_string)
        if not tool_match:
            self._print_colored(f"❌ Invalid command format: {command_string}", Colors.RED)
            return None

        tool_name = tool_match.group(1)
        params_str = tool_match.group(2)

        # Parse parameters with more robust parsing
        try:
            # Create a dictionary to hold parsed parameters
            params = {}
            
            # More robust regex to handle strings, numbers, and variables
            param_matches = re.findall(r'(\w+):\s*("(?:\\.|[^"])*"|\$\{[^}]+\}|[0-9]+)', params_str)
            
            for key, value in param_matches:
                # Remove quotes from string values
                if value.startswith('"') and value.endswith('"'):
                    value = value[1:-1]
                
                # Handle numeric values
                if value.isdigit():
                    value = int(value)
                
                # Handle variable interpolation
                if isinstance(value, str) and value.startswith('${') and value.endswith('}'):
                    var_name = value[2:-1]
                    # Placeholder for advanced variable resolution
                    value = f"<VAR:{var_name}>"

                # Special handling for workspace_uri to ensure correct parsing
                if key == 'workspace_uri':
                    # Ensure correct URI format
                    value = str(value).replace('""', '"').strip('"')
                
                params[key] = value

            if not params:
                raise ValueError("No valid parameters found")

        except Exception as e:
            self._print_colored(f"❌ Failed to parse parameters: {e}", Colors.RED)
            return None

        return MCPToolCommand(tool_name=tool_name, parameters=params)

    def interpolate_variables(self, params: Dict, context: 'MCPExternalTestRunner.CommandContext') -> Dict:
        """Interpolate variables in parameters using context"""
        interpolated_params = {}
        for key, value in params.items():
            if isinstance(value, str) and value.startswith('<VAR:') and value.endswith('>'):
                var_name = value[5:-1]
                
                # Variable resolution strategies
                if var_name == 'last_result':
                    # Use last result if available
                    if context.last_result:
                        interpolated_params[key] = context.last_result
                    else:
                        raise ValueError(f"No previous result available for variable {var_name}")
                elif var_name in context.variables:
                    # Use predefined context variables
                    interpolated_params[key] = context.variables[var_name]
                else:
                    raise ValueError(f"Undefined variable {var_name}")
            else:
                interpolated_params[key] = value
        
        return interpolated_params

    def run_test(self, command_string: str, context: Optional['MCPExternalTestRunner.CommandContext'] = None) -> Dict:
        """Run a complete MCP tool test with optional context"""
        # Initialize context if not provided
        if context is None:
            context = self.CommandContext()

        self._print_colored(f"🧪 Testing MCP Tool: {command_string}", Colors.CYAN)

        # Parse the command
        parsed_command = self.parse_command(command_string)
        if not parsed_command:
            return {"success": False, "error": "Invalid command format"}

        # Initialize server if needed
        if not self.server_process:
            if not self._start_server():
                return {"success": False, "error": "Failed to start MCP server"}

        try:
            # Interpolate variables
            interpolated_params = self.interpolate_variables(parsed_command.parameters, context)

            # Call the tool
            result = self.call_tool(parsed_command.tool_name, interpolated_params)

            # Determine success based on response
            if "error" in result:
                self._print_colored(f"❌ Tool call failed: {result['error']}", Colors.RED)
                return {"success": False, "result": result}
            
            # Update context
            context.last_result = result
            
            self._print_colored(f"✅ Tool call succeeded", Colors.GREEN)
            return {"success": True, "result": result}

        except Exception as e:
            self._print_colored(f"❌ Test execution error: {e}", Colors.RED)
            return {"success": False, "error": str(e)}

    def run_test_sequence(self, command_sequence: List[str]) -> List[Dict]:
        """Run a sequence of MCP tool tests with shared context"""
        context = self.CommandContext()
        results = []

        for command in command_sequence:
            result = self.run_test(command, context)
            results.append(result)

            # Stop sequence if a test fails
            if not result['success']:
                break

        return results

    def __del__(self):
        """Cleanup server process"""
        if self.server_process:
            try:
                self.server_process.terminate()
                self.server_process.wait(timeout=5)
            except Exception:
                pass

def main():
    project_dir = Path(__file__).parent.parent
    test_runner = MCPExternalTestRunner(project_dir)

    # Support multiple input methods
    if len(sys.argv) > 1:
        # Check if input is a file
        if os.path.isfile(sys.argv[1]):
            with open(sys.argv[1], 'r') as f:
                commands = [line.strip() for line in f if line.strip()]
        else:
            # Treat arguments as command sequence
            commands = [" ".join(sys.argv[1:])]
    else:
        # Default test command if none provided
        commands = [
            '● lsp:project_analysis (MCP)(analysis_type: "document_symbols", query: "mcpserver/tools.go", workspace_uri: "file:///home/rockerboo/code/mcp-lsp-bridge")'
        ]

    try:
        # Run tests
        test_results = test_runner.run_test_sequence(commands)

        # Pretty print the results
        print("\n📋 Test Results:")
        print(json.dumps(test_results, indent=2))

        # Set exit code based on test success
        sys.exit(0 if all(result['success'] for result in test_results) else 1)

    except Exception as e:
        print(f"❌ Test execution error: {e}")
        traceback.print_exc()
        sys.exit(1)

if __name__ == "__main__":
    main()