package memory_mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// MemorySearchResult represents a semantic matching result from episodic memory.
type MemorySearchResult struct {
	FactID   string                 `json:"fact_id"`
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata"`
	Score    float64                `json:"score"`
}

// MemoryServer implements Redis (Working Memory) and Qdrant (Episodic Memory) APIs.
type MemoryServer struct {
	redisClient  *redis.Client
	redisEnabled bool
	qdrantURL    string
	qdrantClient *http.Client

	// Thread-safe fallback storage in case infrastructure services are not running
	mu          sync.RWMutex
	kvStore     map[string]string
	vectorStore []MemorySearchResult
}

// NewMemoryServer creates a new MemoryServer instance.
func NewMemoryServer(redisURL string, qdrantURL string) *MemoryServer {
	opts, err := redis.ParseURL(redisURL)
	var rClient *redis.Client
	var redisEnabled bool
	if err == nil {
		rClient = redis.NewClient(opts)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := rClient.Ping(ctx).Err(); err == nil {
			redisEnabled = true
		} else {
			fmt.Printf("[MemoryServer] Redis ping failed, enabling local fallback: %v\n", err)
		}
	} else {
		fmt.Printf("[MemoryServer] Invalid Redis URL: %v, enabling local fallback\n", err)
	}

	s := &MemoryServer{
		redisClient:  rClient,
		redisEnabled: redisEnabled,
		qdrantURL:    qdrantURL,
		qdrantClient: &http.Client{Timeout: 3 * time.Second},
		kvStore:      make(map[string]string),
		vectorStore:  make([]MemorySearchResult, 0),
	}

	// Try to ensure the Qdrant collection exists in the background if enabled
	go s.ensureQdrantCollection()

	return s
}

// StoreWorkingMemory stores key-value state in working memory.
func (s *MemoryServer) StoreWorkingMemory(sessionID string, key string, value interface{}) error {
	payloadBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal working memory value: %w", err)
	}

	if s.redisEnabled {
		ctx := context.Background()
		redisKey := fmt.Sprintf("%s:%s", sessionID, key)
		if err := s.redisClient.Set(ctx, redisKey, payloadBytes, 24*time.Hour).Err(); err != nil {
			return fmt.Errorf("redis store error: %w", err)
		}
		return nil
	}

	// Fallback to in-memory KV
	s.mu.Lock()
	defer s.mu.Unlock()
	localKey := fmt.Sprintf("%s:%s", sessionID, key)
	s.kvStore[localKey] = string(payloadBytes)
	return nil
}

// GetWorkingMemory retrieves key-value state from working memory.
func (s *MemoryServer) GetWorkingMemory(sessionID string, key string) (interface{}, error) {
	var payloadBytes []byte

	if s.redisEnabled {
		ctx := context.Background()
		redisKey := fmt.Sprintf("%s:%s", sessionID, key)
		val, err := s.redisClient.Get(ctx, redisKey).Result()
		if err == redis.Nil {
			return nil, fmt.Errorf("key %s not found in session %s", key, sessionID)
		} else if err != nil {
			return nil, fmt.Errorf("redis get error: %w", err)
		}
		payloadBytes = []byte(val)
	} else {
		// Fallback to in-memory KV
		s.mu.RLock()
		localKey := fmt.Sprintf("%s:%s", sessionID, key)
		val, exists := s.kvStore[localKey]
		s.mu.RUnlock()
		if !exists {
			return nil, fmt.Errorf("key %s not found in session %s", key, sessionID)
		}
		payloadBytes = []byte(val)
	}

	var output interface{}
	if err := json.Unmarshal(payloadBytes, &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal working memory value: %w", err)
	}
	return output, nil
}

// StoreEpisodicMemory indexes a semantic fact into vector memory.
func (s *MemoryServer) StoreEpisodicMemory(sessionID string, text string, metadata map[string]interface{}) error {
	vector := getEmbedding(text)
	factID := uuid.New().String()

	payload := map[string]interface{}{
		"session_id": sessionID,
		"text":       text,
	}
	for k, v := range metadata {
		payload[k] = v
	}

	// Try using Qdrant REST API
	err := s.qdrantUpsert(factID, vector, payload)
	if err == nil {
		return nil
	}

	// Fallback to local in-memory vector store
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vectorStore = append(s.vectorStore, MemorySearchResult{
		FactID:   factID,
		Text:     text,
		Metadata: payload,
		Score:    1.0,
	})
	return nil
}

