import time
from typing import Dict, Any, Callable, List, Optional

class MCPToolDefinition:
    def __init__(self, name: str, description: str, server_id: str, parameters: Dict[str, Any], handler: Callable):
        self.name = name
        self.description = description
        self.server_id = server_id
        self.parameters = parameters
        self.handler = handler

    def to_dict(self) -> Dict[str, Any]:
        return {
            "name": self.name,
            "description": self.description,
            "server_id": self.server_id,
            "parameters": self.parameters
        }


class MCPRouter:
    """Model Context Protocol (MCP) Transport Router & Dispatcher"""
    def __init__(self):
        self.tools: Dict[str, MCPToolDefinition] = {}
        self.servers: Dict[str, Dict[str, Any]] = {}

    def register_server(self, server_id: str, name: str, transport_type: str = "in-memory"):
        self.servers[server_id] = {
            "server_id": server_id,
            "name": name,
            "transport": transport_type,
            "status": "connected",
            "registered_at": time.time(),
            "tools_count": 0
        }

    def register_tool(self, tool_def: MCPToolDefinition):
        self.tools[tool_def.name] = tool_def
        if tool_def.server_id in self.servers:
            self.servers[tool_def.server_id]["tools_count"] += 1

    def list_tools(self) -> List[Dict[str, Any]]:
        return [t.to_dict() for t in self.tools.values()]

    def list_servers(self) -> List[Dict[str, Any]]:
        return list(self.servers.values())

    def dispatch(self, tool_name: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
        start = time.time()
        if tool_name not in self.tools:
            return {
                "error": f"Tool '{tool_name}' not found in MCP registry",
                "result": None,
                "latency_ms": round((time.time() - start) * 1000, 2)
            }

        tool = self.tools[tool_name]
        try:
            res = tool.handler(arguments)
            latency = round((time.time() - start) * 1000, 2)
            return {
                "error": None,
                "tool_name": tool_name,
                "server_id": tool.server_id,
                "result": res,
                "latency_ms": latency
            }
        except Exception as e:
            latency = round((time.time() - start) * 1000, 2)
            return {
                "error": str(e),
                "tool_name": tool_name,
                "server_id": tool.server_id,
                "result": None,
                "latency_ms": latency
            }
