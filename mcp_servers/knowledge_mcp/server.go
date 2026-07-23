package knowledge_mcp

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Edge represents an entity-relationship link in the Knowledge Graph fallback.
type Edge struct {
	Source     string                 `json:"source"`
	Relation   string                 `json:"relation"`
	Target     string                 `json:"target"`
	Properties map[string]interface{} `json:"properties"`
}

// KnowledgeServer handles Neo4j and PostgreSQL database connectivity and tool execution.
type KnowledgeServer struct {
	neo4jDriver  neo4j.DriverWithContext
	neo4jEnabled bool
	postgresDB   *sql.DB
	pgEnabled    bool

	// Thread-safe fallback store for local development and offline unit tests
	mu             sync.RWMutex
	nodes          map[string]map[string]interface{}
	edges          []Edge
	relationalData map[string]map[string]interface{}
}

// NewKnowledgeServer creates a new KnowledgeServer instance.
func NewKnowledgeServer(neo4jURI, neo4jUser, neo4jPassword, postgresDSN string) *KnowledgeServer {
	var neoDriver neo4j.DriverWithContext
	var neoEnabled bool
	var pgDB *sql.DB
	var pgEnabled bool

	// Try to initialize Neo4j driver
	if neo4jURI != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		driver, err := neo4j.NewDriverWithContext(neo4jURI, neo4j.BasicAuth(neo4jUser, neo4jPassword, ""))
		if err == nil {
			if err := driver.VerifyConnectivity(ctx); err == nil {
				neoDriver = driver
				neoEnabled = true
			} else {
				fmt.Printf("[KnowledgeServer] Neo4j ping failed: %v, enabling fallback\n", err)
			}
		}
	}

	// Try to initialize PostgreSQL connection
	if postgresDSN != "" {
		db, err := sql.Open("postgres", postgresDSN)
		if err == nil {
			db.SetConnMaxLifetime(2 * time.Minute)
			if err := db.Ping(); err == nil {
				pgDB = db
				pgEnabled = true
			} else {
				fmt.Printf("[KnowledgeServer] Postgres ping failed: %v, enabling fallback\n", err)
			}
		}
	}

	s := &KnowledgeServer{
		neo4jDriver:  neoDriver,
		neo4jEnabled: neoEnabled,
		postgresDB:   pgDB,
		pgEnabled:    pgEnabled,

		// Seed local fallback stores with architectural context
		nodes: map[string]map[string]interface{}{
			"ContextOS":   {"type": "System", "description": "AI Agent Runtime OS"},
			"Microkernel": {"type": "CoreModule", "description": "Go/Python state concurrency engine"},
			"MCP":         {"type": "Protocol", "description": "Model Context Protocol"},
			"A2A":         {"type": "Protocol", "description": "Agent to Agent Communication Bus"},
		},
		edges: []Edge{
			{Source: "ContextOS", Relation: "HAS_CORE", Target: "Microkernel", Properties: make(map[string]interface{})},
			{Source: "ContextOS", Relation: "USES_PROTOCOL", Target: "MCP", Properties: make(map[string]interface{})},
			{Source: "ContextOS", Relation: "USES_PROTOCOL", Target: "A2A", Properties: make(map[string]interface{})},
		},
		relationalData: map[string]map[string]interface{}{
			"ContextOS":   {"status": "stable", "version": "1.0.0", "owner": "engineering"},
			"Microkernel": {"performance": "ultra-fast", "concurrency": "goroutines"},
			"MCP":         {"version": "0.1.0", "transport": "stdio/sse"},
			"A2A":         {"protocol": "gRPC", "serialization": "protobuf"},
		},
	}

	return s
}

// Close releases any database connections.
func (s *KnowledgeServer) Close() {
	if s.neo4jEnabled && s.neo4jDriver != nil {
		s.neo4jDriver.Close(context.Background())
	}
	if s.pgEnabled && s.postgresDB != nil {
		s.postgresDB.Close()
	}
}

