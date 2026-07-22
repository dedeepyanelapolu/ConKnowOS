import time
import uuid
from typing import List, Dict, Any, Optional

class StepStatus:
    PENDING = "PENDING"
    RUNNING = "RUNNING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"

class WorkflowStep:
    def __init__(self, step_id: str, name: str, tool_name: str, inputs: Dict[str, Any], depends_on: Optional[List[str]] = None):
        self.step_id = step_id
        self.name = name
        self.tool_name = tool_name
        self.inputs = inputs
        self.depends_on = depends_on or []
        self.status = StepStatus.PENDING
        self.output: Any = None
        self.error: Optional[str] = None
        self.start_time: Optional[float] = None
        self.end_time: Optional[float] = None
        self.latency_ms: float = 0.0
        self.token_cost: float = 0.00015

    def to_dict(self) -> Dict[str, Any]:
        return {
            "step_id": self.step_id,
            "name": self.name,
            "tool_name": self.tool_name,
            "inputs": self.inputs,
            "depends_on": self.depends_on,
            "status": self.status,
            "output": self.output,
            "error": self.error,
            "latency_ms": self.latency_ms,
            "token_cost": self.token_cost
        }


class WorkflowState:
    def __init__(self, workflow_id: str, name: str):
        self.workflow_id = workflow_id
        self.name = name
        self.created_at = time.time()
        self.updated_at = time.time()
        self.steps: Dict[str, WorkflowStep] = {}
        self.status = StepStatus.PENDING
        self.checkpoint_history: List[Dict[str, Any]] = []

    def add_step(self, step: WorkflowStep):
        self.steps[step.step_id] = step

    def create_checkpoint(self, note: str = "auto_checkpoint") -> Dict[str, Any]:
        self.updated_at = time.time()
        cp = {
            "checkpoint_id": str(uuid.uuid4()),
            "timestamp": self.updated_at,
            "workflow_id": self.workflow_id,
            "note": note,
            "status": self.status,
            "completed_steps": [s.step_id for s in self.steps.values() if s.status == StepStatus.COMPLETED],
            "pending_steps": [s.step_id for s in self.steps.values() if s.status == StepStatus.PENDING]
        }
        self.checkpoint_history.append(cp)
        return cp

    def to_dict(self) -> Dict[str, Any]:
        return {
            "workflow_id": self.workflow_id,
            "name": self.name,
            "status": self.status,
            "created_at": self.created_at,
            "updated_at": self.updated_at,
            "steps": [s.to_dict() for s in self.steps.values()],
            "checkpoints": self.checkpoint_history
        }


class WorkflowRunner:
    """Microkernel Execution State Engine & Task Runner"""
    def __init__(self):
        self.workflows: Dict[str, WorkflowState] = {}

    def create_workflow(self, name: str) -> WorkflowState:
        wf_id = f"wf_{uuid.uuid4().hex[:8]}"
        wf = WorkflowState(wf_id, name)
        self.workflows[wf_id] = wf
        wf.create_checkpoint("initialization")
        return wf

    def get_workflow(self, wf_id: str) -> Optional[WorkflowState]:
        return self.workflows.get(wf_id)

    def execute_step(self, wf_id: str, step_id: str, tool_dispatcher) -> Dict[str, Any]:
        wf = self.get_workflow(wf_id)
        if not wf or step_id not in wf.steps:
            return {"error": "Workflow or step not found"}

        step = wf.steps[step_id]
        
        # Check dependencies
        for dep in step.depends_on:
            if dep in wf.steps and wf.steps[dep].status != StepStatus.COMPLETED:
                return {"error": f"Dependency step {dep} is not completed"}

        step.status = StepStatus.RUNNING
        step.start_time = time.time()
        
        # Execute tool via MCP transport router / dispatcher
        res = tool_dispatcher(step.tool_name, step.inputs)

        step.end_time = time.time()
        step.latency_ms = round((step.end_time - step.start_time) * 1000, 2)

        if "error" in res and res["error"]:
            step.status = StepStatus.FAILED
            step.error = str(res["error"])
            wf.status = StepStatus.FAILED
        else:
            step.status = StepStatus.COMPLETED
            step.output = res.get("result", res)

        wf.create_checkpoint(f"after_step_{step_id}")
        
        # Check if all steps completed
        if all(s.status == StepStatus.COMPLETED for s in wf.steps.values()):
            wf.status = StepStatus.COMPLETED

        return step.to_dict()
