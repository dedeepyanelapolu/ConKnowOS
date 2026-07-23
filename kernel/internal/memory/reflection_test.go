package memory

import (
	"testing"

	"contextos/kernel/internal/mcp"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestReflectionEngine_ExecuteReflection(t *testing.T) {
	// 1. Setup mock mcpRouter and ContextManager
	router := mcp.NewRouter()
	router.RegisterServer("mcp_memory", "Memory", "in-memory")

	// Store items in memory mock handlers
	// Pre-populate with two highly similar facts for session-123
	memories := []map[string]interface{}{
		{
			"fact_id": "1",
			"text":    "Clean architecture is solid",
			"score":   0.95,
			"metadata": map[string]interface{}{
				"session_id": "session-123",
			},
		},
		{
			"fact_id": "2",
			"text":    "Clean architecture is solid",
			"score":   0.88, // lower score duplicate
			"metadata": map[string]interface{}{
				"session_id": "session-123",
			},
		},
		{
			"fact_id": "3",
			"text":    "Something else completely different",
			"score":   0.99,
			"metadata": map[string]interface{}{
				"session_id": "session-123",
			},
		},
	}

	deletedIDs := make(map[string]bool)

	_ = router.RegisterTool(mcp.ToolInfo{
		Name:     "search_episodic_memory",
		ServerID: "mcp_memory",
		Handler: func(args map[string]interface{}) (interface{}, error) {
			var active []map[string]interface{}
			for _, m := range memories {
				id, _ := m["fact_id"].(string)
				if !deletedIDs[id] {
					active = append(active, m)
				}
			}
			return map[string]interface{}{"results": active}, nil
		},
	})

	_ = router.RegisterTool(mcp.ToolInfo{
		Name:     "delete_episodic_memory",
		ServerID: "mcp_memory",
		Handler: func(args map[string]interface{}) (interface{}, error) {
			rawIDs := args["fact_ids"]
			if slice, ok := rawIDs.([]string); ok {
				for _, id := range slice {
					deletedIDs[id] = true
				}
			} else if slice, ok := rawIDs.([]interface{}); ok {
				for _, item := range slice {
					if idStr, ok := item.(string); ok {
						deletedIDs[idStr] = true
					}
				}
			}
			return map[string]interface{}{"status": "deleted"}, nil
		},
	})

	cm := NewContextManager(router, nil)

	// 2. Setup sqlmock
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// InitSchema expectation
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS workflow_lessons").
		WillReturnResult(sqlmock.NewResult(0, 0))

	re := NewReflectionEngine(cm, db)

	// ExecuteReflection expectation (should insert lesson rule)
	mock.ExpectExec("INSERT INTO workflow_lessons").
		WithArgs("session-123", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = re.ExecuteReflection("session-123")
	if err != nil {
		t.Fatalf("reflection failed: %v", err)
	}

	// Verify that the duplicate fact ("2") was deleted
	if !deletedIDs["2"] {
		t.Errorf("expected duplicate fact '2' to be deleted, but it wasn't")
	}

	// Verify that unique fact "1" and "3" were kept
	if deletedIDs["1"] {
		t.Errorf("expected fact '1' (higher score) to be kept, but it was deleted")
	}
	if deletedIDs["3"] {
		t.Errorf("expected fact '3' to be kept, but it was deleted")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}
