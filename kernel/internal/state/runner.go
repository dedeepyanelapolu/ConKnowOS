package state

import (
	"errors"
	"fmt"
	"sync"
)

// WorkflowState represents the state of a workflow execution.
type WorkflowState string

const (
	StateIdle      WorkflowState = "Idle"
	StatePlanning  WorkflowState = "Planning"
	StateExecuting WorkflowState = "Executing"
	StateReview    WorkflowState = "Review"
	StateCompleted WorkflowState = "Completed"
	StateFailed    WorkflowState = "Failed"
)

// String returns string representation of WorkflowState.
func (s WorkflowState) String() string {
	return string(s)
}

// IsValid checks if state is a recognized WorkflowState.
func (s WorkflowState) IsValid() bool {
	switch s {
	case StateIdle, StatePlanning, StateExecuting, StateReview, StateCompleted, StateFailed:
		return true
	default:
		return false
	}
}

// TaskNode represents a node in a task execution graph.
type TaskNode struct {
	ID           string   `json:"id"`
	Task         string   `json:"task"`
	Dependencies []string `json:"dependencies"`
	Status       string   `json:"status"` // pending, running, completed, failed
}

// TaskGraph manages the execution graph for workflow tasks.
type TaskGraph struct {
	Nodes map[string]*TaskNode `json:"nodes"`
}

// NewTaskGraph initializes a new TaskGraph.
func NewTaskGraph() *TaskGraph {
	return &TaskGraph{
		Nodes: make(map[string]*TaskNode),
	}
}

// AddTask adds a task to the execution graph.
func (g *TaskGraph) AddTask(id, task string, dependencies ...string) {
	if g.Nodes == nil {
		g.Nodes = make(map[string]*TaskNode)
	}
	g.Nodes[id] = &TaskNode{
		ID:           id,
		Task:         task,
		Dependencies: dependencies,
		Status:       "pending",
	}
}

// WorkflowRunner manages task execution graphs, holds active session state, and executes state transitions.
type WorkflowRunner struct {
	mu           sync.RWMutex
	SessionID    string                 `json:"session_id"`
	CurrentState WorkflowState          `json:"current_state"`
	Payload      map[string]interface{} `json:"payload"`
	TaskGraph    *TaskGraph             `json:"task_graph"`
	checkpointer Checkpointer
}

// NewWorkflowRunner creates a new WorkflowRunner instance.
func NewWorkflowRunner(sessionID string, checkpointer Checkpointer) *WorkflowRunner {
	if checkpointer == nil {
		checkpointer = NewInMemoryCheckpointer()
	}
	return &WorkflowRunner{
		SessionID:    sessionID,
		CurrentState: StateIdle,
		Payload:      make(map[string]interface{}),
		TaskGraph:    NewTaskGraph(),
		checkpointer: checkpointer,
	}
}

// RestoreWorkflowRunner restores a WorkflowRunner instance from an existing checkpoint.
func RestoreWorkflowRunner(sessionID string, checkpointer Checkpointer) (*WorkflowRunner, error) {
	if checkpointer == nil {
		return nil, errors.New("checkpointer cannot be nil")
	}

	cp, err := checkpointer.RestoreCheckpoint(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to restore workflow state: %w", err)
	}

	runner := &WorkflowRunner{
		SessionID:    cp.SessionID,
		CurrentState: WorkflowState(cp.CurrentState),
		Payload:      cp.Payload,
		TaskGraph:    NewTaskGraph(),
		checkpointer: checkpointer,
	}

	if runner.Payload == nil {
		runner.Payload = make(map[string]interface{})
	}

	return runner, nil
}

// ValidateTransition checks whether a transition from currentState to targetState is allowed.
func (r *WorkflowRunner) ValidateTransition(targetState WorkflowState) error {
	if !targetState.IsValid() {
		return fmt.Errorf("invalid target state: %s", targetState)
	}

	currentState := r.CurrentState

	// Same state transition is allowed
	if currentState == targetState {
		return nil
	}

	validTransitions := map[WorkflowState][]WorkflowState{
		StateIdle:      {StatePlanning},
		StatePlanning:  {StateExecuting, StateFailed},
		StateExecuting: {StateReview, StateCompleted, StateFailed},
		StateReview:    {StateExecuting, StatePlanning, StateCompleted, StateFailed},
		StateCompleted: {StateIdle},
		StateFailed:    {StateIdle, StatePlanning},
	}

	allowed, exists := validTransitions[currentState]
	if !exists {
		return fmt.Errorf("unrecognized current state: %s", currentState)
	}

	for _, s := range allowed {
		if s == targetState {
			return nil
		}
	}

	return fmt.Errorf("invalid state transition from '%s' to '%s'", currentState, targetState)
}

// TransitionTo executes a state transition and automatically saves a checkpoint.
func (r *WorkflowRunner) TransitionTo(targetState WorkflowState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ValidateTransition(targetState); err != nil {
		return err
	}

	r.CurrentState = targetState

	// Checkpoints are written automatically during every state transition
	if r.checkpointer != nil {
		if err := r.checkpointer.SaveCheckpoint(r.SessionID, string(r.CurrentState), r.Payload); err != nil {
			return fmt.Errorf("failed to write checkpoint during state transition to %s: %w", targetState, err)
		}
	}

	return nil
}

// UpdatePayload updates the session payload and automatically saves a checkpoint.
func (r *WorkflowRunner) UpdatePayload(payload map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Payload == nil {
		r.Payload = make(map[string]interface{})
	}

	for k, v := range payload {
		r.Payload[k] = v
	}

	if r.checkpointer != nil {
		if err := r.checkpointer.SaveCheckpoint(r.SessionID, string(r.CurrentState), r.Payload); err != nil {
			return fmt.Errorf("failed to save checkpoint after updating payload: %w", err)
		}
	}

	return nil
}

// GetState returns the current workflow state.
func (r *WorkflowRunner) GetState() WorkflowState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.CurrentState
}

// GetPayload returns a copy of the current payload.
func (r *WorkflowRunner) GetPayload() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	payloadCopy := make(map[string]interface{})
	for k, v := range r.Payload {
		payloadCopy[k] = v
	}
	return payloadCopy
}
