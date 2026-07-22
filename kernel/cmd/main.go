package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"contextos/kernel/config"
	"contextos/kernel/internal/a2a"
	"contextos/kernel/internal/mcp"
	"contextos/kernel/internal/memory"
	"contextos/kernel/internal/state"
)

func main() {
	cfg := config.LoadConfig()

	memMgr := memory.NewContextManager()
	stateEngine := state.NewWorkflowEngine()
	mcpRouter := mcp.NewRouter()
	a2aRouter := a2a.NewRouter()

	mcpRouter.RegisterServer("mcp_memory", "Redis & Qdrant Memory Server", "in-memory")
	mcpRouter.RegisterServer("mcp_knowledge", "Neo4j & Postgres Knowledge Server", "in-memory")
	mcpRouter.RegisterServer("mcp_tools", "Native Tools Server", "in-memory")

	a2aRouter.RegisterAgent("agent_planner", "Strategic Planner Agent", []string{"planning", "decomposition"})
	a2aRouter.RegisterAgent("agent_executor", "Tool Execution Agent", []string{"mcp_dispatch", "coding"})

	http.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"system":    "ContextOS Go Microkernel",
			"version":   "1.0.0",
			"timestamp": time.Now().Unix(),
		})
	})

	http.HandleFunc("/api/v1/mcp/tools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tools": mcpRouter.ListTools(),
		})
	})

	http.HandleFunc("/api/v1/a2a/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agents": a2aRouter.ListAgents(),
		})
	})

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	fmt.Printf("ContextOS Go Microkernel running on %s\n", addr)
	_ = memMgr
	_ = stateEngine
}
