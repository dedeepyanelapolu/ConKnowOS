package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"contextos/kernel/internal/a2a"
	"contextos/kernel/internal/memory"
	"contextos/kernel/internal/state"
)

// ExecuteRequest represents the input parameters for executing or resuming a session.
type ExecuteRequest struct {
	SessionID string                 `json:"session_id"`
	Task      string                 `json:"task"`
	Context   map[string]interface{} `json:"context"`
}

// ExecuteResponse represents the result of starting or resuming execution.
type ExecuteResponse struct {
	SessionID    string                 `json:"session_id"`
	CurrentState string                 `json:"current_state"`
	Payload      map[string]interface{} `json:"payload"`
}

// APIHandler handles HTTP request routing and execution runner management.
type APIHandler struct {
	checkpointer *state.Checkpointer
	contextMgr   *memory.ContextManager
	eventBus     *a2a.EventBus
	reflection   *memory.ReflectionEngine
	mu           sync.RWMutex
	runners      map[string]*state.WorkflowRunner
}

// NewAPIHandler creates a new APIHandler instance.
func NewAPIHandler(checkpointer *state.Checkpointer, contextMgr *memory.ContextManager) *APIHandler {
	return &APIHandler{
		checkpointer: checkpointer,
		contextMgr:   contextMgr,
		runners:      make(map[string]*state.WorkflowRunner),
	}
}

// SetEventBus assigns the event bus.
func (h *APIHandler) SetEventBus(eb *a2a.EventBus) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.eventBus = eb
}

// SetReflectionEngine assigns the reflection engine.
func (h *APIHandler) SetReflectionEngine(re *memory.ReflectionEngine) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reflection = re
}

// ExecuteHandler handles the POST /api/v1/agent/execute endpoint.
func (h *APIHandler) ExecuteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	var runner *state.WorkflowRunner
	var transitionErr error

	// Try to restore the checkpoint from database
	checkpoint, err := h.checkpointer.RestoreCheckpoint(req.SessionID)
	if err != nil {
		// Session does not exist, initialize a new WorkflowRunner
		runner = state.NewWorkflowRunner(req.SessionID, h.checkpointer)
		runner.SetContextManager(h.contextMgr)
		runner.SetEventBus(h.eventBus)
		runner.SetReflectionEngine(h.reflection)
		runner.Payload["task"] = req.Task
		runner.Payload["context"] = req.Context

		// Store runner in active registry
		h.runners[req.SessionID] = runner

		// Trigger initial state transition (Idle -> Planning)
		transitionErr = runner.TransitionTo(state.Planning)
	} else {
		// Resume existing session from persisted state
		runner = state.NewWorkflowRunner(req.SessionID, h.checkpointer)
		runner.SetContextManager(h.contextMgr)
		runner.SetEventBus(h.eventBus)
		runner.SetReflectionEngine(h.reflection)
		runner.CurrentState = state.WorkflowState(checkpoint.CurrentState)
		runner.Payload = checkpoint.Payload

		// Store runner in active registry
		h.runners[req.SessionID] = runner

		// If restored state is Idle, transition to Planning.
		// Otherwise, keep the current resumed state.
		if runner.CurrentState == state.Idle {
			transitionErr = runner.TransitionTo(state.Planning)
		}
	}

	if transitionErr != nil {
		http.Error(w, "Failed to execute state transition: "+transitionErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // HTTP 202 Accepted
	_ = json.NewEncoder(w).Encode(ExecuteResponse{
		SessionID:    runner.SessionID,
		CurrentState: string(runner.CurrentState),
		Payload:      runner.Payload,
	})
}

// StateHandler handles the GET /api/v1/agent/state/{session_id} endpoint.
func (h *APIHandler) StateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Retrieve session_id path parameter (Go 1.22+ standard path wildcard extraction)
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}

	checkpoint, err := h.checkpointer.RestoreCheckpoint(sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to restore checkpoint: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(checkpoint)
}
