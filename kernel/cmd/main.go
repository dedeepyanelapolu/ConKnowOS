package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"contextos/kernel/config"
	"contextos/kernel/internal/a2a"
	"contextos/kernel/internal/api"
	"contextos/kernel/internal/knowledge"
	"contextos/kernel/internal/mcp"
	"contextos/kernel/internal/memory"
	"contextos/kernel/internal/state"
	"contextos/mcp_servers/memory_mcp"
	"github.com/contextos/knowledge_mcp"

	_ "github.com/lib/pq"
)

func main() {
	cfg := config.LoadConfig()

	mcpRouter := mcp.NewRouter()
	a2aRouter := a2a.NewRouter()
	eventBus := a2a.NewEventBus()

	mcpRouter.RegisterServer("mcp_memory", "Redis & Qdrant Memory Server", "in-memory")
	mcpRouter.RegisterServer("mcp_knowledge", "Neo4j & Postgres Knowledge Server", "in-memory")
	mcpRouter.RegisterServer("mcp_tools", "Native Tools Server", "in-memory")

	// Initialize Memory MCP Server
	memServer := memory_mcp.NewMemoryServer(cfg.RedisURL, cfg.QdrantURL)

	// Register tools directly on the router in main.go to avoid circular/internal package import issues
	_ = mcpRouter.RegisterTool(mcp.ToolInfo{
		Name:        "store_working_memory",
		Description: "Store key-value state in Redis working memory",
		ServerID:    "mcp_memory",
		Parameters:  map[string]interface{}{"session_id": "string", "key": "string", "value": "any"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			sessionID, _ := args["session_id"].(string)
			key, _ := args["key"].(string)
			value := args["value"]
			if sessionID == "" || key == "" {
				return nil, fmt.Errorf("session_id and key are required")
			}
			err := memServer.StoreWorkingMemory(sessionID, key, value)
			return map[string]interface{}{"status": "stored", "key": key}, err
		},
	})

	_ = mcpRouter.RegisterTool(mcp.ToolInfo{
		Name:        "get_working_memory",
		Description: "Retrieve stored key-value state from Redis",
		ServerID:    "mcp_memory",
		Parameters:  map[string]interface{}{"session_id": "string", "key": "string"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			sessionID, _ := args["session_id"].(string)
			key, _ := args["key"].(string)
			if sessionID == "" || key == "" {
				return nil, fmt.Errorf("session_id and key are required")
			}
			val, err := memServer.GetWorkingMemory(sessionID, key)
			if err != nil {
				return map[string]interface{}{"found": false, "key": key, "value": nil}, nil
			}
			return map[string]interface{}{"found": true, "key": key, "value": val}, nil
		},
	})

	_ = mcpRouter.RegisterTool(mcp.ToolInfo{
		Name:        "store_episodic_memory",
		Description: "Index a semantic fact into Qdrant vector memory",
		ServerID:    "mcp_memory",
		Parameters:  map[string]interface{}{"session_id": "string", "text": "string", "metadata": "map"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			sessionID, _ := args["session_id"].(string)
			text, _ := args["text"].(string)
			meta, _ := args["metadata"].(map[string]interface{})
			if sessionID == "" || text == "" {
				return nil, fmt.Errorf("session_id and text are required")
			}
			err := memServer.StoreEpisodicMemory(sessionID, text, meta)
			return map[string]interface{}{"status": "indexed"}, err
		},
	})

	_ = mcpRouter.RegisterTool(mcp.ToolInfo{
		Name:        "search_episodic_memory",
		Description: "Perform semantic similarity search on Qdrant episodic memory",
		ServerID:    "mcp_memory",
		Parameters:  map[string]interface{}{"query": "string", "limit": "int"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			query, _ := args["query"].(string)
			limitVal := args["limit"]
			limit := 3
			if limitVal != nil {
				switch v := limitVal.(type) {
				case float64:
					limit = int(v)
				case int:
					limit = v
				}
			}
			if query == "" {
				return nil, fmt.Errorf("query is required")
			}
			results, err := memServer.SearchEpisodicMemory(query, limit)
			return map[string]interface{}{"query": query, "results": results}, err
		},
	})

	_ = mcpRouter.RegisterTool(mcp.ToolInfo{
		Name:        "delete_episodic_memory",
		Description: "Delete vector records from Qdrant episodic memory",
		ServerID:    "mcp_memory",
		Parameters:  map[string]interface{}{"fact_ids": "array"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			rawIDs := args["fact_ids"]
			var ids []string
			if rawIDs != nil {
				if slice, ok := rawIDs.([]interface{}); ok {
					for _, item := range slice {
						if idStr, ok := item.(string); ok {
							ids = append(ids, idStr)
						}
					}
				} else if strSlice, ok := rawIDs.([]string); ok {
					ids = strSlice
				}
			}
			err := memServer.DeleteEpisodicMemory(ids)
			return map[string]interface{}{"status": "deleted"}, err
		},
	})

	// Initialize Knowledge MCP Server
	knowServer := knowledge_mcp.NewKnowledgeServer(cfg.Neo4jURI, cfg.Neo4jUser, cfg.Neo4jPassword, cfg.PostgresDSN)

	// Register knowledge tools directly on the router
	_ = mcpRouter.RegisterTool(mcp.ToolInfo{
		Name:        "query_graph",
		Description: "Query entity relationships in Neo4j knowledge graph",
		ServerID:    "mcp_knowledge",
		Parameters:  map[string]interface{}{"cypher_query": "string", "params": "map"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			cypher, _ := args["cypher_query"].(string)
			params, _ := args["params"].(map[string]interface{})
			return knowServer.QueryGraph(cypher, params)
		},
	})

	_ = mcpRouter.RegisterTool(mcp.ToolInfo{
		Name:        "add_graph_relation",
		Description: "Add entity-relationship edge to Neo4j knowledge graph",
		ServerID:    "mcp_knowledge",
		Parameters:  map[string]interface{}{"source_entity": "string", "relation": "string", "target_entity": "string", "properties": "map"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			src, _ := args["source_entity"].(string)
			rel, _ := args["relation"].(string)
			tgt, _ := args["target_entity"].(string)
			props, _ := args["properties"].(map[string]interface{})
			err := knowServer.AddGraphRelation(src, rel, tgt, props)
			return map[string]interface{}{"status": "success"}, err
		},
	})

	_ = mcpRouter.RegisterTool(mcp.ToolInfo{
		Name:        "query_relational",
		Description: "Query relational attributes from PostgreSQL database",
		ServerID:    "mcp_knowledge",
		Parameters:  map[string]interface{}{"sql_query": "string", "args": "array"},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			sqlQuery, _ := args["sql_query"].(string)
			rawArgs := args["args"]
			var sqlArgs []interface{}
			if rawArgs != nil {
				if slice, ok := rawArgs.([]interface{}); ok {
					sqlArgs = slice
				}
			}
			return knowServer.QueryRelational(sqlQuery, sqlArgs)
		},
	})

	// Initialize KnowledgeRouter and ContextManager
	knowledgeRouter := knowledge.NewKnowledgeRouter(mcpRouter)
	contextMgr := memory.NewContextManager(mcpRouter, knowledgeRouter)

	stateEngine := state.NewWorkflowEngine()

	a2aRouter.RegisterAgent("agent_planner", "Strategic Planner Agent", []string{"planning", "decomposition"})
	a2aRouter.RegisterAgent("agent_executor", "Tool Execution Agent", []string{"mcp_dispatch", "coding"})

	// Connect to PostgreSQL Relational Storage
	db, err := sql.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Initialize state persistence checkpointer
	checkpointer := state.NewCheckpointer(db)
	if err := checkpointer.InitSchema(); err != nil {
		log.Printf("Warning: Failed to initialize database schema: %v. Checkpoint features will fail.", err)
	}

	// Initialize ReflectionEngine
	reflection := memory.NewReflectionEngine(contextMgr, db)

	// Initialize API Handler with checkpointer and context manager
	apiHandler := api.NewAPIHandler(checkpointer, contextMgr)
	apiHandler.SetEventBus(eventBus)
	apiHandler.SetReflectionEngine(reflection)

	// Create WSHub
	wsHub := api.NewWSHub(eventBus)

	// Set up ServeMux using Go 1.22+ routing standard patterns
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"system":    "ContextOS Go Microkernel",
			"version":   "1.0.0",
			"timestamp": time.Now().Unix(),
		})
	})

	mux.HandleFunc("GET /api/v1/mcp/tools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tools": mcpRouter.ListTools(),
		})
	})

	mux.HandleFunc("GET /api/v1/a2a/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"agents": a2aRouter.ListAgents(),
		})
	})

	// Add execution and state query REST endpoints
	mux.HandleFunc("POST /api/v1/agent/execute", apiHandler.ExecuteHandler)
	mux.HandleFunc("GET /api/v1/agent/state/{session_id}", apiHandler.StateHandler)
	mux.Handle("/ws/trace", wsHub)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	fmt.Printf("ContextOS Go Microkernel running on %s\n", addr)

	_ = stateEngine

	// Start HTTP Server
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
