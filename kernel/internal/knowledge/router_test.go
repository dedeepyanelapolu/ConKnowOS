package knowledge

import (
	"strings"
	"testing"

	"contextos/kernel/internal/mcp"
)

func TestKnowledgeRouter_FetchKnowledgeContext(t *testing.T) {
	router := mcp.NewRouter()
	router.RegisterServer("mcp_knowledge", "Knowledge", "in-memory")

	// Register Mock Graph Tool Handler
	_ = router.RegisterTool(mcp.ToolInfo{
		Name:     "query_graph",
		ServerID: "mcp_knowledge",
		Handler: func(args map[string]interface{}) (interface{}, error) {
			params, _ := args["params"].(map[string]interface{})
			entity, _ := params["entity"].(string)
			if entity == "ContextOS" {
				return []map[string]interface{}{
					{"source": "ContextOS", "relation": "HAS_CORE", "target": "Microkernel"},
				}, nil
			}
			return []map[string]interface{}{}, nil
		},
	})

	// Register Mock Relational Tool Handler
	_ = router.RegisterTool(mcp.ToolInfo{
		Name:     "query_relational",
		ServerID: "mcp_knowledge",
		Handler: func(args map[string]interface{}) (interface{}, error) {
			sqlArgs, _ := args["args"].([]interface{})
			if len(sqlArgs) > 0 && sqlArgs[0] == "ContextOS" {
				return []map[string]interface{}{
					{"version": "1.0.0", "status": "stable"},
				}, nil
			}
			return []map[string]interface{}{}, nil
		},
	})

	kr := NewKnowledgeRouter(router)

	ctxBlock, err := kr.FetchKnowledgeContext([]string{"ContextOS"})
	if err != nil {
		t.Fatalf("failed to fetch knowledge context: %v", err)
	}

	// Verify formatted text blocks are merged cleanly
	if !strings.Contains(ctxBlock, "=== KNOWLEDGE GRAPH & DATA CONTEXT ===") {
		t.Errorf("expected header block missing")
	}
	if !strings.Contains(ctxBlock, "- (ContextOS) -[HAS_CORE]-> (Microkernel)") {
		t.Errorf("expected graph relation missing")
	}
	if !strings.Contains(ctxBlock, "* version: 1.0.0") || !strings.Contains(ctxBlock, "* status: stable") {
		t.Errorf("expected SQL attributes missing")
	}
}
