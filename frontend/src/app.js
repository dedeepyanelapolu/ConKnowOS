// ContextOS Telemetry Portal Client App
const API_BASE = "http://localhost:8080/api/v1";

let activeWorkflow = null;
let sampleTurns = [
  { role: "user", content: "Initialize ContextOS microkernel with Redis and Qdrant memory plugins." },
  { role: "assistant", content: "Memory MCP server connected. Qdrant vector index loaded with 3 semantic facts." },
  { role: "user", content: "Execute code calculation metrics on the current directory." }
];

document.addEventListener("DOMContentLoaded", () => {
  initTabs();
  fetchHealthAndStats();
  initCompactorUI();
  initMCPDirectory();
  initA2ABus();
  initKnowledgeGraph();
  initWorkflowEngine();

  // Refresh interval
  setInterval(fetchHealthAndStats, 5000);
});

// Tab Navigation
function initTabs() {
  document.querySelectorAll(".nav-item").forEach(btn => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".nav-item").forEach(b => b.classList.remove("active"));
      document.querySelectorAll(".tab-content").forEach(t => t.classList.remove("active"));
      
      btn.classList.add("active");
      const target = btn.getAttribute("data-tab");
      document.getElementById(target).classList.add("active");
    });
  });
}

// Fetch Health & System Stats
async function fetchHealthAndStats() {
  try {
    const res = await fetch(`${API_BASE}/health`);
    const data = await res.json();
    if (data.status === "healthy") {
      document.getElementById("kernel-state").textContent = "ONLINE";
      document.getElementById("kernel-status").className = "status-badge active";
    }

    const statsRes = await fetch(`${API_BASE}/telemetry/stats`);
    const stats = await statsRes.json();
    
    document.getElementById("mcp-count").textContent = stats.registered_tools || 11;
    document.getElementById("a2a-count").textContent = stats.a2a_agents || 3;
    document.getElementById("stat-sessions").textContent = stats.active_sessions || 1;
    document.getElementById("stat-workflows").textContent = stats.total_workflows || 1;
    document.getElementById("stat-events").textContent = stats.events_published || 0;
  } catch (err) {
    document.getElementById("kernel-state").textContent = "OFFLINE";
    document.getElementById("kernel-status").className = "status-badge danger";
  }
}

// Workflow DAG Engine
function initWorkflowEngine() {
  document.getElementById("btn-trigger-workflow").addEventListener("click", createSampleWorkflow);
  document.getElementById("btn-refresh-wf").addEventListener("click", renderWorkflowDAG);
  document.getElementById("btn-run-step").addEventListener("click", executeNextStep);

  createSampleWorkflow();
}

async function createSampleWorkflow() {
  try {
    const res = await fetch(`${API_BASE}/workflow/create`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "System Diagnostic & Memory Retrieval Pipeline" })
    });
    activeWorkflow = await res.json();
    renderWorkflowDAG();
  } catch (err) {
    console.error("Workflow creation error:", err);
  }
}

function renderWorkflowDAG() {
  const container = document.getElementById("dag-nodes-list");
  if (!activeWorkflow || !activeWorkflow.steps) {
    container.innerHTML = `<p class="subtitle">No active workflow. Click "Run Sample Workflow" to initialize.</p>`;
    return;
  }

  container.innerHTML = activeWorkflow.steps.map(step => `
    <div class="node-card ${step.status}" onclick="inspectStep('${step.step_id}')">
      <div class="node-title">
        <span>${step.name}</span>
        <span class="badge">${step.status}</span>
      </div>
      <div class="node-tool">Tool: ${step.tool_name}</div>
      <div class="node-meta">
        <span>ID: ${step.step_id}</span>
        <span>Latency: ${step.latency_ms}ms</span>
      </div>
    </div>
  `).join("");
}

window.inspectStep = function(stepId) {
  if (!activeWorkflow) return;
  const step = activeWorkflow.steps.find(s => s.step_id === stepId);
  document.getElementById("wf-step-detail").textContent = JSON.stringify(step, null, 2);
};

async function executeNextStep() {
  if (!activeWorkflow) return;
  const pendingStep = activeWorkflow.steps.find(s => s.status === "PENDING");
  if (!pendingStep) {
    alert("All workflow steps completed!");
    return;
  }

  try {
    const res = await fetch(`${API_BASE}/workflow/step/execute`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ workflow_id: activeWorkflow.workflow_id, step_id: pendingStep.step_id })
    });
    const updatedStep = await res.json();
    const idx = activeWorkflow.steps.findIndex(s => s.step_id === pendingStep.step_id);
    activeWorkflow.steps[idx] = updatedStep;
    renderWorkflowDAG();
    inspectStep(pendingStep.step_id);
  } catch (err) {
    console.error("Step execution error:", err);
  }
}

// Prompt Compactor Inspector UI
function initCompactorUI() {
  renderTurns();
  document.getElementById("btn-add-turn").addEventListener("click", () => {
    const role = document.getElementById("turn-role").value;
    const content = document.getElementById("turn-content").value;
    if (content.trim()) {
      sampleTurns.push({ role, content });
      document.getElementById("turn-content").value = "";
      renderTurns();
      runCompactor();
    }
  });

  document.getElementById("btn-run-compactor").addEventListener("click", runCompactor);
  runCompactor();
}

