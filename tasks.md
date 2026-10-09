# Implementation Tasks Breakdown

This document breaks down the **AI System Design Intelligence Layer (`sdskill`)** into atomic, self-contained implementation tasks. Each task is designed to be completed, verified, and tested in a single session before updating `completedTask.md`.

---

## Phase 0: Project Setup & Baseline Tooling

### Task 0.1: Initialize Go Module & Project Structure
- **Objective**: Establish the Go module and base directory tree following idiomatic Go standards.
- **Target Files**:
  - `go.mod`
  - Directory skeleton (`cmd/`, `pkg/model/`, `pkg/normalizer/`, `pkg/classifier/`, `pkg/knowledge/`, `pkg/evaluator/`, `pkg/contract/`, `pkg/validator/`, `pkg/engine/`, `pkg/mcp/`, `pkg/server/`, `tests/`)
- **Acceptance Criteria**:
  - `go.mod` initialized with module path `github.com/sdskill/archintel`.
  - Directory layout created.
  - Project builds and tests pass cleanly.
- **Verification**: Run `go test ./...` and verify clean module status.

---

## Phase 1: Domain Models & Contract Schemas (`pkg/model`)

### Task 1.1: Requirement Domain Models
- **Objective**: Define pure Go structures representing raw, normalized, and inferred requirements.
- **Target Files**:
  - `pkg/model/requirements.go`
  - `pkg/model/requirements_test.go`
- **Acceptance Criteria**:
  - Models for `RawRequirement`, `NormalizedRequirements`, `TrafficProfile`, `DataProfile`, `Constraints`.
  - Support for explicit vs. inferred properties.
  - Serialization tags (`json`) and unit tests validating struct behaviors and defaults.
- **Verification**: `go test -v ./pkg/model -run TestRequirements`

### Task 1.2: Classification & Workload Models
- **Objective**: Define structures and enums for multi-dimensional workload classification.
- **Target Files**:
  - `pkg/model/classification.go`
  - `pkg/model/classification_test.go`
- **Acceptance Criteria**:
  - Multi-label traits defined: `TransactionHeavy`, `ReadHeavy`, `WriteHeavy`, `LatencySensitive`, `BackgroundProcessingIntensive`, `ConsistencySensitive`, `ResourceConstrained`.
  - Structures for `WorkloadTrait`, `ClassificationScore`, and `ClassificationResult`.
  - Helper methods for querying active traits and confidence thresholds.
- **Verification**: `go test -v ./pkg/model -run TestClassification`

### Task 1.3: Architecture Contract Domain Model
- **Objective**: Define the complete, machine-readable Architecture Contract model that will be provided to AI coding agents.
- **Target Files**:
  - `pkg/model/contract.go`
  - `pkg/model/contract_test.go`
- **Acceptance Criteria**:
  - Structures for `ArchitectureContract`, `SystemTopology`, `ComponentManifest`, `DataRules`, `QueueRules`, `ImplementationConstraints`, `ValidationCriteria`.
  - JSON schema validation helper to guarantee contract validity.
- **Verification**: `go test -v ./pkg/model -run TestContract`

---

## Phase 2: Normalization & Ambiguity Detection (`pkg/normalizer`)

### Task 2.1: Requirement Normalizer
- **Objective**: Parse raw natural language requirements and structured parameters into `NormalizedRequirements`.
- **Target Files**:
  - `pkg/normalizer/normalizer.go`
  - `pkg/normalizer/normalizer_test.go`
- **Acceptance Criteria**:
  - Unit normalization (converting DAU/MAU to RPS, calculating peak loads).
  - Extraction of domain hints (e-commerce, fintech, telemetry, content).
  - Unit tests covering various real-world input styles.
- **Verification**: `go test -v ./pkg/normalizer -run TestNormalizer`

### Task 2.2: Ambiguity & Material Gap Detector
- **Objective**: Identify critical architectural unknowns (e.g. unspecified consistency for financial flows, missing traffic expectations) and provide sensible defaults.
- **Target Files**:
  - `pkg/normalizer/gaps.go`
  - `pkg/normalizer/gaps_test.go`
- **Acceptance Criteria**:
  - Emits non-blocking clarifying questions with sensible assumptions.
  - Prevents pipeline stalling while explicitly flagging assumptions in the contract.
- **Verification**: `go test -v ./pkg/normalizer -run TestGaps`

---

## Phase 3: Workload Classification Engine (`pkg/classifier`)

