package memory_mcp

import (
	"testing"
)

func TestMemoryServer_WorkingMemory(t *testing.T) {
	// Initialize in mock/fallback mode by using empty URLs to guarantee offline execution
	server := NewMemoryServer("redis://invalid-url-to-trigger-fallback:0", "http://invalid-url:0")

	// Store value
	sessionID := "test-session"
	key := "user_preference"
	val := map[string]interface{}{"theme": "dark", "notifications": true}

	err := server.store_working_memory(sessionID, key, val)
	if err != nil {
		t.Fatalf("failed to store working memory: %v", err)
	}

	// Retrieve value
	retrieved, err := server.get_working_memory(sessionID, key)
	if err != nil {
		t.Fatalf("failed to get working memory: %v", err)
	}

	retrievedMap, ok := retrieved.(map[string]interface{})
	if !ok {
		t.Fatalf("expected retrieved to be map, got %T", retrieved)
	}

	if retrievedMap["theme"] != "dark" {
		t.Errorf("expected theme to be dark, got %v", retrievedMap["theme"])
	}

	// Test fallback counter
	if count := server.GetKVCount(); count != 1 {
		t.Errorf("expected KV count to be 1, got %d", count)
	}
}

func TestMemoryServer_EpisodicMemory(t *testing.T) {
	server := NewMemoryServer("redis://invalid-url:0", "http://invalid-url:0")

	sessionID := "test-session"
	text1 := "ContextOS is built on clean architecture"
	text2 := "Model Context Protocol isolates resources"
	meta := map[string]interface{}{"source": "unit_test"}

	// Store facts
	if err := server.store_episodic_memory(sessionID, text1, meta); err != nil {
		t.Fatalf("failed to store episodic memory 1: %v", err)
	}
	if err := server.store_episodic_memory(sessionID, text2, meta); err != nil {
		t.Fatalf("failed to store episodic memory 2: %v", err)
	}

	if count := server.GetVectorCount(); count != 2 {
		t.Errorf("expected vector count to be 2, got %d", count)
	}

	// Search
	results, err := server.search_episodic_memory("clean architecture", 1)
	if err != nil {
		t.Fatalf("failed to search episodic memory: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// The query "clean architecture" should return text1 as highest score
	if results[0].Text != text1 {
		t.Errorf("expected best match to be '%s', got '%s'", text1, results[0].Text)
	}
}
