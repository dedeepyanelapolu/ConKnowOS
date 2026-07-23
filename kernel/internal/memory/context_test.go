package memory

import (
	"testing"
	"time"

	"contextos/kernel/internal/mcp"
)

func TestRankCandidates(t *testing.T) {
	cm := NewContextManager(nil, nil)

	now := time.Now().Unix()

	// Candidate 1: relevance 0.9, age 10 hours
	c1 := CandidateMemory{
		FactID:    "1",
		Text:      "High relevance, older",
		Score:     0.9,
		Timestamp: now - 10*3600,
	}

	// Candidate 2: relevance 0.7, age 1 hour
	c2 := CandidateMemory{
		FactID:    "2",
		Text:      "Lower relevance, newer",
		Score:     0.7,
		Timestamp: now - 1*3600,
	}

	// Candidate 3: relevance 0.4, age 48 hours
	c3 := CandidateMemory{
		FactID:    "3",
		Text:      "Low relevance, old",
		Score:     0.4,
		Timestamp: now - 48*3600,
	}

	candidates := []CandidateMemory{c1, c2, c3}

	// Run ranking with alpha = 0.5, lambda = 0.1 (50% relevance, 50% recency weight)
	rankedDecay := cm.RankCandidates(candidates, 0.5, 0.1)

	if len(rankedDecay) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(rankedDecay))
	}

	// Candidate 2 combined: 0.5 * 0.7 + 0.5 * exp(-0.1) = 0.35 + 0.5 * 0.9048 = 0.8024
	// Candidate 1 combined: 0.5 * 0.9 + 0.5 * exp(-1.0) = 0.45 + 0.5 * 0.3678 = 0.6339
	// Candidate 2 (newer but lower similarity) should rank above Candidate 1 (older but higher similarity)
	if rankedDecay[0].FactID != "2" {
		t.Errorf("expected Candidate 2 (newer) to rank first under decay, got %s (score %f)", rankedDecay[0].FactID, rankedDecay[0].Score)
	}
}

func TestCompactContext(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "System Instruction 1"},
		{Role: "system", Content: "System Instruction 2"},
		{Role: "user", Content: "Hello world message"},
		{Role: "assistant", Content: "Response from assistant"},
		{Role: "user", Content: "Last query of user"},
	}

	// Token estimates:
	// "System Instruction 1" -> ceil(20/3.8) = 6
	// "System Instruction 2" -> ceil(20/3.8) = 6
	// "Hello world message" -> ceil(19/3.8) = 5
	// "Response from assistant" -> ceil(23/3.8) = 7
	// "Last query of user" -> ceil(18/3.8) = 5

	// If maxTokens is 20:
	// System tokens: 6 + 6 = 12
	// Remaining: 8 tokens
	// "Last query of user" (5 tokens) fits.
	// "Response from assistant" (7 tokens) does not fit (12 + 5 + 7 = 24 > 20).
	// We expect system messages (2), followed by a system note message, followed by "Last query of user".
	compacted, err := CompactContext(messages, 20)
	if err != nil {
		t.Fatalf("unexpected error in CompactContext: %v", err)
	}

	// Verify system messages are kept
	sysCount := 0
	noteCount := 0
	userCount := 0
	for _, m := range compacted {
		if m.Role == "system" {
			if m.Content == "System Instruction 1" || m.Content == "System Instruction 2" {
				sysCount++
			} else {
				noteCount++ // system note about pruning
			}
		} else if m.Role == "user" && m.Content == "Last query of user" {
			userCount++
		}
	}

	if sysCount != 2 {
		t.Errorf("expected 2 original system messages, got %d", sysCount)
	}
	if noteCount != 1 {
		t.Errorf("expected 1 system pruning note, got %d", noteCount)
	}
	if userCount != 1 {
		t.Errorf("expected 1 user message, got %d", userCount)
	}
}

func TestContextManager_MCPDispatch(t *testing.T) {
	router := mcp.NewRouter()
	router.RegisterServer("mcp_memory", "Memory", "in-memory")

	// Register Mock handlers in Router
	_ = router.RegisterTool(mcp.ToolInfo{
		Name:     "store_working_memory",
		ServerID: "mcp_memory",
		Handler: func(args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{"status": "stored"}, nil
		},
	})
	_ = router.RegisterTool(mcp.ToolInfo{
		Name:     "get_working_memory",
		ServerID: "mcp_memory",
		Handler: func(args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{"found": true, "value": "test-data"}, nil
		},
	})

	cm := NewContextManager(router, nil)

	err := cm.StoreSessionState("session-1", "key-1", "data")
	if err != nil {
		t.Fatalf("failed to store session state: %v", err)
	}

	val, err := cm.GetSessionState("session-1", "key-1")
	if err != nil {
		t.Fatalf("failed to get session state: %v", err)
	}

	if val != "test-data" {
		t.Errorf("expected test-data, got %v", val)
	}
}
