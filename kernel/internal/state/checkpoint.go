package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// Checkpoint represents a persistent snapshot of a workflow execution session.
type Checkpoint struct {
	SessionID    string                 `json:"session_id" db:"session_id"`
	CurrentState string                 `json:"current_state" db:"current_state"`
	Payload      map[string]interface{} `json:"payload" db:"payload"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
}

// Checkpointer defines the contract for checkpoint persistence.
type Checkpointer interface {
	SaveCheckpoint(sessionID string, state string, payload map[string]interface{}) error
	RestoreCheckpoint(sessionID string) (*Checkpoint, error)
}

// PostgresCheckpointer implements Checkpointer using PostgreSQL.
type PostgresCheckpointer struct {
	db *sql.DB
}

// NewPostgresCheckpointer initializes a new PostgresCheckpointer.
func NewPostgresCheckpointer(db *sql.DB) *PostgresCheckpointer {
	return &PostgresCheckpointer{db: db}
}

// InitSchema creates the workflow_checkpoints table if it does not exist.
func (p *PostgresCheckpointer) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS workflow_checkpoints (
		session_id TEXT PRIMARY KEY,
		current_state TEXT NOT NULL,
		payload JSONB NOT NULL DEFAULT '{}'::jsonb,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := p.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to initialize workflow_checkpoints schema: %w", err)
	}
	return nil
}

// SaveCheckpoint saves or updates a checkpoint in PostgreSQL.
func (p *PostgresCheckpointer) SaveCheckpoint(sessionID string, state string, payload map[string]interface{}) error {
	if payload == nil {
		payload = make(map[string]interface{})
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint payload: %w", err)
	}

	query := `
	INSERT INTO workflow_checkpoints (session_id, current_state, payload, created_at, updated_at)
	VALUES ($1, $2, $3, NOW(), NOW())
	ON CONFLICT (session_id)
	DO UPDATE SET
		current_state = EXCLUDED.current_state,
		payload = EXCLUDED.payload,
		updated_at = NOW();`

	_, err = p.db.Exec(query, sessionID, state, string(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to save checkpoint for session %s: %w", sessionID, err)
	}
	return nil
}

// RestoreCheckpoint retrieves a checkpoint from PostgreSQL.
func (p *PostgresCheckpointer) RestoreCheckpoint(sessionID string) (*Checkpoint, error) {
	query := `
	SELECT session_id, current_state, payload, created_at, updated_at
	FROM workflow_checkpoints
	WHERE session_id = $1;`

	var cp Checkpoint
	var payloadBytes []byte

	err := p.db.QueryRow(query, sessionID).Scan(
		&cp.SessionID,
		&cp.CurrentState,
		&payloadBytes,
		&cp.CreatedAt,
		&cp.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("checkpoint not found for session_id: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to restore checkpoint for session %s: %w", sessionID, err)
	}

	if len(payloadBytes) > 0 {
		cp.Payload = make(map[string]interface{})
		if err := json.Unmarshal(payloadBytes, &cp.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal checkpoint payload: %w", err)
		}
	}

	return &cp, nil
}

// InMemoryCheckpointer provides an in-memory implementation of Checkpointer for testing or fallback.
type InMemoryCheckpointer struct {
	mu          sync.RWMutex
	checkpoints map[string]*Checkpoint
}

// NewInMemoryCheckpointer initializes a new InMemoryCheckpointer.
func NewInMemoryCheckpointer() *InMemoryCheckpointer {
	return &InMemoryCheckpointer{
		checkpoints: make(map[string]*Checkpoint),
	}
}

// SaveCheckpoint saves or updates a checkpoint in memory.
func (m *InMemoryCheckpointer) SaveCheckpoint(sessionID string, state string, payload map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cp, exists := m.checkpoints[sessionID]
	if !exists {
		cp = &Checkpoint{
			SessionID: sessionID,
			CreatedAt: now,
		}
		m.checkpoints[sessionID] = cp
	}

	cp.CurrentState = state
	cp.Payload = payload
	cp.UpdatedAt = now
	return nil
}

// RestoreCheckpoint fetches a checkpoint from memory.
func (m *InMemoryCheckpointer) RestoreCheckpoint(sessionID string) (*Checkpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cp, exists := m.checkpoints[sessionID]
	if !exists {
		return nil, fmt.Errorf("checkpoint not found for session_id: %s", sessionID)
	}

	// Return deep copy of payload to prevent race conditions
	payloadCopy := make(map[string]interface{})
	for k, v := range cp.Payload {
		payloadCopy[k] = v
	}

	return &Checkpoint{
		SessionID:    cp.SessionID,
		CurrentState: cp.CurrentState,
		Payload:      payloadCopy,
		CreatedAt:    cp.CreatedAt,
		UpdatedAt:    cp.UpdatedAt,
	}, nil
}