### Task 3.1: Rule-Based Heuristic Classifier
- **Objective**: Implement a deterministic classifier evaluating workload dimensions based on normalized requirements.
- **Target Files**:
  - `pkg/classifier/classifier.go` (Interface definition)
  - `pkg/classifier/heuristic.go`
  - `pkg/classifier/heuristic_test.go`
- **Acceptance Criteria**:
  - Decoupled `Classifier` interface.
  - Deterministic evaluation producing calibrated scores for each trait.
  - Multi-label classification (e.g. simultaneously `TransactionHeavy` and `WriteHeavy`).
- **Verification**: `go test -v ./pkg/classifier -run TestHeuristicClassifier`

### Task 3.2: LAYA Model Classifier Adapter & Fallback
- **Objective**: Implement client adapter for the LAYA non-autoregressive decision model with graceful fallback to heuristic engine.
- **Target Files**:
  - `pkg/classifier/laya_adapter.go`
  - `pkg/classifier/laya_adapter_test.go`
- **Acceptance Criteria**:
  - HTTP client communicating with LAYA inference runtime (`/v1/classify`).
  - Transparent fallback to `HeuristicClassifier` if endpoint is unreachable or disabled.
  - Mock HTTP server tests verifying timeout handling, malformed responses, and fallback metadata flags.
- **Verification**: `go test -v ./pkg/classifier -run TestLayaAdapter`

---

## Phase 4: Architectural Knowledge Catalog (`pkg/knowledge`)

### Task 4.1: Curated Architectural Pattern Repository
- **Objective**: Implement in-memory, structured repository of proven architectural patterns.
- **Target Files**:
  - `pkg/knowledge/catalog.go`
  - `pkg/knowledge/patterns.go`
  - `pkg/knowledge/patterns_test.go`
- **Acceptance Criteria**:
  - Embedded patterns: Modular Monolith, Database-Backed Job Queue (PostgreSQL `SKIP LOCKED`), Transactional Outbox, Cache-Aside, Optimistic vs. Pessimistic Concurrency.
  - Structured metadata: prerequisites, trade-offs, failure modes, scalability limits.
- **Verification**: `go test -v ./pkg/knowledge -run TestPatternCatalog`

### Task 4.2: Anti-Pattern & Risk Catalog
- **Objective**: Encode common AI coding agent anti-patterns with detection rules and remediation guidance.
- **Target Files**:
  - `pkg/knowledge/antipatterns.go`
  - `pkg/knowledge/antipatterns_test.go`
- **Acceptance Criteria**:
  - Catalog of anti-patterns (e.g., Unbounded goroutines, Distributed 2PC for simple services, Synchronous third-party payment calls in DB transactions).
  - Unit tests verifying anti-pattern lookup and risk scoring.
- **Verification**: `go test -v ./pkg/knowledge -run TestAntiPatterns`

---

## Phase 5: Trade-Off Evaluation & Decision Engine (`pkg/evaluator`)

### Task 5.1: Multi-Criteria Candidate Evaluator
- **Objective**: Implement deterministic ranking engine comparing architectural options across consistency, cost, latency, complexity, and operational overhead.
- **Target Files**:
  - `pkg/evaluator/evaluator.go`
  - `pkg/evaluator/tradeoffs.go`
  - `pkg/evaluator/evaluator_test.go`
- **Acceptance Criteria**:
  - Multi-criteria scoring matrix weighting requirements against pattern trade-offs.
  - Selection of the simplest sufficient topology (e.g. Modular Monolith before microservices).
- **Verification**: `go test -v ./pkg/evaluator -run TestEvaluator`

### Task 5.2: Rejection Rationale Generator
- **Objective**: Automatically generate explicit justifications explaining why rejected alternatives were discarded.
- **Target Files**:
  - `pkg/evaluator/rejections.go`
  - `pkg/evaluator/rejections_test.go`
- **Acceptance Criteria**:
  - Clear explanations for why higher-complexity architectures (e.g., Kafka, Microservices) were not chosen when unneeded.
- **Verification**: `go test -v ./pkg/evaluator -run TestRejections`

---

## Phase 6: Architecture Contract Generation (`pkg/contract`)

### Task 6.1: Contract Generator & Serializer
- **Objective**: Assemble evaluated decisions, constraints, queue configurations, and data rules into an immutable, versioned Architecture Contract.
- **Target Files**:
  - `pkg/contract/generator.go`
  - `pkg/contract/generator_test.go`
