import urllib.request
import json
from typing import Dict, Any, List, Optional

class AgentSession:
    """Session wrapper for ContextOS agent turns and context building"""
    def __init__(self, client: "ContextOSClient", session_id: str, system_prompt: str):
        self.client = client
        self.session_id = session_id
        self.system_prompt = system_prompt

    def add_user_message(self, content: str) -> Dict[str, Any]:
        return self.client.add_message(self.session_id, "user", content)

    def add_assistant_message(self, content: str, tool_calls: Optional[List[Any]] = None) -> Dict[str, Any]:
        return self.client.add_message(self.session_id, "assistant", content, tool_calls)

    def get_compacted_context(self, vector_facts: Optional[List[str]] = None) -> Dict[str, Any]:
        return self.client.build_context(self.session_id, self.system_prompt, vector_facts)


class ContextOSClient:
    """ContextOS Microkernel SDK Client"""
    def __init__(self, endpoint: str = "http://localhost:8080"):
        self.endpoint = endpoint.rstrip("/")

    def _request(self, path: str, method: str = "GET", data: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        url = f"{self.endpoint}{path}"
        headers = {"Content-Type": "application/json"}
        req_body = json.dumps(data).encode("utf-8") if data else None
        
        req = urllib.request.Request(url, data=req_body, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=10) as resp:
                res_bytes = resp.read()
                return json.loads(res_bytes.decode("utf-8"))
        except Exception as e:
            return {"error": str(e)}

    def get_health(self) -> Dict[str, Any]:
        return self._request("/api/v1/health")

    def create_session(self, session_id: str, system_prompt: str = "You are a ContextOS Agent") -> AgentSession:
        return AgentSession(self, session_id, system_prompt)

    def add_message(self, session_id: str, role: str, content: str, tool_calls: Optional[List[Any]] = None) -> Dict[str, Any]:
        return self._request("/api/v1/context/message", "POST", {
            "session_id": session_id,
            "role": role,
            "content": content,
            "tool_calls": tool_calls or []
        })

    def build_context(self, session_id: str, system_prompt: str, vector_facts: Optional[List[str]] = None) -> Dict[str, Any]:
        return self._request("/api/v1/context/build", "POST", {
            "session_id": session_id,
            "system_prompt": system_prompt,
            "vector_facts": vector_facts or []
        })

    def create_workflow(self, name: str) -> Dict[str, Any]:
        return self._request("/api/v1/workflow/create", "POST", {"name": name})

    def list_mcp_tools(self) -> Dict[str, Any]:
        return self._request("/api/v1/mcp/tools")

    def dispatch_mcp_tool(self, tool_name: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
        return self._request("/api/v1/mcp/dispatch", "POST", {
            "tool_name": tool_name,
            "arguments": arguments
        })

    def publish_a2a_event(self, topic: str, event_type: str, sender_id: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        return self._request("/api/v1/a2a/publish", "POST", {
            "topic": topic,
            "event_type": event_type,
            "sender_id": sender_id,
            "payload": payload
        })
