package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"contextos/kernel/internal/state"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestExecuteHandler_NewSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	checkpointer := state.NewCheckpointer(db)
	handler := NewAPIHandler(checkpointer, nil)

	// Since it's a new session, RestoreCheckpoint will return sql.ErrNoRows
	mock.ExpectQuery("SELECT session_id, current_state, payload, created_at, updated_at FROM workflow_checkpoints").
		WithArgs("session-new").
		WillReturnError(sql.ErrNoRows)

	// Transition Idle -> Planning will write to DB
	payload := map[string]interface{}{
		"task":    "Test Task",
		"context": map[string]interface{}{"user_id": "123"},
	}
	payloadBytes, _ := json.Marshal(payload)

	mock.ExpectExec("INSERT INTO workflow_checkpoints").
		WithArgs("session-new", string(state.Planning), payloadBytes).
		WillReturnResult(sqlmock.NewResult(1, 1))

	reqBody, _ := json.Marshal(ExecuteRequest{
		SessionID: "session-new",
		Task:      "Test Task",
		Context:   map[string]interface{}{"user_id": "123"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/execute", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()

	handler.ExecuteHandler(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.SessionID != "session-new" {
		t.Errorf("expected session_id to be 'session-new', got '%s'", resp.SessionID)
	}
	if resp.CurrentState != string(state.Planning) {
		t.Errorf("expected state to be Planning, got '%s'", resp.CurrentState)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestStateHandler(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	checkpointer := state.NewCheckpointer(db)
	handler := NewAPIHandler(checkpointer, nil)

	payload := map[string]interface{}{"task": "Test Task"}
	payloadBytes, _ := json.Marshal(payload)
	now := time.Now()

	mock.ExpectQuery("SELECT session_id, current_state, payload, created_at, updated_at FROM workflow_checkpoints").
		WithArgs("session-test").
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "current_state", "payload", "created_at", "updated_at"}).
			AddRow("session-test", "Planning", payloadBytes, now, now))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/state/session-test", nil)

	// Use standard ServeMux to test routing and path parameter wildcard matching
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/agent/state/{session_id}", handler.StateHandler)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var cp state.Checkpoint
	if err := json.NewDecoder(rr.Body).Decode(&cp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if cp.SessionID != "session-test" {
		t.Errorf("expected session-test, got %s", cp.SessionID)
	}
	if cp.CurrentState != "Planning" {
		t.Errorf("expected state to be Planning, got %s", cp.CurrentState)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}