// DeleteEpisodicMemory deletes vector records by IDs from Qdrant or local fallback cache.
func (s *MemoryServer) DeleteEpisodicMemory(factIDs []string) error {
	type DeleteRequest struct {
		Points []string `json:"points"`
	}
	reqBody := DeleteRequest{Points: factIDs}
	bodyBytes, err := json.Marshal(reqBody)
	if err == nil {
		url := fmt.Sprintf("%s/collections/episodic_memory/points/delete", s.qdrantURL)
		req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, err := s.qdrantClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 400 {
					return nil
				}
			}
		}
	}

	// Local fallback store deletion
	s.mu.Lock()
	defer s.mu.Unlock()
	idMap := make(map[string]bool)
	for _, id := range factIDs {
		idMap[id] = true
	}
	var retained []MemorySearchResult
	for _, item := range s.vectorStore {
		if !idMap[item.FactID] {
			retained = append(retained, item)
		}
	}
	s.vectorStore = retained
	return nil
}

// SearchEpisodicMemory performs semantic search on vector memory.
func (s *MemoryServer) SearchEpisodicMemory(query string, limit int) ([]MemorySearchResult, error) {
	vector := getEmbedding(query)

	// Try querying Qdrant REST API
	results, err := s.qdrantSearch(vector, limit)
	if err == nil {
		return results, nil
	}

	// Fallback to local cosine similarity search
	return s.searchInMemory(vector, limit), nil
}

// ensureQdrantCollection sets up the Qdrant collection if running.
func (s *MemoryServer) ensureQdrantCollection() {
	url := fmt.Sprintf("%s/collections/episodic_memory", s.qdrantURL)
	reqBody := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     4, // 4-dimensional deterministic embeddings
			"distance": "Cosine",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.qdrantClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// qdrantUpsert pushes a point to Qdrant.
func (s *MemoryServer) qdrantUpsert(id string, vector []float32, payload map[string]interface{}) error {
	url := fmt.Sprintf("%s/collections/episodic_memory/points?wait=true", s.qdrantURL)
	reqBody := map[string]interface{}{
		"points": []map[string]interface{}{
			{
				"id":      id,
				"vector":  vector,
				"payload": payload,
			},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.qdrantClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("qdrant upsert status: %d", resp.StatusCode)
	}
	return nil
}

// qdrantSearch queries Qdrant points.
func (s *MemoryServer) qdrantSearch(vector []float32, limit int) ([]MemorySearchResult, error) {
	url := fmt.Sprintf("%s/collections/episodic_memory/points/search", s.qdrantURL)
	reqBody := map[string]interface{}{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.qdrantClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("qdrant search status: %d", resp.StatusCode)
	}

	var searchResponse struct {
		Result []struct {
			ID      string                 `json:"id"`
			Score   float64                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, err
	}

	var results []MemorySearchResult
	for _, r := range searchResponse.Result {
		text, _ := r.Payload["text"].(string)
		results = append(results, MemorySearchResult{
			FactID:   r.ID,
			Text:     text,
			Metadata: r.Payload,
			Score:    r.Score,
		})
	}
	return results, nil
}

// searchInMemory performs cosine similarity matching on our in-memory fallback cache.
func (s *MemoryServer) searchInMemory(queryVector []float32, limit int) []MemorySearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scoredResult struct {
		res   MemorySearchResult
		score float64
	}

	var scored []scoredResult
	for _, item := range s.vectorStore {
		itemVector := getEmbedding(item.Text)
		score := cosineSimilarity(queryVector, itemVector)
		item.Score = score
		scored = append(scored, scoredResult{res: item, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var results []MemorySearchResult
	for i := 0; i < len(scored) && i < limit; i++ {
		results = append(results, scored[i].res)
	}
	return results
}

// getEmbedding generates a deterministic mock vector embedding of dimension 4.
func getEmbedding(text string) []float32 {
	vector := []float32{0.1, 0.2, 0.3, 0.4}
	var hash int
	for _, char := range text {
		hash = int(char) + (hash << 5) - hash
	}
	if hash < 0 {
		hash = -hash
	}
	for i := range vector {
		vector[i] += float32(hash%100) / 500.0
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

// GetKVCount returns KV store size for tests.
func (s *MemoryServer) GetKVCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.kvStore)
}

// GetVectorCount returns Vector store size for tests.
func (s *MemoryServer) GetVectorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.vectorStore)
}

// Custom helpers matching lowercase requirements
func (s *MemoryServer) store_working_memory(session_id string, key string, value interface{}) error {
	return s.StoreWorkingMemory(session_id, key, value)
}

func (s *MemoryServer) get_working_memory(session_id string, key string) (interface{}, error) {
	return s.GetWorkingMemory(session_id, key)
}

func (s *MemoryServer) store_episodic_memory(session_id string, text string, metadata map[string]interface{}) error {
	return s.StoreEpisodicMemory(session_id, text, metadata)
}

func (s *MemoryServer) search_episodic_memory(query string, limit int) ([]MemorySearchResult, error) {
	return s.SearchEpisodicMemory(query, limit)
}
