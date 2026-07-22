package state_test

import (
	"encoding/json"
	"testing"

	"contextos/kernel/internal/state"
)

func TestWorkflowState_Validation(t *testing.T) {
	validStates := []state.WorkflowState{
		state.StateIdle,
		state.StatePlanning,
		state.StateExecuting,
		state.StateReview,
		state.StateCompleted,
		state.StateFailed,
	}

	for _, s := range validStates {
		if !s.IsValid() {
			t.Errorf("expected state %s to be valid", s)
		}
	}

	invalidState := state.WorkflowState("UnknownState")
	if invalidState.IsValid() {
		t.Errorf("expected invalidState to be invalid")
	}
}

func TestWorkflowRunner_ValidTransitions(t *testing.T) {
	cp := state.NewInMemoryCheckpointer()
	runner := state.NewWorkflowRunner("sess-001", cp)

	if runner.GetState() != state.StateIdle {
		t.Fatalf("expected initial state to be Idle, got %s", runner.GetState())
	}

	// Idle -> Planning
	if err := runner.TransitionTo(state.StatePlanning); err != nil {
		t.Fatalf("unexpected error on transition Idle -> Planning: %v", err)
	}
	if runner.GetState() != state.StatePlanning {
		t.Errorf("expected state Planning, got %s", runner.GetState())
	}

	// Verify checkpoint was updated
	checkpoint, err := cp.RestoreCheckpoint("sess-001")
	if err != nil {
		t.Fatalf("failed to restore checkpoint: %v", err)
	}
	if checkpoint.CurrentState != string(state.StatePlanning) {
		t.Errorf("expected checkpoint state Planning, got %s", checkpoint.CurrentState)
	}

	// Planning -> Executing
	if err := runner.TransitionTo(state.StateExecuting); err != nil {
		t.Fatalf("unexpected error on transition Planning -> Executing: %v", err)
	}

	// Executing -> Review
	if err := runner.TransitionTo(state.StateReview); err != nil {
		t.Fatalf("unexpected error on transition Executing -> Review: %v", err)
	}

	// Review -> Executing
	if err := runner.TransitionTo(state.StateExecuting); err != nil {
		t.Fatalf("unexpected error on transition Review -> Executing: %v", err)
	}

	// Executing -> Completed
	if err := runner.TransitionTo(state.StateCompleted); err != nil {
		t.Fatalf("unexpected error on transition Executing -> Completed: %v", err)
	}

	if runner.GetState() != state.StateCompleted {
		t.Errorf("expected final state Completed, got %s", runner.GetState())
	}
}

func TestWorkflowRunner_InvalidTransitions(t *testing.T) {
	cp := state.NewInMemoryCheckpointer()
	runner := state.NewWorkflowRunner("sess-002", cp)

	// Attempt Idle -> Review (should fail)
	err := runner.TransitionTo(state.StateReview)
	if err == nil {
		t.Fatalf("expected error when transitioning Idle -> Review, got nil")
	}

	// Attempt Idle -> Executing (should fail)
	err = runner.TransitionTo(state.StateExecuting)
	if err == nil {
		t.Fatalf("expected error when transitioning Idle -> Executing, got nil")
	}

	// Move to Planning
	_ = runner.TransitionTo(state.StatePlanning)

	// Attempt Planning -> Completed (should fail)
	err = runner.TransitionTo(state.StateCompleted)
	if err == nil {
		t.Fatalf("expected error when transitioning Planning -> Completed, got nil")
	}

	// Attempt invalid target state string
	err = runner.TransitionTo(state.WorkflowState("InvalidState"))
	if err == nil {
		t.Fatalf("expected error when transitioning to non-existent state, got nil")
	}
}

func TestWorkflowRunner_CheckpointSerialization(t *testing.T) {
	cp := state.NewInMemoryCheckpointer()
	runner := state.NewWorkflowRunner("sess-003", cp)

	payloadData := map[string]interface{}{
		"task":    "Build Core Engine",
		"priority": 1,
		"meta": map[string]interface{}{
			"owner": "Systems Lead",
		},
	}

	if err := runner.UpdatePayload(payloadData); err != nil {
		t.Fatalf("failed to update payload: %v", err)
	}

	if err := runner.TransitionTo(state.StatePlanning); err != nil {
		t.Fatalf("failed transition: %v", err)
	}

	// Restore runner from checkpoint
	restoredRunner, err := state.RestoreWorkflowRunner("sess-003", cp)
	if err != nil {
		t.Fatalf("failed to restore workflow runner: %v", err)
	}

	if restoredRunner.GetState() != state.StatePlanning {
		t.Errorf("expected restored state Planning, got %s", restoredRunner.GetState())
	}

	restoredPayload := restoredRunner.GetPayload()
	if restoredPayload["task"] != "Build Core Engine" {
		t.Errorf("expected task payload 'Build Core Engine', got %v", restoredPayload["task"])
	}

	// Test JSON marshaling of Checkpoint
	cpObject, err := cp.RestoreCheckpoint("sess-003")
	if err != nil {
		t.Fatalf("failed to restore raw checkpoint: %v", err)
	}

	jsonData, err := json.Marshal(cpObject)
	if err != nil {
		t.Fatalf("failed to marshal checkpoint: %v", err)
	}

	var unmarshaled state.Checkpoint
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal checkpoint JSON: %v", err)
	}

	if unmarshaled.SessionID != "sess-003" || unmarshaled.CurrentState != "Planning" {
		t.Errorf("mismatch in unmarshaled checkpoint struct: %+v", unmarshaled)
	}
}

func TestWorkflowRunner_TaskGraph(t *testing.T) {
	runner := state.NewWorkflowRunner("sess-004", nil)
	runner.TaskGraph.AddTask("t1", "Initialize Engine")
	runner.TaskGraph.AddTask("t2", "Execute Phase 1", "t1")

	if len(runner.TaskGraph.Nodes) != 2 {
		t.Errorf("expected 2 task nodes, got %d", len(runner.TaskGraph.Nodes))
	}

	t2Node := runner.TaskGraph.Nodes["t2"]
	if t2Node == nil || len(t2Node.Dependencies) != 1 || t2Node.Dependencies[0] != "t1" {
		t.Errorf("expected task t2 with dependency t1, got %+v", t2Node)
	}
}

func TestInMemoryCheckpointer_NotFound(t *testing.T) {
	cp := state.NewInMemoryCheckpointer()
	_, err := cp.RestoreCheckpoint("non-existent-session")
	if err == nil {
		t.Fatalf("expected error when restoring non-existent session, got nil")
	}
}
