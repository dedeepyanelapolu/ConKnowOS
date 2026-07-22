package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"contextos/kernel/internal/state"
)

// ExecuteRequest defines the payload for POST /api/v1/agent/execute.
type ExecuteRequest struct {
	SessionID string                 `json:"session_id"`
	Task      string                 `json:"task"`
	Context   map[string]interface{} `json:"context"`
}

// ExecuteResponse defines the response structure for POST /api/v1/agent/execute.
type ExecuteResponse struct {
	SessionID string                 `json:"session_id"`
	Status    string                 `json:"status"`
	Task      string                 `json:"task"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Message   string                 `json:"message"`
}

// ErrorResponse defines the standard error response structure.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Handler handles HTTP requests for the Workflow Runner API.
type Handler struct {
	checkpointer state.Checkpointer
}

// NewHandler initializes a new Handler.
func NewHandler(checkpointer state.Checkpointer) *Handler {
	if checkpointer == nil {
		checkpointer = state.NewInMemoryCheckpointer()
	}
	return &Handler{
		checkpointer: checkpointer,
	}
}

// RegisterRoutes registers the REST API endpoints on the given http.ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/agent/execute", h.HandleExecute)
	mux.HandleFunc("/api/v1/agent/state/", h.HandleGetState)
}

// HandleExecute processes POST /api/v1/agent/execute.
func (h *Handler) HandleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON request body"})
		return
	}

	if strings.TrimSpace(req.SessionID) == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "session_id is required"})
		return
	}

	var runner *state.WorkflowRunner
	var err error

	// Try to restore existing checkpoint or initialize a new instance
	runner, err = state.RestoreWorkflowRunner(req.SessionID, h.checkpointer)
	if err != nil {
		runner = state.NewWorkflowRunner(req.SessionID, h.checkpointer)
	}

	// Update payload with task and context
	payload := map[string]interface{}{
		"task":    req.Task,
		"context": req.Context,
	}
	if err := runner.UpdatePayload(payload); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// Trigger initial transition: Idle -> Planning
	if runner.GetState() == state.StateIdle {
		if err := runner.TransitionTo(state.StatePlanning); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
	}

	resp := ExecuteResponse{
		SessionID: runner.SessionID,
		Status:    runner.GetState().String(),
		Task:      req.Task,
		Payload:   runner.GetPayload(),
		Message:   "Workflow execution initiated successfully",
	}

	writeJSON(w, http.StatusAccepted, resp)
}

// HandleGetState processes GET /api/v1/agent/state/:session_id.
func (h *Handler) HandleGetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	// Extract session_id from URL path (/api/v1/agent/state/:session_id)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agent/state/")
	sessionID := strings.Trim(path, "/")

	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "session_id path parameter is required"})
		return
	}

	cp, err := h.checkpointer.RestoreCheckpoint(sessionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, cp)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
