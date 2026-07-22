# PROJECT_CONTEXT.md — ContextOS Master Architecture & ADK Specification

> **Notice to AI Coding Agents (Antigravity, Cursor, Claude Code, Gemini CLI):**
> Read this document completely before writing, modifying, or refactoring code.
> Adhere strictly to the Clean Architecture principles, directory structure, module boundaries, and tech stack guidelines specified below.

---

## 1. System Vision & Overview

**ContextOS** is a high-performance runtime operating system for AI agents. Analogous to how traditional operating systems manage CPU and RAM, ContextOS manages **Context, Memory, Workflow State, Tools, Knowledge, and Multi-Agent Execution**.

### The Core Problem
LLMs are stateless. Re-sending full conversation histories and unfiltered vector search results leads to expensive token usage, high latency, context degradation, and an inability to recover state when failures occur.

### The ContextOS Solution
ContextOS sits between user applications and LLM providers. It uses a **Go / Python Microkernel**, **Model Context Protocol (MCP)** servers, and an **Agent-to-Agent (A2A)** event bus to handle memory retrieval, prompt compression, state checkpointing, and tool execution prior to calling LLMs.

---

## 2. Tech Stack Standard

| Layer | Technology | Purpose |
| :--- | :--- | :--- |
| **Core Microkernel** | **Go (Golang 1.22+) / Python 3.14+** | Concurrency engine, Workflow Runner, state machine, HTTP/gRPC gateway |
| **Tool & Data Protocol** | **MCP (Model Context Protocol)** | Isolated server plugins exposing tools and database queries |
| **Inter-Agent Protocol** | **A2A Protocol (Protobuf / gRPC)** | Agent-to-Agent communication, task negotiation, and event bus |
| **Working Memory** | **Redis** | Sub-millisecond current task state, prompt caching, and active turns |
| **Episodic & Vector Memory** | **Qdrant** | Vector similarity search for past sessions and semantic facts |
| **Knowledge Graph** | **Neo4j** | Entity-relationship graphs and dynamic semantic facts |
| **Relational Storage** | **PostgreSQL** | Workflow checkpoints, state recovery, and execution audit trails |
| **Observability Dashboard**| **HTML5 / React Telemetry Portal** | Live execution graph rendering, step latency, and token cost metrics |

---

## 3. Directory Structure

```text
ContextOS/
├── 00_PROJECT/
│   ├── PROJECT_CONTEXT.md          <-- Master Architecture & ADK Specification
│   └── ROADMAP.md                  <-- Phase Backlog & Status
├── proto/
│   └── agent_a2a.proto             <-- Protobuf definitions for A2A bus
├── kernel/                         <-- Microkernel Core
│   ├── go.mod                      <-- Go module definition
│   ├── cmd/
│   │   ├── main.go                 <-- Go Entrypoint
│   │   └── main.py                 <-- Python Microkernel Server Entrypoint
│   ├── config/
│   │   ├── config.go               <-- Go Config loader
│   │   └── config.py               <-- Python Config loader
│   └── internal/
│       ├── state/                  <-- Workflow Runner & Checkpointer
│       ├── memory/                 <-- Context Manager & Token Compactor
│       ├── mcp/                    <-- MCP Client Transport Router
│       └── a2a/                    <-- A2A Protocol Router
├── mcp_servers/                    <-- Isolated MCP Server Plugins
│   ├── memory_mcp/                 <-- Redis & Qdrant MCP Interface
│   ├── knowledge_mcp/              <-- Neo4j & PostgreSQL MCP Interface
│   └── tools_mcp/                  <-- Native Execution Plugins
├── frontend/                       <-- Telemetry Portal & Observability Dashboard
├── sdk/                            <-- Client SDKs (Python & Go)
└── docker-compose.yml              <-- Local Infrastructure Services
```
