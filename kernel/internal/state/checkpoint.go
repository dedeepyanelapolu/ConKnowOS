package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Checkpoint represents the persisted workflow execution state.
type Checkpoint struct {
	SessionID    string                 `json:"session_id"`
	CurrentState string                 `json:"current_state"`
	Payload      map[string]interface{} `json:"payload"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// Checkpointer handles saving and restoring workflow checkpoints to/from PostgreSQL.
type Checkpointer struct {
	db *sql.DB
}

// NewCheckpointer creates a new Checkpointer instance.
func NewCheckpointer(db *sql.DB) *Checkpointer {
	return &Checkpointer{db: db}
}

// InitSchema creates the workflow_checkpoints table if it does not exist.
func (c *Checkpointer) InitSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS workflow_checkpoints (
		session_id TEXT PRIMARY KEY,
		current_state TEXT NOT NULL,
		payload JSONB NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := c.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	return nil
}

// SaveCheckpoint saves or updates a workflow checkpoint in the database.
func (c *Checkpointer) SaveCheckpoint(sessionID string, state string, payload map[string]interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
	INSERT INTO workflow_checkpoints (session_id, current_state, payload, created_at, updated_at)
	VALUES ($1, $2, $3, NOW(), NOW())
	ON CONFLICT (session_id) DO UPDATE SET
		current_state = EXCLUDED.current_state,
		payload = EXCLUDED.payload,
		updated_at = NOW();`

	_, err = c.db.Exec(query, sessionID, state, payloadBytes)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}
	return nil
}

// RestoreCheckpoint retrieves a workflow checkpoint from the database.
func (c *Checkpointer) RestoreCheckpoint(sessionID string) (*Checkpoint, error) {
	query := `
	SELECT session_id, current_state, payload, created_at, updated_at
	FROM workflow_checkpoints
	WHERE session_id = $1;`

	row := c.db.QueryRow(query, sessionID)

	var cp Checkpoint
	var payloadBytes []byte

	err := row.Scan(&cp.SessionID, &cp.CurrentState, &payloadBytes, &cp.CreatedAt, &cp.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("checkpoint not found for session %s", sessionID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to restore checkpoint: %w", err)
	}

	if err := json.Unmarshal(payloadBytes, &cp.Payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return &cp, nil
}
