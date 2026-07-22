package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"contextos/kernel/internal/api"
	"contextos/kernel/internal/state"
)

func TestAPI_ExecuteAndGetState(t *testing.T) {
	cp := state.NewInMemoryCheckpointer()
	handler := api.NewHandler(cp)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 1. Test POST /api/v1/agent/execute
	reqBody := api.ExecuteRequest{
		SessionID: "api-session-123",
		Task:      "Initialize Phase 1 Engine",
		Context: map[string]interface{}{
			"env": "production",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/execute", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d. Body: %s", w.Code, w.Body.String())
	}

	var execResp api.ExecuteResponse
	if err := json.Unmarshal(w.Body.Bytes(), &execResp); err != nil {
		t.Fatalf("failed to parse execute response JSON: %v", err)
	}

	if execResp.SessionID != "api-session-123" {
		t.Errorf("expected session_id 'api-session-123', got '%s'", execResp.SessionID)
	}
	if execResp.Status != "Planning" {
		t.Errorf("expected status 'Planning', got '%s'", execResp.Status)
	}

	// 2. Test GET /api/v1/agent/state/:session_id
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/agent/state/api-session-123", nil)
	getW := httptest.NewRecorder()

	mux.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", getW.Code, getW.Body.String())
	}

	var cpState state.Checkpoint
	if err := json.Unmarshal(getW.Body.Bytes(), &cpState); err != nil {
		t.Fatalf("failed to parse checkpoint JSON: %v", err)
	}

	if cpState.SessionID != "api-session-123" {
		t.Errorf("expected session_id 'api-session-123', got '%s'", cpState.SessionID)
	}
	if cpState.CurrentState != "Planning" {
		t.Errorf("expected state 'Planning', got '%s'", cpState.CurrentState)
	}
}

func TestAPI_ExecuteInvalidRequests(t *testing.T) {
	cp := state.NewInMemoryCheckpointer()
	handler := api.NewHandler(cp)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test missing session_id
	reqBody := api.ExecuteRequest{
		Task: "No Session Task",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/execute", bytes.NewBuffer(bodyBytes))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing session_id, got %d", w.Code)
	}

	// Test GET non-existent session
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/agent/state/non-existent", nil)
	getW := httptest.NewRecorder()

	mux.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for non-existent session, got %d", getW.Code)
	}
}
