package memory

import (
	"database/sql"
	"fmt"
	"math"
	"sync"
)

// ReflectionEngine consolidates session memories, removes duplicates, and extracts lessons learned.
type ReflectionEngine struct {
	contextMgr *ContextManager
	db         *sql.DB
	mu         sync.Mutex
}

// NewReflectionEngine creates a new ReflectionEngine instance.
func NewReflectionEngine(contextMgr *ContextManager, db *sql.DB) *ReflectionEngine {
	re := &ReflectionEngine{
		contextMgr: contextMgr,
		db:         db,
	}
	_ = re.InitSchema()
	return re
}

// InitSchema sets up the SQL schema for persistent lessons learned.
func (re *ReflectionEngine) InitSchema() error {
	if re.db == nil {
		return nil
	}
	query := `
	CREATE TABLE IF NOT EXISTS workflow_lessons (
		id SERIAL PRIMARY KEY,
		session_id TEXT NOT NULL,
		lesson TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := re.db.Exec(query)
	return err
}

// Reflect runs deduplication and rule extraction asynchronously.
func (re *ReflectionEngine) Reflect(sessionID string) {
	go func() {
		_ = re.ExecuteReflection(sessionID)
	}()
}

// ExecuteReflection performs the actual reflection processing.
func (re *ReflectionEngine) ExecuteReflection(sessionID string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	// 1. Retrieve candidates from episodic vector memory
	candidates, err := re.contextMgr.SearchEpisodicMemory("general", 50)
	if err != nil {
		return fmt.Errorf("failed to fetch episodic memories: %w", err)
	}

	// Filter memories belonging to the active session
	var sessionMemories []CandidateMemory
	for _, c := range candidates {
		if c.Metadata != nil {
			if sid, ok := c.Metadata["session_id"].(string); ok && sid == sessionID {
				sessionMemories = append(sessionMemories, c)
			}
		}
	}

	if len(sessionMemories) == 0 {
		return nil
	}

	// 2. Identify duplicate or redundant facts
	var toDelete []string
	deletedMap := make(map[string]bool)

	for i := 0; i < len(sessionMemories); i++ {
		cI := sessionMemories[i]
		if deletedMap[cI.FactID] {
			continue
		}

		for j := i + 1; j < len(sessionMemories); j++ {
			cJ := sessionMemories[j]
			if deletedMap[cJ.FactID] {
				continue
			}

			// Compute cosine similarity between their text mock embeddings
			v1 := getEmbedding(cI.Text)
			v2 := getEmbedding(cJ.Text)
			similarity := cosineSimilarity(v1, v2)

			// Threshold 0.90 indicates high semantic redundancy
			if similarity > 0.90 {
				// Keep the one with higher score, mark the other for deletion
				if cI.Score >= cJ.Score {
					toDelete = append(toDelete, cJ.FactID)
					deletedMap[cJ.FactID] = true
				} else {
					toDelete = append(toDelete, cI.FactID)
					deletedMap[cI.FactID] = true
					break // cI is marked for deletion, no need to compare further
				}
			}
		}
	}

	// 3. Remove low-score vector duplicates
	if len(toDelete) > 0 {
		_ = re.contextMgr.DeleteEpisodicMemory(toDelete)
	}

	// 4. Extract "Lessons Learned" and save to PostgreSQL database
	// Filter out deleted memories to synthesize lessons
	var uniqueMemories []CandidateMemory
	for _, c := range sessionMemories {
		if !deletedMap[c.FactID] {
			uniqueMemories = append(uniqueMemories, c)
		}
	}

	if re.db != nil && len(uniqueMemories) > 0 {
		// Create a simple procedural lesson learned record
		lessonText := fmt.Sprintf("Procedural Rule: Unified session %s has %d unique semantic facts. Ensure deduplicated execution paths.", sessionID, len(uniqueMemories))
		query := `INSERT INTO workflow_lessons (session_id, lesson, created_at) VALUES ($1, $2, NOW())`
		_, err = re.db.Exec(query, sessionID, lessonText)
		if err != nil {
			return fmt.Errorf("failed to save lessons learned: %w", err)
		}
	}

	return nil
}

// GetLessonsCount returns total lessons stored for a session (for testing).
func (re *ReflectionEngine) GetLessonsCount(sessionID string) (int, error) {
	if re.db == nil {
		return 0, nil
	}
	var count int
	err := re.db.QueryRow("SELECT COUNT(*) FROM workflow_lessons WHERE session_id = $1", sessionID).Scan(&count)
	return count, err
}

// getEmbedding generates a deterministic mock vector embedding of dimension 4.
func getEmbedding(text string) []float32 {
	var hash int
	for _, char := range text {
		hash = int(char) + (hash << 5) - hash
	}
	if hash < 0 {
		hash = -hash
	}
	// generate 4 components based on hash directly spanning from -1.0 to 1.0
	vector := []float32{
		float32(hash%100)/50.0 - 1.0,
		float32((hash/100)%100)/50.0 - 1.0,
		float32((hash/10000)%100)/50.0 - 1.0,
		float32((hash/1000000)%100)/50.0 - 1.0,
	}
	return vector
}

// cosineSimilarity calculates cosine distance between two 1D float32 slices.
func cosineSimilarity(v1, v2 []float32) float64 {
	if len(v1) != len(v2) || len(v1) == 0 {
		return 0.0
	}
	var dot, norm1, norm2 float32
	for i := 0; i < len(v1); i++ {
		dot += v1[i] * v2[i]
		norm1 += v1[i] * v1[i]
		norm2 += v2[i] * v2[i]
	}
	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}
	return float64(dot / (float32(math.Sqrt(float64(norm1))) * float32(math.Sqrt(float64(norm2)))))
}
