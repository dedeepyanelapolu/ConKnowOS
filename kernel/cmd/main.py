import sys
import os
import time
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

# Adjust path to import internal & mcp_servers modules
SYS_BASE = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
if SYS_BASE not in sys.path:
    sys.path.insert(0, SYS_BASE)

from kernel.config.config import settings
from kernel.internal.memory.memory_manager import ContextManager
from kernel.internal.state.state_engine import WorkflowRunner, WorkflowStep
from kernel.internal.mcp.mcp_router import MCPRouter, MCPToolDefinition
from kernel.internal.a2a.a2a_router import A2ARouter

from mcp_servers.memory_mcp.memory_server import MemoryMCPServer
from mcp_servers.knowledge_mcp.knowledge_server import KnowledgeMCPServer
from mcp_servers.tools_mcp.tools_server import ToolsMCPServer

# Initialize Core Services
context_manager = ContextManager()
workflow_runner = WorkflowRunner()
mcp_router = MCPRouter()
a2a_router = A2ARouter()

# Instantiate MCP Servers & Register Plugins
memory_mcp_instance = MemoryMCPServer()
knowledge_mcp_instance = KnowledgeMCPServer()
tools_mcp_instance = ToolsMCPServer()

# Register MCP Servers
mcp_router.register_server("mcp_memory", "Redis & Qdrant Memory Server", "in-memory")
mcp_router.register_server("mcp_knowledge", "Neo4j & Postgres Knowledge Server", "in-memory")
mcp_router.register_server("mcp_tools", "Native System Execution Tools Server", "in-memory")

# Register Tools
mcp_router.register_tool(MCPToolDefinition(
    "store_working_memory", "Store key-value state in Redis working memory", "mcp_memory",
    {"key": "string", "value": "any", "ttl_seconds": "int"}, memory_mcp_instance.store_working_memory
))
mcp_router.register_tool(MCPToolDefinition(
    "get_working_memory", "Retrieve stored key-value state from Redis", "mcp_memory",
    {"key": "string"}, memory_mcp_instance.get_working_memory
))
mcp_router.register_tool(MCPToolDefinition(
    "search_vector_memory", "Perform semantic similarity search on Qdrant episodic memory", "mcp_memory",
    {"query": "string", "limit": "int"}, memory_mcp_instance.search_vector_memory
))
mcp_router.register_tool(MCPToolDefinition(
    "store_vector_fact", "Index a semantic fact into Qdrant vector memory", "mcp_memory",
    {"text": "string"}, memory_mcp_instance.store_vector_fact
))

mcp_router.register_tool(MCPToolDefinition(
    "add_graph_relation", "Add entity-relationship edge to Neo4j knowledge graph", "mcp_knowledge",
    {"source": "string", "target": "string", "relation": "string"}, knowledge_mcp_instance.add_graph_relation
))
mcp_router.register_tool(MCPToolDefinition(
    "query_knowledge_graph", "Query entity relationships in Neo4j knowledge graph", "mcp_knowledge",
    {"entity": "string"}, knowledge_mcp_instance.query_knowledge_graph
))
mcp_router.register_tool(MCPToolDefinition(
    "log_audit_checkpoint", "Record execution state checkpoint to Postgres audit log", "mcp_knowledge",
    {"workflow_id": "string", "action": "string"}, knowledge_mcp_instance.log_audit_checkpoint
))
mcp_router.register_tool(MCPToolDefinition(
    "query_audit_logs", "Query audit logs from PostgreSQL database", "mcp_knowledge",
    {"workflow_id": "string"}, knowledge_mcp_instance.query_audit_logs
))

mcp_router.register_tool(MCPToolDefinition(
    "execute_shell_command", "Execute a shell/CLI command safely", "mcp_tools",
    {"command": "string"}, tools_mcp_instance.execute_shell_command
))
mcp_router.register_tool(MCPToolDefinition(
    "calculate_code_metrics", "Analyze codebase line counts and file types", "mcp_tools",
    {"directory": "string"}, tools_mcp_instance.calculate_code_metrics
))
mcp_router.register_tool(MCPToolDefinition(
    "system_diagnostics", "Return system OS, Python, and hardware diagnostic metrics", "mcp_tools",
    {}, tools_mcp_instance.system_diagnostics
))

# Register Sample Agents in A2A Bus
a2a_router.registry.register_agent("agent_planner", "Strategic Planner Agent", ["planning", "decomposition"])
a2a_router.registry.register_agent("agent_executor", "Tool Execution Agent", ["mcp_dispatch", "coding"])
a2a_router.registry.register_agent("agent_evaluator", "Quality Evaluator Agent", ["reflection", "validation"])


