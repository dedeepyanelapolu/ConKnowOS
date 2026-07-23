package knowledge_mcp

import (
	"testing"
)

func TestKnowledgeServer_Graph(t *testing.T) {
	// Start in fallback mode (empty parameters) to guarantee offline execution
	server := NewKnowledgeServer("", "", "", "")
	defer server.Close()

	// 1. Query graph neighbors for ContextOS
	results, err := server.query_graph("MATCH (s)-[r]->(t) WHERE s.name = $entity RETURN s, r, t", map[string]interface{}{"entity": "ContextOS"})
	if err != nil {
		t.Fatalf("failed to query graph: %v", err)
	}

	// We expect 3 relationships seeded: HAS_CORE to Microkernel, USES_PROTOCOL to MCP, USES_PROTOCOL to A2A
	if len(results) != 3 {
		t.Errorf("expected 3 connections for ContextOS, got %d", len(results))
	}

	// 2. Add relation
	err = server.add_graph_relation("MCP", "EXTENDS", "JSON-RPC", map[string]interface{}{"spec": "v2.0"})
	if err != nil {
		t.Fatalf("failed to add relationship: %v", err)
	}

	if count := server.GetEdgesCount(); count != 4 {
		t.Errorf("expected 4 edges, got %d", count)
	}

	// Query for MCP connections
	results, err = server.query_graph("", map[string]interface{}{"entity": "MCP"})
	if err != nil {
		t.Fatalf("failed to query graph: %v", err)
	}

	// MCP has 2 edges in fallback: incoming USES_PROTOCOL and outgoing EXTENDS
	if len(results) != 2 {
		t.Errorf("expected 2 connections for MCP, got %d", len(results))
	}
}

func TestKnowledgeServer_Relational(t *testing.T) {
	server := NewKnowledgeServer("", "", "", "")
	defer server.Close()

	// Query relational attributes for Microkernel
	results, err := server.query_relational("SELECT * FROM entity_attributes WHERE entity_key = $1", []interface{}{"Microkernel"})
	if err != nil {
		t.Fatalf("failed to query relational: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 row, got %d", len(results))
	}

	row := results[0]
	if row["entity_key"] != "Microkernel" {
		t.Errorf("expected entity_key to be Microkernel, got %v", row["entity_key"])
	}
	if row["performance"] != "ultra-fast" {
		t.Errorf("expected performance ultra-fast, got %v", row["performance"])
	}
}
