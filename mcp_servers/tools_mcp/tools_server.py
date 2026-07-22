import os
import subprocess
import time
from typing import Dict, Any

class ToolsMCPServer:
    """MCP Server Plugin: Native Shell Execution, System Diagnostics & Code Calculation"""
    def __init__(self):
        self.server_id = "mcp_tools_server"
        self.name = "Native Tools Execution MCP Plugin"

    def execute_shell_command(self, args: Dict[str, Any]) -> Dict[str, Any]:
        command = args.get("command")
        if not command:
            raise ValueError("command string required")
        try:
            output = subprocess.check_output(command, shell=True, stderr=subprocess.STDOUT, timeout=10, text=True)
            return {"exit_code": 0, "stdout": output, "stderr": ""}
        except subprocess.CalledProcessError as e:
            return {"exit_code": e.returncode, "stdout": e.output, "stderr": str(e)}
        except Exception as e:
            return {"exit_code": 1, "stdout": "", "stderr": str(e)}

    def calculate_code_metrics(self, args: Dict[str, Any]) -> Dict[str, Any]:
        target_dir = args.get("directory", "D:\\CMR_Project")
        total_files = 0
        total_lines = 0
        file_extensions: Dict[str, int] = {}

        for root, _, files in os.walk(target_dir):
            if ".git" in root or "node_modules" in root:
                continue
            for f in files:
                total_files += 1
                ext = f.split(".")[-1] if "." in f else "other"
                file_extensions[ext] = file_extensions.get(ext, 0) + 1
                full_path = os.path.join(root, f)
                try:
                    with open(full_path, "r", encoding="utf-8", errors="ignore") as file_obj:
                        total_lines += len(file_obj.readlines())
                except Exception:
                    pass

        return {
            "directory": target_dir,
            "total_files": total_files,
            "total_lines": total_lines,
            "file_types": file_extensions
        }

    def system_diagnostics(self, args: Dict[str, Any]) -> Dict[str, Any]:
        import platform
        return {
            "os": platform.system(),
            "os_release": platform.release(),
            "python_version": platform.python_version(),
            "architecture": platform.machine(),
            "timestamp": time.time(),
            "status": "healthy"
        }
