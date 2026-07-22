import time
import math
from typing import List, Dict, Any, Optional

class TokenCompactor:
    """High-performance Prompt Compactor & Context Optimizer"""
    def __init__(self, max_tokens: int = 128000, target_summary_tokens: int = 4096, ratio: float = 0.35):
        self.max_tokens = max_tokens
        self.target_summary_tokens = target_summary_tokens
        self.ratio = ratio

    def estimate_tokens(self, text: str) -> int:
        """Estimate token count based on whitespace and subword heuristics (4 chars ~ 1 token)"""
        if not text:
            return 0
        return math.ceil(len(text) / 3.8)

    def compact_history(self, messages: List[Dict[str, Any]], sliding_window: int = 6) -> Dict[str, Any]:
        """
        Compacts full turn history into:
        1. Condensed Summary of older turns.
        2. Unmodified sliding window of recent turns.
        """
        start_time = time.time()
        total_raw_tokens = sum(self.estimate_tokens(m.get("content", "")) for m in messages)
        
        if len(messages) <= sliding_window:
            return {
                "summary": "",
                "recent_messages": messages,
                "raw_tokens": total_raw_tokens,
                "compacted_tokens": total_raw_tokens,
                "savings_percent": 0.0,
                "latency_ms": round((time.time() - start_time) * 1000, 2)
            }

        older_messages = messages[:-sliding_window]
        recent_messages = messages[-sliding_window:]

        # Perform semantic summarization / key-fact extraction on older messages
        extracted_facts = []
        for idx, msg in enumerate(older_messages):
            role = msg.get("role", "user").upper()
            content = msg.get("content", "")
            if len(content) > 100:
                snippet = content[:90] + "..."
            else:
                snippet = content
            extracted_facts.append(f"[{role} turn {idx+1}]: {snippet}")

        summary = "CONTEXT SUMMARY:\n" + "\n".join(extracted_facts)
        
        summary_tokens = self.estimate_tokens(summary)
        recent_tokens = sum(self.estimate_tokens(m.get("content", "")) for m in recent_messages)
        compacted_tokens = summary_tokens + recent_tokens

        savings = max(0.0, round((1.0 - (compacted_tokens / max(1, total_raw_tokens))) * 100, 2))
        latency = round((time.time() - start_time) * 1000, 2)

        return {
            "summary": summary,
            "recent_messages": recent_messages,
            "raw_tokens": total_raw_tokens,
            "compacted_tokens": compacted_tokens,
            "savings_percent": savings,
            "latency_ms": latency
        }


class ContextManager:
    """Manages active session context, working memory, and episodic vector lookups"""
    def __init__(self):
        self.compactor = TokenCompactor()
        self.sessions: Dict[str, List[Dict[str, Any]]] = {}
        self.working_memory: Dict[str, Dict[str, Any]] = {}

    def get_or_create_session(self, session_id: str) -> List[Dict[str, Any]]:
        if session_id not in self.sessions:
            self.sessions[session_id] = []
        return self.sessions[session_id]

    def add_message(self, session_id: str, role: str, content: str, tool_calls: Optional[List[Any]] = None):
        session = self.get_or_create_session(session_id)
        msg = {
            "timestamp": time.time(),
            "role": role,
            "content": content,
            "tool_calls": tool_calls or []
        }
        session.append(msg)
        return msg

    def build_prompt_context(self, session_id: str, system_prompt: str, vector_facts: Optional[List[str]] = None) -> Dict[str, Any]:
        session = self.get_or_create_session(session_id)
        compacted = self.compactor.compact_history(session)

        facts_block = ""
        if vector_facts:
            facts_block = "\nEPISODIC KNOWLEDGE RETRIEVED:\n" + "\n".join([f"- {f}" for f in vector_facts])

        full_prompt = f"{system_prompt}\n{facts_block}\n\n{compacted['summary']}\n"
        
        return {
            "session_id": session_id,
            "system_prompt": system_prompt,
            "vector_facts": vector_facts or [],
            "summary": compacted["summary"],
            "messages": compacted["recent_messages"],
            "token_stats": {
                "raw_tokens": compacted["raw_tokens"] + self.compactor.estimate_tokens(system_prompt),
                "compacted_tokens": compacted["compacted_tokens"] + self.compactor.estimate_tokens(system_prompt),
                "savings_percent": compacted["savings_percent"],
                "compaction_latency_ms": compacted["latency_ms"]
            }
        }
