import time
import uuid
from typing import Dict, Any, List

class KnowledgeMCPServer:
    """MCP Server Plugin: Neo4j Knowledge Graph & PostgreSQL Relational Audit Log"""
    def __init__(self):
        self.server_id = "mcp_knowledge_server"
        self.name = "Knowledge Graph & Audit MCP Plugin"
        self.nodes: Dict[str, Dict[str, Any]] = {
            "ContextOS": {"type": "System", "description": "AI Agent Runtime OS"},
            "Microkernel": {"type": "CoreModule", "description": "Go/Python state concurrency engine"},
            "MCP": {"type": "Protocol", "description": "Model Context Protocol"},
            "A2A": {"type": "Protocol", "description": "Agent to Agent Communication Bus"}
        }
        self.edges: List[Dict[str, Any]] = [
            {"source": "ContextOS", "relation": "HAS_CORE", "target": "Microkernel"},
            {"source": "ContextOS", "relation": "USES_PROTOCOL", "target": "MCP"},
            {"source": "ContextOS", "relation": "USES_PROTOCOL", "target": "A2A"}
        ]
        self.audit_logs: List[Dict[str, Any]] = []

    def add_graph_relation(self, args: Dict[str, Any]) -> Dict[str, Any]:
        source = args.get("source")
        target = args.get("target")
        relation = args.get("relation", "RELATED_TO")
        
        if not source or not target:
            raise ValueError("source and target are required")

        if source not in self.nodes:
            self.nodes[source] = {"type": "Entity", "description": f"Custom entity {source}"}
        if target not in self.nodes:
            self.nodes[target] = {"type": "Entity", "description": f"Custom entity {target}"}

        edge = {"source": source, "relation": relation, "target": target}
        self.edges.append(edge)
        return {"status": "edge_created", "edge": edge}

    def query_knowledge_graph(self, args: Dict[str, Any]) -> Dict[str, Any]:
        entity = args.get("entity", "")
        matching_edges = [e for e in self.edges if e["source"] == entity or e["target"] == entity or not entity]
        return {
            "query_entity": entity,
            "nodes": [dict(id=k, **v) for k, v in self.nodes.items() if not entity or k == entity],
            "edges": matching_edges
        }

    def log_audit_checkpoint(self, args: Dict[str, Any]) -> Dict[str, Any]:
        audit_id = f"audit_{uuid.uuid4().hex[:8]}"
        record = {
            "audit_id": audit_id,
            "timestamp": time.time(),
            "workflow_id": args.get("workflow_id", "wf_global"),
            "step_id": args.get("step_id", "step_0"),
            "action": args.get("action", "EXECUTE_TOOL"),
            "details": args.get("details", {})
        }
        self.audit_logs.append(record)
        return {"status": "logged", "audit_id": audit_id}

    def query_audit_logs(self, args: Dict[str, Any]) -> Dict[str, Any]:
        workflow_id = args.get("workflow_id")
        filtered = [a for a in self.audit_logs if not workflow_id or a["workflow_id"] == workflow_id]
        return {"total": len(filtered), "logs": filtered}
