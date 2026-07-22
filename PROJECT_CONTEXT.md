# ContextOS Architecture & Project Context

## Overview
ContextOS is an advanced agentic operating system designed for managing context-aware task workflows, persistent state transitions, and autonomous execution graphs.

## Architecture & System Design
ContextOS follows **Clean Architecture** principles in Go, maintaining clear boundaries between domain logic, state management, and external delivery mechanisms (HTTP API gateway).

### Core Components
1. **Kernel Engine & Workflow Runner (`kernel/internal/state/runner.go`)**
   - Implements a state machine for task execution lifecycle: `Idle` -> `Planning` -> `Executing` -> `Review` -> `Completed` / `Failed`.
   - Validates all state transitions against allowed state graphs.
   - Triggers persistent state checkpointing on every state mutation.

2. **State Checkpointing & Persistence (`kernel/internal/state/checkpoint.go`)**
   - PostgreSQL backed persistence store for workflow session state.
   - Manages schema migrations (`workflow_checkpoints` table with JSONB payloads).
   - Provides methods for saving (`SaveCheckpoint`) and restoring (`RestoreCheckpoint`) session state.

3. **HTTP API Gateway (`kernel/cmd/main.go` & `kernel/internal/api/`)**
   - REST API endpoints for starting execution (`POST /api/v1/agent/execute`) and querying state (`GET /api/v1/agent/state/:session_id`).

4. **Testing Suite (`kernel/internal/state/runner_test.go`)**
   - Unit tests covering state machine transitions, invalid transition error handling, and JSON/Payload serialization/deserialization.
