# Engineering Rules & Operational Guidelines

This document governs the engineering standards, operational discipline, and development workflow for the **AI System Design Intelligence Layer (`sdskill`)**.

---

## 1. Workflow & Task Lifecycle Rules

### Rule 1.1: Always Read Context & Tasks Before Implementing
Before implementing any task or writing code, always consult:
- `context.md`: The overarching system design, 7-stage workflow, schema boundaries, and architectural philosophy.
- `tasks.md`: The active task definition, requirements, acceptance criteria, and dependencies.

### Rule 1.2: One Task at a Time
- Implement tasks in distinct, granular chunks as defined in `tasks.md`.
- Avoid mixing changes across unrelated tasks or jumping ahead before foundational layers are verified.

### Rule 1.3: Mandatory Verification & Testing
- Every implemented component must be verified with automated unit, integration, or protocol tests before completion.
- Tests must be executed using the local Go toolchain (`go test -v ./...` or package-specific test runs).
- A task cannot be considered complete if any relevant test fails, is skipped, or is untested.

### Rule 1.4: Update Completed Tasks Log
- Immediately after successfully verifying a task, append an entry to `completedTask.md` documenting:
  - Task ID and title.
  - Brief description of what was implemented.
  - Affected files and components.
  - Verification command and test outcome.
  - Timestamp of completion.

---

## 2. Architecture & Code Quality Rules

### Rule 2.1: Clean Architecture & Separation of Concerns
- Maintain clean boundaries between packages:
  - `pkg/model`: Pure domain entities and contracts without third-party protocol dependencies.
  - `pkg/normalizer`: Ingestion and standardization of raw requirements into domain models.
  - `pkg/classifier`: Workload taxonomy evaluation behind a decoupled `Classifier` interface.
  - `pkg/knowledge`: In-memory architectural patterns, anti-patterns, and trade-off matrices.
  - `pkg/evaluator`: Deterministic ranking and decision rules.
  - `pkg/contract`: Architecture contract assembly and schema generation.
  - `pkg/validator`: Post-implementation structural and architectural compliance checks.
  - `pkg/engine`: Orchestrator tying the pipeline stages together.
  - `pkg/mcp` & `pkg/server`: Protocol adapters (MCP stdio/SSE and HTTP REST).
- Business logic must never depend directly on MCP or HTTP protocol types.

### Rule 2.2: Idiomatic Go Standards
- **Explicit Interfaces**: Declare interfaces where they are consumed, not where they are defined.
- **Context-Aware Cancellation**: Propagate `context.Context` through all IO and pipeline operations.
- **Structured Error Handling**: Use `fmt.Errorf("...: %w", err)` for proper error wrapping. Never silently discard errors or return empty results on failure.
- **Bounded Resource Usage**: Enforce timeouts on external calls and restrict concurrent operations where appropriate.
- **No Global State**: Prefer dependency injection via struct constructors (`New(...)`).

### Rule 2.3: Clean Code & Developer-Friendly Comments
- All packages, exported structs, interfaces, and methods must have descriptive doc comments.
- Code should be written in clean, readable Go with self-explanatory variable names.
- Provide accessible, easy-to-understand inline comments explaining **why** specific architectural decisions, formulas, or heuristics were chosen so real software engineers can immediately understand and extend the codebase.

---

## 3. Integration & Honesty Rules

### Rule 3.1: No Fabricated External Services (Honesty Rule)
- External integrations (such as LAYA inference or third-party APIs) must be backed by explicit interfaces.
- If an external runtime is not running or unavailable, gracefully fall back to deterministic heuristic implementations (`RuleBasedClassifier`) with clear metadata flags.
- Never fake network responses or claim an external ML model succeeded when running a local fallback.

### Rule 3.2: Zero UI Principle
- This project is exclusively an architectural oracle and decision engine for AI coding agents.
- Do not build graphical dashboards, web frontends, React/HTML interfaces, or interactive chats.
- All interfaces must remain strictly machine-readable (MCP tools, JSON-RPC, or REST JSON).
