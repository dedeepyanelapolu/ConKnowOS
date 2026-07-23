package knowledge

import (
	"fmt"
	"strings"

	"contextos/kernel/internal/mcp"
)

// KnowledgeRouter interfaces with the Knowledge MCP Server tools.
type KnowledgeRouter struct {
	mcpRouter *mcp.Router
}

// NewKnowledgeRouter creates a new KnowledgeRouter.
func NewKnowledgeRouter(mcpRouter *mcp.Router) *KnowledgeRouter {
	return &KnowledgeRouter{
		mcpRouter: mcpRouter,
	}
}

// FetchKnowledgeContext retrieves graph relations from Neo4j alongside relational attributes from PostgreSQL,
// merging them into a structured prompt context block.
func (kr *KnowledgeRouter) FetchKnowledgeContext(entityKeys []string) (string, error) {
	if len(entityKeys) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("=== KNOWLEDGE GRAPH & DATA CONTEXT ===\n")

	for _, key := range entityKeys {
		// 1. Fetch graph connections using query_graph
		cypher := "MATCH (s)-[r]->(t) WHERE s.name = $entity OR t.name = $entity RETURN s.name, type(r), t.name"
		graphArgs := map[string]interface{}{
			"cypher_query": cypher,
			"params": map[string]interface{}{
				"entity": key,
			},
		}

		resGraph, err := kr.mcpRouter.Dispatch("query_graph", graphArgs)
		if err == nil {
			if list, ok := resGraph.([]map[string]interface{}); ok && len(list) > 0 {
				builder.WriteString(fmt.Sprintf("Relationships for '%s':\n", key))
				for _, edge := range list {
					src, _ := edge["source"].(string)
					rel, _ := edge["relation"].(string)
					tgt, _ := edge["target"].(string)
					builder.WriteString(fmt.Sprintf("  - (%s) -[%s]-> (%s)\n", src, rel, tgt))
				}
			}
		}

		// 2. Fetch SQL attributes using query_relational
		sqlQuery := "SELECT * FROM entity_attributes WHERE entity_key = $1"
		sqlArgs := map[string]interface{}{
			"sql_query": sqlQuery,
			"args":      []interface{}{key},
		}

		resRel, err := kr.mcpRouter.Dispatch("query_relational", sqlArgs)
		if err == nil {
			if list, ok := resRel.([]map[string]interface{}); ok && len(list) > 0 {
				builder.WriteString(fmt.Sprintf("Attributes for '%s':\n", key))
				for _, row := range list {
					for k, v := range row {
						if k == "entity_key" {
							continue
						}
						builder.WriteString(fmt.Sprintf("  * %s: %v\n", k, v))
					}
				}
			}
		}
	}

	return builder.String(), nil
}
