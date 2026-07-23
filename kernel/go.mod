module contextos/kernel

go 1.25.0

require github.com/lib/pq v1.12.3

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/contextos/knowledge_mcp v0.0.0-00010101000000-000000000000
	github.com/gorilla/websocket v1.5.3
)

require github.com/neo4j/neo4j-go-driver/v5 v5.28.4 // indirect

replace github.com/contextos/knowledge_mcp => ../mcp_servers/knowledge_mcp
