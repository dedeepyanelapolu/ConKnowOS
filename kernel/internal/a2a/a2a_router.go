package a2a

import (
	"sync"
	"time"
)

type AgentInfo struct {
	AgentID       string   `json:"agent_id"`
	Name          string   `json:"name"`
	Capabilities  []string `json:"capabilities"`
	Status        string   `json:"status"`
	LastHeartbeat int64    `json:"last_heartbeat"`
}

type EventMessage struct {
	EventID   string      `json:"event_id"`
	Timestamp int64       `json:"timestamp"`
	Topic     string      `json:"topic"`
	SenderID  string      `json:"sender_id"`
	Payload   interface{} `json:"payload"`
}

type Router struct {
	mu     sync.RWMutex
	agents map[string]*AgentInfo
	events []EventMessage
}

func NewRouter() *Router {
	return &Router{
		agents: make(map[string]*AgentInfo),
		events: make([]EventMessage, 0),
	}
}

func (r *Router) RegisterAgent(agentID, name string, caps []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents[agentID] = &AgentInfo{
		AgentID:       agentID,
		Name:          name,
		Capabilities:  caps,
		Status:        "active",
		LastHeartbeat: time.Now().Unix(),
	}
}

func (r *Router) PublishEvent(eventID, topic, sender string, payload interface{}) EventMessage {
	r.mu.Lock()
	defer r.mu.Unlock()

	msg := EventMessage{
		EventID:   eventID,
		Timestamp: time.Now().Unix(),
		Topic:     topic,
		SenderID:  sender,
		Payload:   payload,
	}

	r.events = append(r.events, msg)
	return msg
}

func (r *Router) ListAgents() []AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]AgentInfo, 0, len(r.agents))
	for _, a := range r.agents {
		res = append(res, *a)
	}
	return res
}
