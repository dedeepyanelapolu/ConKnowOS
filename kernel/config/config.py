import os

class Settings:
    """ContextOS Microkernel Settings & Environment Configurations"""
    HOST: str = os.getenv("CONTEXTOS_HOST", "0.0.0.0")
    PORT: int = int(os.getenv("CONTEXTOS_PORT", "8080"))
    ENV: str = os.getenv("CONTEXTOS_ENV", "development")
    
    # Storage URLs & Connections
    REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")
    QDRANT_URL: str = os.getenv("QDRANT_URL", "http://localhost:6333")
    NEO4J_URI: str = os.getenv("NEO4J_URI", "bolt://localhost:7687")
    NEO4J_USER: str = os.getenv("NEO4J_USER", "neo4j")
    NEO4J_PASSWORD: str = os.getenv("NEO4J_PASSWORD", "password123")
    POSTGRES_DSN: str = os.getenv("POSTGRES_DSN", "postgresql://contextos:secret@localhost:5432/contextos_db")
    
    # Context & Token Compression Defaults
    MAX_CONTEXT_TOKENS: int = int(os.getenv("MAX_CONTEXT_TOKENS", "128000"))
    TARGET_SUMMARY_TOKENS: int = int(os.getenv("TARGET_SUMMARY_TOKENS", "4096"))
    SLIDING_WINDOW_TURNS: int = int(os.getenv("SLIDING_WINDOW_TURNS", "10"))
    PROMPT_COMPACTION_RATIO: float = float(os.getenv("PROMPT_COMPACTION_RATIO", "0.35"))

settings = Settings()
