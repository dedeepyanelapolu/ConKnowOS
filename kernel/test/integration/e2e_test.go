package integration

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"contextos/kernel/internal/a2a"
	"contextos/kernel/internal/api"
	"contextos/kernel/internal/mcp"
	"contextos/kernel/internal/memory"
	"contextos/kernel/internal/state"
	sdk "github.com/contextos/sdk/go"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestEndToEndLifecycle(t *testing.T) {
	// 1. Setup mock DB using sqlmock
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// 2. Setup checkpointer, mcpRouter, eventBus, and contextManager
	checkpointer := state.NewCheckpointer(db)
	mcpRouter := mcp.NewRouter()
	mcpRouter.RegisterServer("mcp_memory", "Memory", "in-memory")

	// Register tools to prevent missing tool errors
	_ = mcpRouter.RegisterTool(mcp.ToolInfo{
		Name:     "search_episodic_memory",
		ServerID: "mcp_memory",
		Handler: func(args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{"results": []interface{}{}}, nil
		},
	})

	eventBus := a2a.NewEventBus()
	contextMgr := memory.NewContextManager(mcpRouter, nil)
	reflection := memory.NewReflectionEngine(contextMgr, db)

	// 3. Setup API Handler and wire eventBus / reflection
	apiHandler := api.NewAPIHandler(checkpointer, contextMgr)
	apiHandler.SetEventBus(eventBus)
	apiHandler.SetReflectionEngine(reflection)

	// 4. Setup Test HTTP Server
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/agent/execute", apiHandler.ExecuteHandler)
	mux.HandleFunc("GET /api/v1/agent/state/{session_id}", apiHandler.StateHandler)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Setup expectations for POST /api/v1/agent/execute
	// First request: tries to restore checkpoint -> returns sql.ErrNoRows -> inserts initial state checkpoint
	mock.ExpectQuery("SELECT .* FROM workflow_checkpoints").
		WithArgs("session-test-e2e").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec("INSERT INTO workflow_checkpoints").
		WithArgs("session-test-e2e", "Planning", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 5. Use Go SDK Client to execute task
	client := sdk.NewClient(ts.URL)

	resp, err := client.ExecuteAgent("session-test-e2e", "Assemble Microkernel Graph", map[string]interface{}{})
	if err != nil {
		t.Fatalf("SDK ExecuteAgent failed: %v", err)
	}

	if resp.SessionID != "session-test-e2e" {
		t.Errorf("expected session_id 'session-test-e2e', got %s", resp.SessionID)
	}
	if resp.CurrentState != "Planning" {
		t.Errorf("expected current_state 'Planning', got %s", resp.CurrentState)
	}

	// 6. Test GET state details via SDK
	// Mock returns the Planning checkpoint with 5 columns
	payloadBytes, _ := json.Marshal(map[string]interface{}{"task": "Assemble Microkernel Graph"})
	rows := sqlmock.NewRows([]string{"session_id", "current_state", "payload", "created_at", "updated_at"}).
		AddRow("session-test-e2e", "Planning", payloadBytes, time.Now(), time.Now())

	mock.ExpectQuery("SELECT .* FROM workflow_checkpoints").
		WithArgs("session-test-e2e").
		WillReturnRows(rows)

	stateResp, err := client.GetState("session-test-e2e")
	if err != nil {
		t.Fatalf("SDK GetState failed: %v", err)
	}

	if stateResp.CurrentState != "Planning" {
		t.Errorf("expected state 'Planning', got %s", stateResp.CurrentState)
	}

	// Verify SQL expectations were fully met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations not met: %v", err)
	}
}