// QueryGraph executes a Cypher query on Neo4j or uses the in-memory fallback.
func (s *KnowledgeServer) QueryGraph(cypherQuery string, params map[string]interface{}) ([]map[string]interface{}, error) {
	if s.neo4jEnabled {
		ctx := context.Background()
		session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		result, err := session.Run(ctx, cypherQuery, params)
		if err != nil {
			return nil, err
		}

		var records []map[string]interface{}
		for result.Next(ctx) {
			records = append(records, result.Record().AsMap())
		}
		return records, nil
	}

	// Fallback in-memory matching logic
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse simple query for matches (e.g. finding neighbors of an entity or filtering edges)
	var records []map[string]interface{}

	// Check if query wants to match specific entity neighbors
	// Very simple Cypher simulator searching for connections
	entityFilter := ""
	for k, v := range params {
		if k == "entity" || k == "name" {
			if str, ok := v.(string); ok {
				entityFilter = str
			}
		}
	}

	// If query has entity keyword in it or params
	if entityFilter != "" {
		for _, edge := range s.edges {
			if edge.Source == entityFilter || edge.Target == entityFilter {
				records = append(records, map[string]interface{}{
					"source":   edge.Source,
					"relation": edge.Relation,
					"target":   edge.Target,
				})
			}
		}
	} else {
		// Return all relationships in fallback store
		for _, edge := range s.edges {
			records = append(records, map[string]interface{}{
				"source":   edge.Source,
				"relation": edge.Relation,
				"target":   edge.Target,
			})
		}
	}

	return records, nil
}

// AddGraphRelation inserts a relation into the graph database.
func (s *KnowledgeServer) AddGraphRelation(sourceEntity string, relation string, targetEntity string, properties map[string]interface{}) error {
	if s.neo4jEnabled {
		ctx := context.Background()
		session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		defer session.Close(ctx)

		cypher := fmt.Sprintf(
			"MERGE (s:Entity {name: $source}) MERGE (t:Entity {name: $target}) MERGE (s)-[r:%s]->(t) SET r += $props RETURN r",
			relation,
		)
		params := map[string]interface{}{
			"source": sourceEntity,
			"target": targetEntity,
			"props":  properties,
		}

		_, err := session.Run(ctx, cypher, params)
		return err
	}

	// Fallback local store
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[sourceEntity]; !exists {
		s.nodes[sourceEntity] = map[string]interface{}{"type": "Entity", "description": "Custom source entity"}
	}
	if _, exists := s.nodes[targetEntity]; !exists {
		s.nodes[targetEntity] = map[string]interface{}{"type": "Entity", "description": "Custom target entity"}
	}

	s.edges = append(s.edges, Edge{
		Source:     sourceEntity,
		Relation:   relation,
		Target:     targetEntity,
		Properties: properties,
	})

	return nil
}

// QueryRelational runs standard SQL queries on PostgreSQL or uses the relational fallback store.
func (s *KnowledgeServer) QueryRelational(sqlQuery string, args []interface{}) ([]map[string]interface{}, error) {
	if s.pgEnabled {
		rows, err := s.postgresDB.Query(sqlQuery, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}

		var results []map[string]interface{}
		for rows.Next() {
			columns := make([]interface{}, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns {
				columnPointers[i] = &columns[i]
			}

			if err := rows.Scan(columnPointers...); err != nil {
				return nil, err
			}

			m := make(map[string]interface{})
			for i, colName := range cols {
				val := columnPointers[i].(*interface{})
				m[colName] = *val
			}
			results = append(results, m)
		}
		return results, nil
	}

	// Fallback in-memory matching logic
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []map[string]interface{}

	// Simulates: SELECT * FROM entity_attributes WHERE entity_key = $1
	entityKey := ""
	for _, arg := range args {
		if str, ok := arg.(string); ok {
			entityKey = str
			break
		}
	}

	if entityKey != "" {
		if attrs, exists := s.relationalData[entityKey]; exists {
			row := make(map[string]interface{})
			row["entity_key"] = entityKey
			for k, v := range attrs {
				row[k] = v
			}
			results = append(results, row)
		}
	} else {
		// Return all records in fallback store
		for k, attrs := range s.relationalData {
			row := make(map[string]interface{})
			row["entity_key"] = k
			for attrK, attrV := range attrs {
				row[attrK] = attrV
			}
			results = append(results, row)
		}
	}

	return results, nil
}

// Custom lowercase helpers matching exact request parameters
func (s *KnowledgeServer) query_graph(cypher_query string, params map[string]interface{}) ([]map[string]interface{}, error) {
	return s.QueryGraph(cypher_query, params)
}

func (s *KnowledgeServer) add_graph_relation(source_entity string, relation string, target_entity string, properties map[string]interface{}) error {
	return s.AddGraphRelation(source_entity, relation, target_entity, properties)
}

func (s *KnowledgeServer) query_relational(sql_query string, args []interface{}) ([]map[string]interface{}, error) {
	return s.QueryRelational(sql_query, args)
}

// GetEdgesCount returns fallback edge size for unit tests.
func (s *KnowledgeServer) GetEdgesCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.edges)
}
