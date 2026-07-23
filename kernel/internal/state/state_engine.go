package state

import (
	"fmt"
	"sync"
	"time"
)

type StepStatus string

const (
	StatusPending   StepStatus = "PENDING"
	StatusRunning   StepStatus = "RUNNING"
	StatusCompleted StepStatus = "COMPLETED"
	StatusFailed    StepStatus = "FAILED"
)

type WorkflowStep struct {
	StepID    string                 `json:"step_id"`
	Name      string                 `json:"name"`
	ToolName  string                 `json:"tool_name"`
	Inputs    map[string]interface{} `json:"inputs"`
	DependsOn []string               `json:"depends_on"`
	Status    StepStatus             `json:"status"`
	Output    interface{}            `json:"output"`
	Error     string                 `json:"error"`
	LatencyMs float64                `json:"latency_ms"`
	TokenCost float64                `json:"token_cost"`
}

type StepCheckpoint struct {
	CheckpointID   string     `json:"checkpoint_id"`
	Timestamp      int64      `json:"timestamp"`
	WorkflowID     string     `json:"workflow_id"`
	Note           string     `json:"note"`
	Status         StepStatus `json:"status"`
	CompletedSteps []string   `json:"completed_steps"`
}

type Workflow struct {
	WorkflowID  string                   `json:"workflow_id"`
	Name        string                   `json:"name"`
	Status      StepStatus               `json:"status"`
	CreatedAt   int64                    `json:"created_at"`
	UpdatedAt   int64                    `json:"updated_at"`
	Steps       map[string]*WorkflowStep `json:"steps"`
	Checkpoints []StepCheckpoint         `json:"checkpoints"`
}

type WorkflowEngine struct {
	mu        sync.RWMutex
	workflows map[string]*Workflow
}

func NewWorkflowEngine() *WorkflowEngine {
	return &WorkflowEngine{
		workflows: make(map[string]*Workflow),
	}
}

func (e *WorkflowEngine) CreateWorkflow(wfID, name string) *Workflow {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf := &Workflow{
		WorkflowID:  wfID,
		Name:        name,
		Status:      StatusPending,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
		Steps:       make(map[string]*WorkflowStep),
		Checkpoints: make([]StepCheckpoint, 0),
	}

	e.workflows[wfID] = wf
	return wf
}

func (e *WorkflowEngine) AddStep(wfID string, step *WorkflowStep) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, exists := e.workflows[wfID]
	if !exists {
		return fmt.Errorf("workflow %s not found", wfID)
	}

	step.Status = StatusPending
	wf.Steps[step.StepID] = step
	return nil
}
