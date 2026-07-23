package state

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"contextos/kernel/internal/a2a"
	"contextos/kernel/internal/memory"
)

// WorkflowState represents the current execution state of a workflow runner.
type WorkflowState string

const (
	Idle      WorkflowState = "Idle"
	Planning  WorkflowState = "Planning"
	Executing WorkflowState = "Executing"
	Review    WorkflowState = "Review"
	Completed WorkflowState = "Completed"
	Failed    WorkflowState = "Failed"
)

// Task represents a node in the task execution graph managed by the WorkflowRunner.
type Task struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	DependsOn []string `json:"depends_on"`
	Completed bool     `json:"completed"`
}

// WorkflowRunner manages task execution graphs, active session state, and executes state transitions.
type WorkflowRunner struct {
	mu           sync.RWMutex
	SessionID    string                   `json:"session_id"`
	CurrentState WorkflowState            `json:"current_state"`
	Payload      map[string]interface{}   `json:"payload"`
	Tasks        map[string]*Task         `json:"tasks"`
	Checkpointer *Checkpointer            `json:"-"`
	ContextMgr   *memory.ContextManager   `json:"-"`
	EventBus     *a2a.EventBus            `json:"-"`
	Reflection   *memory.ReflectionEngine `json:"-"`
}

// NewWorkflowRunner creates a new WorkflowRunner.
func NewWorkflowRunner(sessionID string, checkpointer *Checkpointer) *WorkflowRunner {
	return &WorkflowRunner{
		SessionID:    sessionID,
		CurrentState: Idle,
		Payload:      make(map[string]interface{}),
		Tasks:        make(map[string]*Task),
		Checkpointer: checkpointer,
	}
}

// SetContextManager assigns the ContextManager instance.
func (r *WorkflowRunner) SetContextManager(cm *memory.ContextManager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ContextMgr = cm
}

// SetEventBus assigns the A2A EventBus instance.
func (r *WorkflowRunner) SetEventBus(eb *a2a.EventBus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.EventBus = eb
}

// SetReflectionEngine assigns the ReflectionEngine instance.
func (r *WorkflowRunner) SetReflectionEngine(re *memory.ReflectionEngine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Reflection = re
}

// Valid transition mapping
var validTransitions = map[WorkflowState]map[WorkflowState]bool{
	Idle: {
		Planning: true,
		Failed:   true,
	},
	Planning: {
		Executing: true,
		Failed:    true,
	},
	Executing: {
		Review: true,
		Failed: true,
	},
	Review: {
		Executing: true,
		Completed: true,
		Failed:    true,
	},
	Completed: {},
	Failed: {
		Idle:     true,
		Planning: true,
	},
}

