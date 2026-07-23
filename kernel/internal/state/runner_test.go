package state

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestWorkflowRunnerTransitions(t *testing.T) {
	// Setup sqlmock
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	checkpointer := NewCheckpointer(db)
	runner := NewWorkflowRunner("test-session-123", checkpointer)

	// Verify initial state is Idle
	if runner.CurrentState != Idle {
		t.Errorf("expected initial state to be Idle, got %s", runner.CurrentState)
	}

	// 1. Verify invalid transition: Idle -> Review
	err = runner.TransitionTo(Review)
	if err == nil {
		t.Errorf("expected error when transitioning from Idle to Review, got nil")
	}

	// 2. Verify valid transition: Idle -> Planning
	runner.Payload["test-key"] = "test-value"
	payloadBytes, _ := json.Marshal(runner.Payload)

	mock.ExpectExec("INSERT INTO workflow_checkpoints").
		WithArgs("test-session-123", string(Planning), payloadBytes).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = runner.TransitionTo(Planning)
	if err != nil {
		t.Errorf("unexpected error transitioning from Idle to Planning: %v", err)
	}

	if runner.CurrentState != Planning {
		t.Errorf("expected state to be Planning, got %s", runner.CurrentState)
	}

	// 3. Verify valid transition: Planning -> Executing
	mock.ExpectExec("INSERT INTO workflow_checkpoints").
		WithArgs("test-session-123", string(Executing), payloadBytes).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = runner.TransitionTo(Executing)
	if err != nil {
		t.Errorf("unexpected error transitioning from Planning to Executing: %v", err)
	}

	// 4. Verify invalid transition: Executing -> Idle
	err = runner.TransitionTo(Idle)
	if err == nil {
		t.Errorf("expected error when transitioning from Executing to Idle, got nil")
	}

	// Ensure mock expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestWorkflowRunnerTaskGraph(t *testing.T) {
	runner := NewWorkflowRunner("test-session-tasks", nil)

	taskA := &Task{ID: "task-A", Name: "Task A", DependsOn: []string{}, Completed: false}
	taskB := &Task{ID: "task-B", Name: "Task B", DependsOn: []string{"task-A"}, Completed: false}
	taskC := &Task{ID: "task-C", Name: "Task C", DependsOn: []string{"task-A", "task-B"}, Completed: false}

	// Add tasks
	if err := runner.AddTask(taskA); err != nil {
		t.Fatalf("failed to add task A: %v", err)
	}
	if err := runner.AddTask(taskB); err != nil {
		t.Fatalf("failed to add task B: %v", err)
	}
	if err := runner.AddTask(taskC); err != nil {
		t.Fatalf("failed to add task C: %v", err)
	}

	// Self dependency check
	taskSelf := &Task{ID: "task-self", Name: "Self", DependsOn: []string{"task-self"}}
	if err := runner.AddTask(taskSelf); err == nil {
		t.Errorf("expected error when adding self-referencing task, got nil")
	}

	// Initially, only task A is executable
	execTasks := runner.GetExecutableTasks()
	if len(execTasks) != 1 || execTasks[0].ID != "task-A" {
		t.Errorf("expected only task-A to be executable, got: %+v", execTasks)
	}

	// Mark A completed
	taskA.Completed = true
	execTasks = runner.GetExecutableTasks()
	if len(execTasks) != 1 || execTasks[0].ID != "task-B" {
		t.Errorf("expected only task-B to be executable, got: %+v", execTasks)
	}

	// Mark B completed
	taskB.Completed = true
	execTasks = runner.GetExecutableTasks()
	if len(execTasks) != 1 || execTasks[0].ID != "task-C" {
		t.Errorf("expected only task-C to be executable, got: %+v", execTasks)
	}

	// Mark C completed
	taskC.Completed = true
	execTasks = runner.GetExecutableTasks()
	if len(execTasks) != 0 {
		t.Errorf("expected no executable tasks, got: %+v", execTasks)
	}
}

func TestCheckpointer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	checkpointer := NewCheckpointer(db)

	// Test InitSchema
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS workflow_checkpoints").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := checkpointer.InitSchema(); err != nil {
		t.Errorf("unexpected error in InitSchema: %v", err)
	}

	// Test SaveCheckpoint
	payload := map[string]interface{}{"step": 1.0}
	payloadBytes, _ := json.Marshal(payload)

	mock.ExpectExec("INSERT INTO workflow_checkpoints").
		WithArgs("session-abc", "Planning", payloadBytes).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := checkpointer.SaveCheckpoint("session-abc", "Planning", payload); err != nil {
		t.Errorf("unexpected error in SaveCheckpoint: %v", err)
	}

	// Test RestoreCheckpoint
	now := time.Now()
	rows := sqlmock.NewRows([]string{"session_id", "current_state", "payload", "created_at", "updated_at"}).
		AddRow("session-abc", "Planning", payloadBytes, now, now)

	mock.ExpectQuery("SELECT session_id, current_state, payload, created_at, updated_at FROM workflow_checkpoints").
		WithArgs("session-abc").
		WillReturnRows(rows)

	cp, err := checkpointer.RestoreCheckpoint("session-abc")
	if err != nil {
		t.Fatalf("unexpected error in RestoreCheckpoint: %v", err)
	}

	if cp.SessionID != "session-abc" {
		t.Errorf("expected session-abc, got %s", cp.SessionID)
	}
	if cp.CurrentState != "Planning" {
		t.Errorf("expected Planning state, got %s", cp.CurrentState)
	}
	if cp.Payload["step"] != 1.0 {
		t.Errorf("expected payload step to be 1.0, got %v", cp.Payload["step"])
	}

	// Test RestoreCheckpoint not found
	mock.ExpectQuery("SELECT session_id, current_state, payload, created_at, updated_at FROM workflow_checkpoints").
		WithArgs("session-nonexistent").
		WillReturnError(sql.ErrNoRows)

	_, err = checkpointer.RestoreCheckpoint("session-nonexistent")
	if err == nil {
		t.Errorf("expected error for nonexistent checkpoint, got nil")
	}

	// Ensure mock expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
