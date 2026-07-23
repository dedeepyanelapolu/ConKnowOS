package memory

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Message represents a conversation turn.
type Message struct {
	Timestamp int64  `json:"timestamp"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

// TokenStats represents the compression analytics.
type TokenStats struct {
	RawTokens       int     `json:"raw_tokens"`
	CompactedTokens int     `json:"compacted_tokens"`
	SavingsPercent  float64 `json:"savings_percent"`
	LatencyMs       float64 `json:"latency_ms"`
}

// CompactedResult holds the result of a legacy history compaction.
type CompactedResult struct {
	Summary        string     `json:"summary"`
	RecentMessages []Message  `json:"recent_messages"`
	Stats          TokenStats `json:"stats"`
}

// EstimateTokens calculates approximate token counts (1 token ≈ 3.8 characters).
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return int(math.Ceil(float64(len(text)) / 3.8))
}

// CompactHistory is a legacy history compaction function for backward compatibility.
func CompactHistory(messages []Message, slidingWindow int) CompactedResult {
	start := time.Now()
	totalRaw := 0
	for _, m := range messages {
		totalRaw += EstimateTokens(m.Content)
	}

	if len(messages) <= slidingWindow {
		return CompactedResult{
			Summary:        "",
			RecentMessages: messages,
			Stats: TokenStats{
				RawTokens:       totalRaw,
				CompactedTokens: totalRaw,
				SavingsPercent:  0.0,
				LatencyMs:       float64(time.Since(start).Microseconds()) / 1000.0,
			},
		}
	}

	older := messages[:len(messages)-slidingWindow]
	recent := messages[len(messages)-slidingWindow:]

	var summaryBuilder strings.Builder
	summaryBuilder.WriteString("CONTEXT SUMMARY:\n")
	for idx, m := range older {
		content := m.Content
		if len(content) > 90 {
			content = content[:90] + "..."
		}
		summaryBuilder.WriteString("[" + strings.ToUpper(m.Role) + " step " + string(rune(idx+1)) + "]: " + content + "\n")
	}

	summaryStr := summaryBuilder.String()
	summaryTokens := EstimateTokens(summaryStr)

	recentTokens := 0
	for _, m := range recent {
		recentTokens += EstimateTokens(m.Content)
	}

	compactedTokens := summaryTokens + recentTokens
	savings := 0.0
	if totalRaw > 0 {
		savings = (1.0 - float64(compactedTokens)/float64(totalRaw)) * 100.0
		if savings < 0 {
			savings = 0
		}
	}

	return CompactedResult{
		Summary:        summaryStr,
		RecentMessages: recent,
		Stats: TokenStats{
			RawTokens:       totalRaw,
			CompactedTokens: compactedTokens,
			SavingsPercent:  math.Round(savings*100) / 100,
			LatencyMs:       float64(time.Since(start).Microseconds()) / 1000.0,
		},
	}
}

// CompactContext prunes a list of messages to stay under a given token budget.
// It prioritizes keeping all system instructions and the most recent conversation context.
func CompactContext(messages []Message, maxTokens int) ([]Message, error) {
	if maxTokens <= 0 {
		return nil, fmt.Errorf("maxTokens must be greater than 0")
	}

	var systemMsgs []Message
	var otherMsgs []Message

	for _, m := range messages {
		if strings.ToLower(m.Role) == "system" {
			systemMsgs = append(systemMsgs, m)
		} else {
			otherMsgs = append(otherMsgs, m)
		}
	}

	// Calculate tokens already used by system messages
	systemTokens := 0
	for _, m := range systemMsgs {
		systemTokens += EstimateTokens(m.Content)
	}

	if systemTokens >= maxTokens {
		// System messages alone exceed or hit the budget; keep only system messages
		return systemMsgs, nil
	}

	remainingBudget := maxTokens - systemTokens
	var keptOthers []Message
	otherTokens := 0

	// Iterate backwards from the most recent non-system messages
	for i := len(otherMsgs) - 1; i >= 0; i-- {
		msgTokens := EstimateTokens(otherMsgs[i].Content)
		if otherTokens+msgTokens <= remainingBudget {
			// Prepend to keep chronological order
			keptOthers = append([]Message{otherMsgs[i]}, keptOthers...)
			otherTokens += msgTokens
		} else {
			break
		}
	}

	// If some messages were pruned, append a system note about the pruning
	prunedCount := len(otherMsgs) - len(keptOthers)
	var finalMessages []Message
	finalMessages = append(finalMessages, systemMsgs...)

	if prunedCount > 0 {
		noteContent := fmt.Sprintf("[System Note: Pruned %d older messages from history to fit context limits]", prunedCount)
		noteMsg := Message{
			Role:      "system",
			Content:   noteContent,
			Timestamp: time.Now().Unix(),
		}
		finalMessages = append(finalMessages, noteMsg)
	}

	finalMessages = append(finalMessages, keptOthers...)
	return finalMessages, nil
}