// IsValidTransition checks if transitioning from 'from' state to 'to' state is permitted.
func IsValidTransition(from, to WorkflowState) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// TransitionTo performs state transition validation and updates the current state.
// It also automatically persists the new state to the database via the checkpointer.
// In transitions to Planning and Executing, it retrieves, ranks, and compacts episodic memories.
// Upon state updates, transition events are published to the EventBus. When Completed, Reflection is triggered.
func (r *WorkflowRunner) TransitionTo(to WorkflowState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !IsValidTransition(r.CurrentState, to) {
		return fmt.Errorf("invalid state transition from %s to %s", r.CurrentState, to)
	}

	// Dynamic context integration on Planning / Executing transitions
	if (to == Planning || to == Executing) && r.ContextMgr != nil {
		query := "general"
		if task, ok := r.Payload["task"].(string); ok && task != "" {
			query = task
		}

		// Retrieve candidate vector memories (up to 5)
		candidates, err := r.ContextMgr.SearchEpisodicMemory(query, 5)
		if err == nil {
			// Rank them combining similarity and age decay
			ranked := r.ContextMgr.Rank(candidates)

			// Publish EVENT_MEMORY_RETRIEVED event
			if r.EventBus != nil {
				var vectorTexts []string
				for _, c := range ranked {
					vectorTexts = append(vectorTexts, c.Text)
				}
				r.EventBus.Publish(a2a.A2AEvent{
					EventID:   fmt.Sprintf("evt_mem_%d", time.Now().UnixNano()),
					EventType: "EVENT_MEMORY_RETRIEVED",
					Sender:    "WorkflowRunner",
					Payload: map[string]interface{}{
						"session_id":   r.SessionID,
						"vector_facts": vectorTexts,
					},
				})
			}

			// Map memories to standard system messages
			var messages []memory.Message
			for _, c := range ranked {
				messages = append(messages, memory.Message{
					Role:      "system",
					Content:   c.Text,
					Timestamp: c.Timestamp,
				})
			}

			// Capture conversation history from payload if present
			if histVal, exists := r.Payload["history"]; exists {
				if histBytes, err := json.Marshal(histVal); err == nil {
					var histMsgs []memory.Message
					if err := json.Unmarshal(histBytes, &histMsgs); err == nil {
						messages = append(messages, histMsgs...)
					}
				}
			}

			// Compact history to stay within a 1000 token budget
			compacted, err := memory.CompactContext(messages, 1000)
			if err == nil {
				r.Payload["compacted_context"] = compacted

				// Publish EVENT_TOKEN_COMPACTED event
				if r.EventBus != nil {
					totalInput := 0
					for _, m := range messages {
						totalInput += memory.EstimateTokens(m.Content)
					}
					totalOutput := 0
					for _, m := range compacted {
						totalOutput += memory.EstimateTokens(m.Content)
					}
					r.EventBus.Publish(a2a.A2AEvent{
						EventID:   fmt.Sprintf("evt_comp_%d", time.Now().UnixNano()),
						EventType: "EVENT_TOKEN_COMPACTED",
						Sender:    "WorkflowRunner",
						Payload: map[string]interface{}{
							"session_id":     r.SessionID,
							"input_tokens":   totalInput,
							"compact_tokens": totalOutput,
							"cost_saved":     float64(totalInput-totalOutput) * 0.000002, // Mock cost saved
						},
					})
				}
			}
		}
	}

	fromState := r.CurrentState
	r.CurrentState = to

	// Persist checkpoint in PostgreSQL
	if r.Checkpointer != nil {
		if err := r.Checkpointer.SaveCheckpoint(r.SessionID, string(to), r.Payload); err != nil {
			return fmt.Errorf("failed to automatically save checkpoint on transition: %w", err)
		}
	}

	// Publish state change event to A2A Bus
	if r.EventBus != nil {
		r.EventBus.Publish(a2a.A2AEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", r.SessionID, time.Now().UnixNano()),
			EventType: "EVENT_STATE_CHANGED",
			Sender:    "WorkflowRunner",
			Payload: map[string]interface{}{
				"session_id": r.SessionID,
				"from_state": string(fromState),
				"to_state":   string(to),
			},
		})
	}

	// Trigger Reflection Engine when completed
	if to == Completed && r.Reflection != nil {
		r.Reflection.Reflect(r.SessionID)
	}

	return nil
}

// AddTask adds a new task node to the execution graph.
func (r *WorkflowRunner) AddTask(task *Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if task.ID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	for _, depID := range task.DependsOn {
		if depID == task.ID {
			return fmt.Errorf("task cannot depend on itself: %s", task.ID)
		}
	}

	r.Tasks[task.ID] = task
	return nil
}

// GetExecutableTasks returns a slice of tasks that have all dependencies met and are not yet completed.
func (r *WorkflowRunner) GetExecutableTasks() []*Task {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var executable []*Task
	for _, task := range r.Tasks {
		if task.Completed {
			continue
		}

		dependenciesMet := true
		for _, depID := range task.DependsOn {
			dep, exists := r.Tasks[depID]
			if !exists || !dep.Completed {
				dependenciesMet = false
				break
			}
		}

		if dependenciesMet {
			executable = append(executable, task)
		}
	}

	return executable
}
