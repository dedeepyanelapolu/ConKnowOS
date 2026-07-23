package memory

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"contextos/kernel/internal/knowledge"
	"contextos/kernel/internal/mcp"
)

// CandidateMemory represents a vector memory candidate for prompt construction.
type CandidateMemory struct {
	FactID    string                 `json:"fact_id"`
	Text      string                 `json:"text"`
	Metadata  map[string]interface{} `json:"metadata"`
	Score     float64                `json:"score"`
	Timestamp int64                  `json:"timestamp"`
}

// ContextManager coordinates working and episodic memory retrievals.
type ContextManager struct {
	mcpRouter       *mcp.Router
	knowledgeRouter *knowledge.KnowledgeRouter
}

// NewContextManager creates a new ContextManager.
func NewContextManager(mcpRouter *mcp.Router, knowledgeRouter *knowledge.KnowledgeRouter) *ContextManager {
	return &ContextManager{
		mcpRouter:       mcpRouter,
		knowledgeRouter: knowledgeRouter,
	}
}

// BuildPromptContext compiles system instructions, compacted history, vector facts, and graph attributes.
func (cm *ContextManager) BuildPromptContext(sessionID string, systemPrompt string, entityKeys []string, maxHistoryTokens int) (map[string]interface{}, error) {
	// 1. Fetch vector episodic memories (default search query can be "general" or first entity key)
	query := "general"
	if len(entityKeys) > 0 {
		query = entityKeys[0]
	}
	candidates, err := cm.SearchEpisodicMemory(query, 5)
	var vectorFacts []string
	if err == nil {
		ranked := cm.Rank(candidates)
		for _, c := range ranked {
			vectorFacts = append(vectorFacts, c.Text)
		}
	}

	// 2. Fetch knowledge context from Neo4j & Postgres
	knowledgeCtx := ""
	if cm.knowledgeRouter != nil {
		kCtx, err := cm.knowledgeRouter.FetchKnowledgeContext(entityKeys)
		if err == nil {
			knowledgeCtx = kCtx
		}
	}

	// 3. Compact conversation history if present in working memory
	var history []Message
	if stateVal, err := cm.GetSessionState(sessionID, "history"); err == nil {
		if bytes, err := json.Marshal(stateVal); err == nil {
			_ = json.Unmarshal(bytes, &history)
		}
	}

	compactedHistory, _ := CompactContext(history, maxHistoryTokens)

	// Combine into final context payload
	payload := map[string]interface{}{
		"system_prompt":     systemPrompt,
		"compacted_history": compactedHistory,
		"vector_facts":      vectorFacts,
		"knowledge_context": knowledgeCtx,
	}

	return payload, nil
}

// StoreSessionState writes a key-value value to Redis working memory.
func (cm *ContextManager) StoreSessionState(sessionID string, key string, value interface{}) error {
	args := map[string]interface{}{
		"session_id": sessionID,
		"key":        key,
		"value":      value,
	}
	_, err := cm.mcpRouter.Dispatch("store_working_memory", args)
	return err
}

// GetSessionState retrieves a key-value value from Redis working memory.
func (cm *ContextManager) GetSessionState(sessionID string, key string) (interface{}, error) {
	args := map[string]interface{}{
		"session_id": sessionID,
		"key":        key,
	}
	res, err := cm.mcpRouter.Dispatch("get_working_memory", args)
	if err != nil {
		return nil, err
	}

	m, ok := res.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response type from working memory tool")
	}

	found, _ := m["found"].(bool)
	if !found {
		return nil, fmt.Errorf("key %s not found in session %s", key, sessionID)
	}

	return m["value"], nil
}

// StoreEpisodicMemory stores a semantic text fact in Episodic Memory.
func (cm *ContextManager) StoreEpisodicMemory(sessionID string, text string, metadata map[string]interface{}) error {
	args := map[string]interface{}{
		"session_id": sessionID,
		"text":       text,
		"metadata":   metadata,
	}
	_, err := cm.mcpRouter.Dispatch("store_episodic_memory", args)
	return err
}

// DeleteEpisodicMemory deletes vector records by IDs from episodic vector memory.
func (cm *ContextManager) DeleteEpisodicMemory(factIDs []string) error {
	args := map[string]interface{}{
		"fact_ids": factIDs,
	}
	_, err := cm.mcpRouter.Dispatch("delete_episodic_memory", args)
	return err
}

// SearchEpisodicMemory searches semantic facts in vector memory and parses raw results.
func (cm *ContextManager) SearchEpisodicMemory(query string, limit int) ([]CandidateMemory, error) {
	args := map[string]interface{}{
		"query": query,
		"limit": limit,
	}
	res, err := cm.mcpRouter.Dispatch("search_episodic_memory", args)
	if err != nil {
		return nil, err
	}

	m, ok := res.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response type from search_episodic_memory tool")
	}

	resultsVal, ok := m["results"]
	if !ok {
		return nil, fmt.Errorf("search response does not contain results field")
	}

	// Re-marshal results to map into struct array cleanly
	bytes, err := json.Marshal(resultsVal)
	if err != nil {
		return nil, err
	}

	type RawResult struct {
		FactID   string                 `json:"fact_id"`
		Text     string                 `json:"text"`
		Metadata map[string]interface{} `json:"metadata"`
		Score    float64                `json:"score"`
	}

	var rawResults []RawResult
	if err := json.Unmarshal(bytes, &rawResults); err != nil {
		return nil, err
	}

	var candidates []CandidateMemory
	for _, r := range rawResults {
		var ts int64
		if r.Metadata != nil {
			if tsVal, exists := r.Metadata["timestamp"]; exists {
				switch v := tsVal.(type) {
				case float64:
					ts = int64(v)
				case int64:
					ts = v
				}
			}
		}
		if ts == 0 {
			ts = time.Now().Unix()
		}

		candidates = append(candidates, CandidateMemory{
			FactID:    r.FactID,
			Text:      r.Text,
			Metadata:  r.Metadata,
			Score:     r.Score,
			Timestamp: ts,
		})
	}

	return candidates, nil
}

// RankCandidates scores and sorts retrieved vector memories by combining relevance (similarity score) and recency (exponential decay).
// Formula: FinalScore = alpha * RelevanceScore + (1 - alpha) * e^(-lambda * AgeHours)
func (cm *ContextManager) RankCandidates(candidates []CandidateMemory, alpha float64, lambda float64) []CandidateMemory {
	now := time.Now().Unix()

	type rankedItem struct {
		candidate CandidateMemory
		score     float64
	}

	var items []rankedItem
	for _, c := range candidates {
		ageSeconds := now - c.Timestamp
		if ageSeconds < 0 {
			ageSeconds = 0
		}
		ageHours := float64(ageSeconds) / 3600.0

		// Exponential decay for recency
		recencyScore := math.Exp(-lambda * ageHours)

		// Combined scoring
		combinedScore := alpha*c.Score + (1.0-alpha)*recencyScore

		c.Score = combinedScore
		items = append(items, rankedItem{candidate: c, score: combinedScore})
	}

	// Sort candidates in descending score order
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	var result []CandidateMemory
	for _, item := range items {
		result = append(result, item.candidate)
	}
	return result
}

// Rank ranks candidates using default parameters (alpha = 0.7, lambda = 0.01).
func (cm *ContextManager) Rank(candidates []CandidateMemory) []CandidateMemory {
	return cm.RankCandidates(candidates, 0.7, 0.01)
}
