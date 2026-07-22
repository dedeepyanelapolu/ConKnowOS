package memory

import (
	"math"
	"strings"
	"sync"
	"time"
)

type Message struct {
	Timestamp int64  `json:"timestamp"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

type TokenStats struct {
	RawTokens       int     `json:"raw_tokens"`
	CompactedTokens int     `json:"compacted_tokens"`
	SavingsPercent  float64 `json:"savings_percent"`
	LatencyMs       float64 `json:"latency_ms"`
}

type CompactedResult struct {
	Summary        string    `json:"summary"`
	RecentMessages []Message `json:"recent_messages"`
	Stats          TokenStats `json:"stats"`
}

type ContextManager struct {
	mu       sync.RWMutex
	sessions map[string][]Message
}

func NewContextManager() *ContextManager {
	return &ContextManager{
		sessions: make(map[string][]Message),
	}
}

func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return int(math.Ceil(float64(len(text)) / 3.8))
}

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
