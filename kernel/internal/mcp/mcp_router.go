package mcp

import (
	"fmt"
	"sync"
	"time"
)

type ServerInfo struct {
	ServerID     string `json:"server_id"`
	Name         string `json:"name"`
	Transport    string `json:"transport"`
	Status       string `json:"status"`
	RegisteredAt int64  `json:"registered_at"`
	ToolsCount   int    `json:"tools_count"`
}

type ToolHandler func(args map[string]interface{}) (interface{}, error)

type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	ServerID    string                 `json:"server_id"`
	Parameters  map[string]interface{} `json:"parameters"`
	Handler     ToolHandler            `json:"-"`
}

type Router struct {
	mu      sync.RWMutex
	servers map[string]*ServerInfo
	tools   map[string]ToolInfo
}

func NewRouter() *Router {
	return &Router{
		servers: make(map[string]*ServerInfo),
		tools:   make(map[string]ToolInfo),
	}
}

func (r *Router) RegisterServer(serverID, name, transport string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.servers[serverID] = &ServerInfo{
		ServerID:     serverID,
		Name:         name,
		Transport:    transport,
		Status:       "connected",
		RegisteredAt: time.Now().Unix(),
		ToolsCount:   0,
	}
}

func (r *Router) RegisterTool(info ToolInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[info.Name] = info
	if server, ok := r.servers[info.ServerID]; ok {
		server.ToolsCount++
	}
	return nil
}

func (r *Router) ListTools() []ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]ToolInfo, 0, len(r.tools))
	for _, t := range r.tools {
		res = append(res, t)
	}
	return res
}

func (r *Router) ListServers() []ServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]ServerInfo, 0, len(r.servers))
	for _, s := range r.servers {
		res = append(res, *s)
	}
	return res
}

func (r *Router) Dispatch(toolName string, args map[string]interface{}) (interface{}, error) {
	r.mu.RLock()
	tool, exists := r.tools[toolName]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool %s not registered", toolName)
	}

	if tool.Handler != nil {
		return tool.Handler(args)
	}

	return fmt.Sprintf("Executed tool %s on server %s with args %v", tool.Name, tool.ServerID, args), nil
}
