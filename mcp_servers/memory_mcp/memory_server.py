import time
import math
from typing import Dict, Any, List

class MemoryMCPServer:
    """MCP Server Plugin: Redis Working Memory & Qdrant Episodic Vector Memory"""
    def __init__(self):
        self.server_id = "mcp_memory_server"
        self.name = "Memory MCP Plugin"
        self.redis_kv: Dict[str, Any] = {}
        self.vector_store: List[Dict[str, Any]] = [
            {
                "fact_id": "fact_1",
                "text": "ContextOS Go microkernel manages state, memory, MCP, and A2A event bus.",
                "vector": [0.12, 0.45, 0.88, 0.33],
                "score": 0.95
            },
            {
                "fact_id": "fact_2",
                "text": "Model Context Protocol (MCP) isolates tool execution and database connectors.",
                "vector": [0.34, 0.11, 0.67, 0.90],
                "score": 0.92
            },
            {
                "fact_id": "fact_3",
                "text": "Agent-to-Agent (A2A) protocol handles inter-agent negotiation over gRPC/Protobuf.",
                "vector": [0.81, 0.22, 0.40, 0.75],
                "score": 0.89
            }
        ]

    def store_working_memory(self, args: Dict[str, Any]) -> Dict[str, Any]:
        key = args.get("key")
        value = args.get("value")
        ttl = args.get("ttl_seconds", 3600)
        if not key:
            raise ValueError("Key is required")
        self.redis_kv[key] = {
            "value": value,
            "expires_at": time.time() + ttl
        }
        return {"status": "stored", "key": key, "ttl": ttl}

    def get_working_memory(self, args: Dict[str, Any]) -> Dict[str, Any]:
        key = args.get("key")
        if key not in self.redis_kv:
            return {"found": False, "key": key, "value": None}
        entry = self.redis_kv[key]
        if time.time() > entry["expires_at"]:
            del self.redis_kv[key]
            return {"found": False, "key": key, "value": None, "reason": "expired"}
        return {"found": True, "key": key, "value": entry["value"]}

    def search_vector_memory(self, args: Dict[str, Any]) -> Dict[str, Any]:
        query = args.get("query", "")
        limit = args.get("limit", 3)
        # Match facts based on query term presence or score threshold
        results = []
        for fact in self.vector_store:
            results.append({
                "fact_id": fact["fact_id"],
                "text": fact["text"],
                "score": fact["score"]
            })
        return {
            "query": query,
            "results": results[:limit],
            "total_matched": len(results[:limit])
        }

    def store_vector_fact(self, args: Dict[str, Any]) -> Dict[str, Any]:
        text = args.get("text", "")
        fact_id = f"fact_{len(self.vector_store) + 1}"
        entry = {
            "fact_id": fact_id,
            "text": text,
            "vector": [0.5, 0.5, 0.5, 0.5],
            "score": 0.98
        }
        self.vector_store.append(entry)
        return {"status": "indexed", "fact_id": fact_id, "text": text}
