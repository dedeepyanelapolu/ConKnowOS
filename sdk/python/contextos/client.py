import asyncio
import json
import requests
import websockets
from typing import Callable, Optional, Dict, Any

class ContextOSClient:
    """Asynchronous Client SDK wrapper for ContextOS microkernel API and WebSocket event hub."""
    
    def __init__(self, base_url: str = "http://localhost:8080"):
        self.base_url = base_url.rstrip("/")
        self.ws_url = self.base_url.replace("http://", "ws://").replace("https://", "wss://")

    async def execute_agent(self, session_id: str, task: str, context: Optional[dict] = None) -> dict:
        """Executes or resumes a workflow execution session."""
        url = f"{self.base_url}/api/v1/agent/execute"
        payload = {
            "session_id": session_id,
            "task": task,
            "context": context or {}
        }
        loop = asyncio.get_event_loop()
        response = await loop.run_in_executor(
            None,
            lambda: requests.post(url, json=payload)
        )
        response.raise_for_status()
        return response.json()

    async def get_state(self, session_id: str) -> dict:
        """Retrieves checkpoint state for the specified session_id."""
        url = f"{self.base_url}/api/v1/agent/state/{session_id}"
        loop = asyncio.get_event_loop()
        response = await loop.run_in_executor(
            None,
            lambda: requests.get(url)
        )
        response.raise_for_status()
        return response.json()

    async def stream_trace(self, session_id: str, callback: Callable[[dict], Any]):
        """Listens to WebSocket broker and filters trace signals belonging to the session."""
        url = f"{self.ws_url}/ws/trace"
        async with websockets.connect(url) as websocket:
            async for message in websocket:
                event = json.loads(message)
                payload = event.get("payload", {})
                sid = payload.get("session_id")
                if not session_id or sid == session_id:
                    if asyncio.iscoroutinefunction(callback):
                        await callback(event)
                    else:
                        callback(event)