- **Acceptance Criteria**:
  - Produces valid `ArchitectureContract` JSON.
  - Includes explicit "DOs" and "DON'Ts" for AI coding agents.
  - Full unit test coverage verifying serialization and schema compliance.
- **Verification**: `go test -v ./pkg/contract -run TestGenerator`

---

## Phase 7: Implementation & Compliance Validator (`pkg/validator`)

### Task 7.1: Architecture Violation & Compliance Validator
- **Objective**: Implement engine to validate proposed code structure, packages, and design choices against the contract.
- **Target Files**:
  - `pkg/validator/validator.go`
  - `pkg/validator/rules.go`
  - `pkg/validator/validator_test.go`
- **Acceptance Criteria**:
  - Detects violations such as business logic in HTTP handlers, unbuffered worker loops, or missing idempotency keys on payment routes.
  - Emits structured violation reports with severity (ERROR, WARNING) and remediation advice.
- **Verification**: `go test -v ./pkg/validator -run TestValidator`

---

## Phase 8: Unified Pipeline Engine Orchestration (`pkg/engine`)

### Task 8.1: Pipeline Engine Orchestrator
- **Objective**: Wire all 7 stages together into a cohesive, context-aware engine.
- **Target Files**:
  - `pkg/engine/engine.go`
  - `pkg/engine/engine_test.go`
- **Acceptance Criteria**:
  - Public API methods: `AnalyzeRequirements`, `ClassifyWorkload`, `RecommendArchitecture`, `EvaluateTradeoffs`, `ValidateArchitecture`.
  - Comprehensive unit test validating end-to-end execution through the engine struct.
- **Verification**: `go test -v ./pkg/engine -run TestEngine`

---

## Phase 9: Agent Protocol Adapters (MCP & HTTP) (`pkg/mcp`, `pkg/server`)

### Task 9.1: Model Context Protocol (MCP) Server Adapter
- **Objective**: Expose the engine as an MCP tool server over JSON-RPC 2.0 stdio transport.
- **Target Files**:
  - `pkg/mcp/server.go`
  - `pkg/mcp/tools.go`
  - `pkg/mcp/server_test.go`
- **Acceptance Criteria**:
  - Registers MCP tools: `analyze_requirements`, `classify_workload`, `recommend_architecture`, `evaluate_tradeoffs`, `validate_architecture`.
  - Fully compliant with MCP JSON-RPC 2.0 specifications.
  - Protocol-level tests verifying tool discovery and execution.
- **Verification**: `go test -v ./pkg/mcp -run TestMCPServer`

### Task 9.2: Lightweight HTTP REST Server
- **Objective**: Provide HTTP interface for health checks, debugging, and external test execution.
- **Target Files**:
  - `pkg/server/http.go`
  - `pkg/server/handlers.go`
  - `pkg/server/http_test.go`
- **Acceptance Criteria**:
  - Endpoints: `GET /health`, `POST /api/v1/recommend`, `POST /api/v1/validate`.
  - HTTP unit tests with `httptest.NewServer`.
- **Verification**: `go test -v ./pkg/server -run TestHTTPServer`

---

## Phase 10: CLI Entrypoints & E2E Benchmark Suite (`cmd/`, `tests/`)

### Task 10.1: Main Executables
- **Objective**: Build production-ready CLI entry points.
- **Target Files**:
  - `cmd/archintel/main.go` (HTTP + CLI mode)
  - `cmd/archintel-mcp/main.go` (Dedicated MCP stdio server)
- **Acceptance Criteria**:
  - Graceful shutdown on SIGINT/SIGTERM.
  - Configurable via flags and environment variables.
  - Compiles cleanly into binaries.
- **Verification**: `go build ./cmd/archintel` and `go build ./cmd/archintel-mcp`

### Task 10.2: End-to-End Benchmark & Evaluation Suite
- **Objective**: Run automated architectural evaluation across representative project types.
- **Target Files**:
  - `tests/e2e/e2e_test.go`
  - `tests/fixtures/ecommerce_checkout.json`
  - `tests/fixtures/url_shortener.json`
  - `tests/fixtures/notification_dispatcher.json`
  - `tests/fixtures/crud_admin.json`
- **Acceptance Criteria**:
  - E2E tests assert that:
    - E-commerce recommendations require idempotency, transactional outbox, and row-level locking.
    - URL shortener recommends read-cache-aside and single-table indexing, rejecting distributed message queues.
    - Notification service recommends bounded worker queues with dead-letter handling.
    - CRUD admin service recommends simple modular monolith without queues or microservices.
- **Verification**: `go test -v ./tests/e2e/...`