class ContextOSRequestHandler(BaseHTTPRequestHandler):
    """ContextOS Microkernel HTTP & Telemetry Server"""
    
    def _send_json(self, data: Any, status: int = 200):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Headers", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.end_headers()
        self.wfile.write(json.dumps(data, default=str).encode("utf-8"))

    def _send_file(self, file_path: str, content_type: str):
        if os.path.exists(file_path):
            self.send_response(200)
            self.send_header("Content-Type", content_type)
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            with open(file_path, "rb") as f:
                self.wfile.write(f.read())
        else:
            self._send_json({"error": "File not found"}, 404)

    def do_OPTIONS(self):
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Headers", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.end_headers()

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path

        if path == "/api/v1/health":
            self._send_json({
                "status": "healthy",
                "system": "ContextOS Microkernel",
                "version": "1.0.0",
                "env": settings.ENV,
                "timestamp": time.time()
            })
        elif path == "/api/v1/mcp/tools":
            self._send_json({"tools": mcp_router.list_tools()})
        elif path == "/api/v1/mcp/servers":
            self._send_json({"servers": mcp_router.list_servers()})
        elif path == "/api/v1/a2a/agents":
            self._send_json({"agents": a2a_router.registry.list_agents()})
        elif path == "/api/v1/telemetry/stats":
            self._send_json({
                "active_sessions": len(context_manager.sessions),
                "total_workflows": len(workflow_runner.workflows),
                "mcp_servers": len(mcp_router.servers),
                "registered_tools": len(mcp_router.tools),
                "a2a_agents": len(a2a_router.registry.agents),
                "events_published": len(a2a_router.events_log)
            })
        elif path == "/" or path == "/index.html":
            self._send_file(os.path.join(SYS_BASE, "frontend", "index.html"), "text/html")
        elif path.startswith("/src/") or path.startswith("/public/"):
            file_p = os.path.join(SYS_BASE, "frontend", path.lstrip("/"))
            c_type = "text/css" if path.endswith(".css") else ("application/javascript" if path.endswith(".js") else "text/plain")
            self._send_file(file_p, c_type)
        else:
            self._send_json({"error": "Endpoint not found"}, 404)

    def do_POST(self):
        content_len = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_len).decode("utf-8") if content_len > 0 else "{}"
        try:
            data = json.loads(body)
        except Exception:
            data = {}

        parsed = urlparse(self.path)
        path = parsed.path

        if path == "/api/v1/context/message":
            session_id = data.get("session_id", "default_session")
            role = data.get("role", "user")
            content = data.get("content", "")
            msg = context_manager.add_message(session_id, role, content)
            self._send_json({"status": "success", "message": msg})

        elif path == "/api/v1/context/build":
            session_id = data.get("session_id", "default_session")
            system_prompt = data.get("system_prompt", "You are ContextOS AI Assistant.")
            vector_facts = data.get("vector_facts", [])
            ctx = context_manager.build_prompt_context(session_id, system_prompt, vector_facts)
            self._send_json(ctx)

        elif path == "/api/v1/workflow/create":
            name = data.get("name", "New Workflow")
            wf = workflow_runner.create_workflow(name)
            # Add sample steps
            step1 = WorkflowStep("s1", "Inspect System", "system_diagnostics", {})
            step2 = WorkflowStep("s2", "Search Vector Facts", "search_vector_memory", {"query": "ContextOS"}, depends_on=["s1"])
            wf.add_step(step1)
            wf.add_step(step2)
            self._send_json(wf.to_dict())

        elif path == "/api/v1/workflow/step/execute":
            wf_id = data.get("workflow_id")
            step_id = data.get("step_id")
            res = workflow_runner.execute_step(wf_id, step_id, mcp_router.dispatch)
            self._send_json(res)

        elif path == "/api/v1/mcp/dispatch":
            tool_name = data.get("tool_name", "")
            args = data.get("arguments", {})
            res = mcp_router.dispatch(tool_name, args)
            self._send_json(res)

        elif path == "/api/v1/a2a/publish":
            topic = data.get("topic", "default")
            event_type = data.get("event_type", "INFO")
            sender_id = data.get("sender_id", "kernel")
            payload = data.get("payload", {})
            evt = a2a_router.publish_event(topic, event_type, sender_id, payload)
            self._send_json(evt)

        else:
            self._send_json({"error": "API route not found"}, 404)


def run_kernel():
    server_address = (settings.HOST, settings.PORT)
    httpd = HTTPServer(server_address, ContextOSRequestHandler)
    print("============================================================")
    print(f"[START] ContextOS Microkernel v1.0.0 Running on http://localhost:{settings.PORT}")
    print(f"[DASHBOARD] Observability Dashboard: http://localhost:{settings.PORT}/")
    print(f"[MCP] MCP Servers Registered: {len(mcp_router.servers)} | Tools: {len(mcp_router.tools)}")
    print(f"[A2A] A2A Protocol Bus Active with {len(a2a_router.registry.agents)} Agents")
    print("============================================================")
    httpd.serve_forever()

if __name__ == "__main__":
    run_kernel()