function renderTurns() {
  const container = document.getElementById("turn-history-list");
  container.innerHTML = sampleTurns.map((t, idx) => `
    <div class="turn-bubble ${t.role}">
      <strong>[Turn ${idx+1} - ${t.role.toUpperCase()}]</strong> ${t.content}
    </div>
  `).join("");
}

async function runCompactor() {
  const session_id = "compactor_inspector_session";
  const system_prompt = document.getElementById("compactor-sys").value;

  // Sync messages
  for (const t of sampleTurns) {
    await fetch(`${API_BASE}/context/message`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session_id, role: t.role, content: t.content })
    });
  }

  const res = await fetch(`${API_BASE}/context/build`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      session_id,
      system_prompt,
      vector_facts: ["ContextOS Go/Python Microkernel handles token compaction and memory."]
    })
  });

  const data = await res.json();
  document.getElementById("comp-raw").textContent = data.token_stats.raw_tokens;
  document.getElementById("comp-compacted").textContent = data.token_stats.compacted_tokens;
  document.getElementById("comp-savings").textContent = `${data.token_stats.savings_percent}%`;
  document.getElementById("comp-latency").textContent = `${data.token_stats.compaction_latency_ms}ms`;

  document.getElementById("token-savings").textContent = `${data.token_stats.savings_percent}%`;

  document.getElementById("comp-output").textContent = JSON.stringify(data, null, 2);
}

// MCP Server Directory
async function initMCPDirectory() {
  try {
    const serversRes = await fetch(`${API_BASE}/mcp/servers`);
    const serversData = await serversRes.json();
    
    const toolsRes = await fetch(`${API_BASE}/mcp/tools`);
    const toolsData = await toolsRes.json();

    const serverContainer = document.getElementById("mcp-servers-grid");
    serverContainer.innerHTML = serversData.servers.map(s => `
      <div class="mcp-card">
        <h3>🔌 ${s.name}</h3>
        <p class="subtitle">Server ID: ${s.server_id} | Transport: ${s.transport}</p>
        <p class="subtitle">Registered Tools: <strong>${s.tools_count}</strong></p>
      </div>
    `).join("");

    const select = document.getElementById("tool-select");
    select.innerHTML = toolsData.tools.map(t => `<option value="${t.name}">${t.name} (${t.server_id})</option>`).join("");

    document.getElementById("btn-dispatch-tool").addEventListener("click", dispatchTool);
  } catch (err) {
    console.error("MCP Directory error:", err);
  }
}

async function dispatchTool() {
  const tool_name = document.getElementById("tool-select").value;
  let args = {};
  try {
    args = JSON.parse(document.getElementById("tool-args").value);
  } catch(e) {
    alert("Invalid JSON in arguments field!");
    return;
  }

  const res = await fetch(`${API_BASE}/mcp/dispatch`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tool_name, arguments: args })
  });

  const result = await res.json();
  document.getElementById("tool-dispatch-result").textContent = JSON.stringify(result, null, 2);
}

// A2A Bus UI
async function initA2ABus() {
  const agentsRes = await fetch(`${API_BASE}/a2a/agents`);
  const agentsData = await agentsRes.json();

  document.getElementById("a2a-agents-grid").innerHTML = agentsData.agents.map(a => `
    <div class="mcp-card">
      <h3>🤖 ${a.name}</h3>
      <p class="subtitle">Agent ID: ${a.agent_id} | Status: ${a.status}</p>
      <p class="subtitle">Capabilities: ${a.capabilities.join(", ")}</p>
    </div>
  `).join("");

  document.getElementById("btn-publish-a2a").addEventListener("click", publishA2AEvent);
}

async function publishA2AEvent() {
  const topic = document.getElementById("a2a-topic").value || "a2a.tasks";
  const event_type = document.getElementById("a2a-type").value || "TASK_SUBMITTED";
  const sender_id = document.getElementById("a2a-sender").value || "agent_planner";

  const res = await fetch(`${API_BASE}/a2a/publish`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ topic, event_type, sender_id, payload: { message: "Task negotiation request" } })
  });

  const evt = await res.json();
  const logContainer = document.getElementById("a2a-event-stream");
  logContainer.innerHTML = `<div class="event-row">[${new Date().toLocaleTimeString()}] TOPIC: ${evt.topic} | TYPE: ${evt.event_type} | SENDER: ${evt.sender_id}</div>` + logContainer.innerHTML;
}

// Knowledge Graph
function initKnowledgeGraph() {
  document.getElementById("kg-visual").innerHTML = `
    <div class="kg-node">🔵 [System] ContextOS  ──(HAS_CORE)──► 🟣 [Core] Microkernel</div>
    <div class="kg-node">🔵 [System] ContextOS  ──(USES_PROTOCOL)──► 🟢 [Protocol] MCP</div>
    <div class="kg-node">🔵 [System] ContextOS  ──(USES_PROTOCOL)──► 🟡 [Protocol] A2A</div>
  `;

  document.getElementById("audit-list").innerHTML = `
    <div class="turn-bubble">[Audit ID: audit_18f92a] Action: EXECUTE_TOOL (system_diagnostics) | Status: SUCCESS</div>
    <div class="turn-bubble">[Audit ID: audit_94e1b0] Action: CHECKPOINT (workflow_created) | Status: COMPLETED</div>
  `;
}
