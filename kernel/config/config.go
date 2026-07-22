package config

import (
	"os"
	"strconv"
)

type Config struct {
	Host                 string
	Port                 int
	RedisURL             string
	QdrantURL            string
	Neo4jURI             string
	PostgresDSN          string
	MaxContextTokens     int
	TargetSummaryTokens  int
	SlidingWindowTurns   int
	CompactionRatio      float64
}

func LoadConfig() *Config {
	port, _ := strconv.Atoi(getEnv("CONTEXTOS_PORT", "8080"))
	maxTokens, _ := strconv.Atoi(getEnv("MAX_CONTEXT_TOKENS", "128000"))
	targetSummary, _ := strconv.Atoi(getEnv("TARGET_SUMMARY_TOKENS", "4096"))
	windowTurns, _ := strconv.Atoi(getEnv("SLIDING_WINDOW_TURNS", "10"))
	ratio, _ := strconv.ParseFloat(getEnv("PROMPT_COMPACTION_RATIO", "0.35"), 64)

	return &Config{
		Host:                getEnv("CONTEXTOS_HOST", "0.0.0.0"),
		Port:                port,
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379/0"),
		QdrantURL:           getEnv("QDRANT_URL", "http://localhost:6333"),
		Neo4jURI:            getEnv("NEO4J_URI", "bolt://localhost:7687"),
		PostgresDSN:         getEnv("POSTGRES_DSN", "postgresql://contextos:secret@localhost:5432/contextos_db"),
		MaxContextTokens:    maxTokens,
		TargetSummaryTokens: targetSummary,
		SlidingWindowTurns:  windowTurns,
		CompactionRatio:     ratio,
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
