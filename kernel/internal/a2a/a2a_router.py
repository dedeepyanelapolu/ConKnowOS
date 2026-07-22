import time
import uuid
from typing import Dict, Any, List, Callable, Optional

class AgentRegistry:
    def __init__(self):
        self.agents: Dict[str, Dict[str, Any]] = {}

    def register_agent(self, agent_id: str, name: str, capabilities: List[str]):
        self.agents[agent_id] = {
            "agent_id": agent_id,
            "name": name,
            "capabilities": capabilities,
            "status": "active",
            "last_heartbeat": time.time(),
            "tasks_processed": 0
        }

    def update_heartbeat(self, agent_id: str, cpu: float = 0.0, mem_mb: float = 0.0):
        if agent_id in self.agents:
            ag = self.agents[agent_id]
            ag["last_heartbeat"] = time.time()
            ag["cpu"] = cpu
            ag["mem_mb"] = mem_mb

    def list_agents(self) -> List[Dict[str, Any]]:
        return list(self.agents.values())


class A2ARouter:
    """Agent-to-Agent Protocol Event Bus & Task Negotiator"""
    def __init__(self):
        self.registry = AgentRegistry()
        self.subscribers: Dict[str, List[Callable]] = {}
        self.events_log: List[Dict[str, Any]] = []
        self.active_tasks: Dict[str, Dict[str, Any]] = {}

    def publish_event(self, topic: str, event_type: str, sender_id: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        event = {
            "event_id": f"evt_{uuid.uuid4().hex[:8]}",
            "timestamp": time.time(),
            "topic": topic,
            "event_type": event_type,
            "sender_id": sender_id,
            "payload": payload
        }
        self.events_log.append(event)

        # Notify topic subscribers
        if topic in self.subscribers:
            for cb in self.subscribers[topic]:
                try:
                    cb(event)
                except Exception as e:
                    print(f"Error notifying subscriber: {e}")

        return event

    def subscribe(self, topic: str, callback: Callable):
        if topic not in self.subscribers:
            self.subscribers[topic] = []
        self.subscribers[topic].append(callback)

    def negotiate_task(self, sender_id: str, receiver_id: str, action: str, parameters: Dict[str, Any]) -> Dict[str, Any]:
        task_id = f"task_{uuid.uuid4().hex[:8]}"
        task = {
            "task_id": task_id,
            "sender_id": sender_id,
            "receiver_id": receiver_id,
            "action": action,
            "parameters": parameters,
            "status": "ACCEPTED",
            "created_at": time.time()
        }
        self.active_tasks[task_id] = task

        if receiver_id in self.registry.agents:
            self.registry.agents[receiver_id]["tasks_processed"] += 1

        self.publish_event("a2a.tasks", "TASK_SUBMITTED", sender_id, {"task_id": task_id, "action": action})
        return task
