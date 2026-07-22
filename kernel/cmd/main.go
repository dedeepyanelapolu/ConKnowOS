package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"contextos/kernel/internal/api"
	"contextos/kernel/internal/state"

	_ "github.com/lib/pq"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}

	var checkpointer state.Checkpointer

	if dsn != "" {
		log.Printf("Connecting to PostgreSQL at %s...", dsn)
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			log.Fatalf("Failed to open database connection: %v", err)
		}
		defer db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			log.Printf("Warning: Database ping failed (%v). Falling back to InMemoryCheckpointer.", err)
			checkpointer = state.NewInMemoryCheckpointer()
		} else {
			pgCP := state.NewPostgresCheckpointer(db)
			if err := pgCP.InitSchema(ctx); err != nil {
				log.Fatalf("Failed to initialize PostgreSQL schema: %v", err)
			}
			checkpointer = pgCP
			log.Println("Successfully connected to PostgreSQL and initialized schema.")
		}
	} else {
		log.Println("No DATABASE_URL or POSTGRES_DSN provided. Using InMemoryCheckpointer for local execution.")
		checkpointer = state.NewInMemoryCheckpointer()
	}

	handler := api.NewHandler(checkpointer)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("ContextOS Kernel API Gateway starting on %s...", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
