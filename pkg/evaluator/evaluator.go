package evaluator

import (
	"fmt"

	"github.com/sdskill/archintel/pkg/knowledge"
	"github.com/sdskill/archintel/pkg/model"
)

// Evaluator compares architectural patterns against workload classifications and constraints,
// selecting the simplest viable architecture that satisfies correctness, reliability, and security.
type Evaluator struct {
	catalog *knowledge.Catalog
}

// New creates an Evaluator backed by the architectural knowledge catalog.
func New(catalog *knowledge.Catalog) *Evaluator {
	if catalog == nil {
		catalog = knowledge.NewCatalog()
	}
	return &Evaluator{
		catalog: catalog,
	}
}

// Evaluate analyzes requirements and classification results to produce an architectural decision summary.
func (e *Evaluator) Evaluate(req *model.NormalizedRequirements, class *model.ClassificationResult) *DecisionSummary {
	summary := &DecisionSummary{
		SelectedPatterns: make([]CandidateEvaluation, 0),
	}

	// 1. Evaluate Topology
	summary.RecommendedTopology, summary.TopologyRationale = e.evaluateTopology(req, class)

	// 2. Evaluate Data and Concurrency Rules
	summary.RecommendedDataRules = e.evaluateDataRules(req, class)

	// 3. Evaluate Background Queue and Worker Rules
	summary.RecommendedQueueRules = e.evaluateQueueRules(req, class, summary.RecommendedDataRules)

	// 4. Evaluate Capacity Plan via Little's Law
	summary.CapacityPlan = e.evaluateCapacityPlan(req, summary.RecommendedDataRules, summary.RecommendedQueueRules)

	// 5. Evaluate Offline Synchronization Strategy
	summary.OfflineSync = e.evaluateOfflineSync(req)

	// 6. Select Matching Patterns
	summary.SelectedPatterns = e.selectPatterns(req, class, summary)

	// 7. Evaluate Architecture Decision Traceability
	summary.DecisionTraces = e.evaluateDecisionTraces(req, summary)

	return summary
}

// evaluateTopology enforces "Simplicity by default": prefers Modular Monolith unless extreme scale justifies distribution.
func (e *Evaluator) evaluateTopology(req *model.NormalizedRequirements, class *model.ClassificationResult) (model.TopologyType, string) {
	// If IoT / Telemetry write stream dominates, use Async Pipeline.
	if req.Domain == model.DomainIoTTelemetry && req.IsWriteHeavy() {
		return model.TopologyPipeline,
			"Dedicated async worker pipeline recommended to isolate heavy time-series write ingestion from query paths."
	}

	// If extreme scale (> 5000 RPS) AND independent teams are explicitly required:
	if req.Traffic.ExpectedRPS >= 5000.0 && req.Constraints.DeploymentTarget == "MICROSERVICES" {
		return model.TopologyMicroservices,
			"Scale (> 5,000 baseline RPS) and explicit organizational constraints justify decoupled microservices."
	}

	// Domain-specific Modular Monolith Rationale
	if req.Domain == model.DomainEventManagement {
		return model.TopologyModularMonolith, fmt.Sprintf(
			"Modular Monolith selected for Event Management (%s). Provides atomic database transaction safety for delegate check-in and QR attendance, zero network hops between registration and verification modules, and operational simplicity within single cloud budget constraints.",
			req.ProjectName,
		)
	}

	// Default: Modular Monolith
	return model.TopologyModularMonolith, fmt.Sprintf(
		"Modular Monolith selected for %s. Provides ACID transaction safety, zero network hops between modules, and operational simplicity without distributed infrastructure overhead.",
		req.Domain,
	)
}

// evaluateDataRules deterministically assigns locking, isolation, and outbox rules.
func (e *Evaluator) evaluateDataRules(req *model.NormalizedRequirements, class *model.ClassificationResult) model.DataRules {
	rules := model.DataRules{
		IsolationLevel:         "READ_COMMITTED", // Safe default for PostgreSQL/MySQL
		Locking:                model.LockingNone,
		RequiresACID:           req.Data.RequiresACID,
		RequiresIdempotencyKey: false,
		RequiresOutboxPattern:  false,
		ConnectionPoolMax:      20,
	}

	// 1. Transactional Outbox requirement:
	if req.Domain == model.DomainECommerce || req.Domain == model.DomainFinTech || req.Data.RequiresACID || len(req.Invariants.AsyncOperations) > 0 {
		rules.RequiresOutboxPattern = true
	}

	// 2. Outbox and duplicate prevention mandate RequiresACID
	if rules.RequiresOutboxPattern || req.Invariants.PreventDuplicates {
		rules.RequiresACID = true
	}

	// 3. Database Uniqueness and Conflict Handling for duplicate prevention
	if req.Invariants.PreventDuplicates {
		rules.UniquenessStrategy = "DATABASE_UNIQUE_CONSTRAINT (e.g. compound unique index: UNIQUE (delegate_id, session_id))"
		rules.ConflictHandling = "ATOMIC_INSERT_ON_CONFLICT (INSERT ... ON CONFLICT (delegate_id, session_id) DO NOTHING)"
	}

	// 4. Concurrency Locking & Detailed Concurrency Control
	if req.Data.HasHighContention || (class.HasTrait(model.TraitHighContention) && !req.Invariants.PreventDuplicates) {
		rules.Locking = model.LockingPessimistic
		rules.ConcurrencyDetails = model.ConcurrencyControlDetails{
			Mechanism:                  "PESSIMISTIC_ROW_LOCKING (SELECT ... FOR UPDATE)",
			ExplicitLockingRequired:    true,
			ConcurrentConflictBehavior: "Transactions serialize on mutated row; subsequent requests wait until tx commits or times out",
			IsolationLevel:             "READ_COMMITTED",
			QueueWorkerClaimLocking:    "SELECT ... FOR UPDATE SKIP LOCKED",
			Rationale:                  "Explicit pessimistic locking is strictly required on shared inventory/balance rows to prevent race conditions and overselling under burst concurrency.",
		}
	} else if req.Invariants.PreventDuplicates {
		rules.Locking = model.LockingDatabaseManaged
		rules.ConcurrencyDetails = model.ConcurrencyControlDetails{
			Mechanism:                  "DATABASE_MANAGED_UNIQUE_INDEX (PostgreSQL UNIQUE compound constraint)",
			ExplicitLockingRequired:    false,
			ConcurrentConflictBehavior: "Atomic rejection of concurrent duplicates via INSERT ... ON CONFLICT DO NOTHING without application-level locking",
			IsolationLevel:             "READ_COMMITTED",
			QueueWorkerClaimLocking:    "SELECT ... FOR UPDATE SKIP LOCKED",
			Rationale:                  "No explicit row-level locks (e.g. SELECT FOR UPDATE) or SERIALIZABLE isolation required; database compound unique index enforces absolute duplicate prevention atomically under concurrent scans.",
		}
	} else if req.HasStrictConsistency() || class.HasTrait(model.TraitConsistencySensitive) {
		rules.Locking = model.LockingOptimistic
		rules.ConcurrencyDetails = model.ConcurrencyControlDetails{
			Mechanism:                  "OPTIMISTIC_CONCURRENCY_CONTROL (version column / updated_at check)",
			ExplicitLockingRequired:    false,
			ConcurrentConflictBehavior: "Version mismatch triggers rollback and client retry",
			IsolationLevel:             "READ_COMMITTED",
			QueueWorkerClaimLocking:    "SELECT ... FOR UPDATE SKIP LOCKED",
			Rationale:                  "Optimistic version checking prevents lost updates without holding database row locks.",
		}
	} else {
		rules.Locking = model.LockingNone
		rules.ConcurrencyDetails = model.ConcurrencyControlDetails{
			Mechanism:                  "NONE_REQUIRED (stateless reads or non-conflicting inserts)",
			ExplicitLockingRequired:    false,
			ConcurrentConflictBehavior: "Standard read-committed isolation",
			IsolationLevel:             "READ_COMMITTED",
			QueueWorkerClaimLocking:    "SELECT ... FOR UPDATE SKIP LOCKED",
			Rationale:                  "Workload does not experience write-write contention on shared entities.",
		}
	}

	// 5. Build TransactionPolicy
	atomicOps := make([]string, 0)
	if rules.RequiresOutboxPattern {
		if req.Domain == model.DomainEventManagement {
			atomicOps = append(atomicOps, "delegate_check_in_and_outbox_event", "delegate_registration_and_welcome_event")
		} else if req.Domain == model.DomainECommerce {
			atomicOps = append(atomicOps, "order_placement_and_outbox_event", "inventory_reservation_and_payment_event")
		} else {
			atomicOps = append(atomicOps, "business_state_mutation_and_outbox_event")
		}
	}

	dbInvariants := make([]string, 0)
	if req.Invariants.PreventDuplicates {
		if len(req.Invariants.UniquenessTargets) > 0 {
			dbInvariants = append(dbInvariants, req.Invariants.UniquenessTargets...)
		} else {
			dbInvariants = append(dbInvariants, "unique_entity_constraint")
		}
	}

	eventualAllowed := make([]string, 0)
	if len(req.Invariants.AsyncOperations) > 0 {
		eventualAllowed = append(eventualAllowed, req.Invariants.AsyncOperations...)
	} else {
		eventualAllowed = append(eventualAllowed, "read_dashboard_reporting", "asynchronous_notifications")
	}

	rules.TransactionPolicy = model.TransactionPolicy{
		RequiresACID:               rules.RequiresACID,
		Scope:                      "PER_OPERATION_ATOMIC",
		AtomicOperations:           atomicOps,
		DatabaseEnforcedInvariants: dbInvariants,
		EventualConsistencyAllowed: eventualAllowed,
		Rationale:                  "ACID transactions are strictly bounded to individual multi-step mutations (e.g. state write + outbox record) and unique constraint validation; global strong consistency is avoided to allow asynchronous side effects to process independently.",
	}

	// 6. Idempotency Key requirement & detailed IdempotencyPolicy
	if req.Domain == model.DomainECommerce || req.Domain == model.DomainFinTech || req.Invariants.RequiresIdempotency {
		rules.RequiresIdempotencyKey = true
	}

	applicableOps := make([]string, 0)
	if rules.RequiresIdempotencyKey {
		if req.Domain == model.DomainECommerce {
			applicableOps = append(applicableOps, "order_placement", "payment_capture", "refund_initiation")
		} else if req.Domain == model.DomainFinTech {
			applicableOps = append(applicableOps, "transfer_initiation", "payment_charge", "disbursement")
		} else {
			applicableOps = append(applicableOps, "financial_transaction", "state_mutation")
		}
	} else if req.Invariants.RequiresOfflineHandling {
		applicableOps = append(applicableOps, "offline_batch_sync_upload")
	}

	rules.IdempotencyPolicy = model.IdempotencyPolicy{
		Required:                      rules.RequiresIdempotencyKey,
		ApplicableOperations:          applicableOps,
		KeyScope:                      "HEADER_IDEMPOTENCY_KEY_SCOPED_BY_OPERATION_AND_TENANT",
		FingerprintMechanism:          "SHA-256 hash of canonical request method, path, and normalized body payload",
		StateStorageMechanism:         "Durable relational table (idempotency_keys) recording key, payload_hash, status (IN_PROGRESS/COMPLETED), original_status_code, and serialized response body",
		ConcurrentRequestHandling:     "Return HTTP 409 Conflict with 'Operation in progress' retry-after guidance or serialize execution via database row-level locking",
		PayloadMismatchBehavior:       "Return HTTP 422 Unprocessable Entity rejecting key reuse with mismatched payload fingerprint",
		ResponseReplayBehavior:        "Atomic replay of cached original HTTP status code and response body without re-executing business mutations",
		RetentionPeriod:               "24 to 48 hours rolling retention with background purge",
		EnforcementValidationCriteria: "Verification requires proof of persistent idempotency state lookup, payload fingerprint comparison, and replayed response storage; inspecting request headers alone is insufficient",
		DatabaseUniquenessAlternative: "Operations with natural domain uniqueness (such as attendance check-in on (delegate_id, session_id)) rely primarily on database UNIQUE constraints with atomic ON CONFLICT handling, avoiding unnecessary client idempotency token overhead",
	}

	// 7. Connection Pool Sizing via Little's Law (L = lambda * W)
	avgQueryLatencySec := 0.020                 // 20ms
	estimatedDBRPS := req.Traffic.PeakRPS * 2.0 // Assume ~2 DB queries per incoming HTTP request
	estimatedAvgDBConcurrency := estimatedDBRPS * avgQueryLatencySec

	if req.Traffic.PeakRPS > 1000 {
		rules.ConnectionPoolMax = 50
		rules.PoolSizingRationale = fmt.Sprintf(
			"Little's Law applied to DB query load: lambda=%0.0f queries/sec (est. 2 queries/request at peak %0.0f HTTP RPS), W=20ms avg query latency -> avg active DB concurrency L=%0.2f. Safe pool max of 50 connections provides ~1.25x headroom for burst spikes while staying well within single PostgreSQL instance max_connections (100). Sizing is a planning estimate requiring empirical load validation.",
			estimatedDBRPS, req.Traffic.PeakRPS, estimatedAvgDBConcurrency,
		)
	} else if req.Traffic.PeakRPS > 300 {
		rules.ConnectionPoolMax = 35
		rules.PoolSizingRationale = fmt.Sprintf(
			"Little's Law applied to DB query load: lambda=%0.0f queries/sec (est. 2 queries/request at peak %0.0f HTTP RPS), W=20ms avg query latency -> avg active DB concurrency L=%0.2f. Safe pool max of 35 connections provides headroom for burst spikes without database connection starvation. Sizing is a planning estimate requiring empirical load validation.",
			estimatedDBRPS, req.Traffic.PeakRPS, estimatedAvgDBConcurrency,
		)
	} else {
		rules.ConnectionPoolMax = 20
		rules.PoolSizingRationale = fmt.Sprintf(
			"Little's Law applied to DB query load: lambda=%0.0f queries/sec (est. 2 queries/request at conservative peak %0.0f HTTP RPS scenario), W=20ms avg query latency -> avg active DB concurrency L=%0.2f. Pool of 20 connections provides safe headroom for sudden traffic bursts and concurrent background worker connections (8 workers) without exhausting single-server cloud memory. Sizing represents a conservative planning estimate, not an empirical guarantee.",
			estimatedDBRPS, req.Traffic.PeakRPS, estimatedAvgDBConcurrency,
		)
	}

	return rules
}

// evaluateCapacityPlan documents connection pool and worker sizing derivations using Little's Law.
func (e *Evaluator) evaluateCapacityPlan(req *model.NormalizedRequirements, rules model.DataRules, queue model.QueueRules) model.CapacityPlan {
	avgLatencySec := 0.020 // 20ms
	planningRPS := req.Traffic.PeakRPS
	if planningRPS <= 0 {
		planningRPS = 25.0
	}
	estimatedDBRPS := planningRPS * 2.0
	avgDBConcurrency := estimatedDBRPS * avgLatencySec

	rangeStr := fmt.Sprintf("%0.1f - %0.1f HTTP RPS", req.Traffic.ExpectedRPS, req.Traffic.PeakRPS)
	if req.Traffic.ExpectedRPS == req.Traffic.PeakRPS || req.Traffic.ExpectedRPS == 0 {
		rangeStr = fmt.Sprintf("Up to %0.1f HTTP RPS", req.Traffic.PeakRPS)
	}

	return model.CapacityPlan{
		QuantityBeingEstimated:      "Concurrent Active Database Transactions and Connections",
		SelectedScenario:            "Conservative Peak Burst Scenario",
		ScenarioRationale:           fmt.Sprintf("Sized using upper bound (%0.1f HTTP RPS) of derived arrival rate window to ensure adequate connection headroom during concentrated attendee check-in spikes.", planningRPS),
		EstimatedArrivalRateRange:   rangeStr,
		PlanningArrivalRate:         planningRPS,
		LittlesLawFormula:           "L = lambda * W (where lambda = DB query rate, W = average query duration)",
		EstimatedServiceTime:        fmt.Sprintf("%0.0f ms average database query latency", avgLatencySec*1000.0),
		EstimatedAverageConcurrency: avgDBConcurrency,
		RecommendedDBPoolMax:        rules.ConnectionPoolMax,
		WorkerConcurrencyMax:        queue.MaxWorkerConcurrency,
		HeadroomRationale:           fmt.Sprintf("Pool of %d connections accommodates derived average DB concurrency of %0.2f with safe headroom for burst arrivals and %d background worker connections without starving database resources.", rules.ConnectionPoolMax, avgDBConcurrency, queue.MaxWorkerConcurrency),
		CapacityCaveats: []string{
			"Traffic volume is an estimated planning figure, not a load-tested capacity guarantee",
			"Database query count per HTTP request assumes optimal indexing and zero N+1 query patterns",
			"Background worker pool consumes database connections independently from web request handlers",
			"Capacity must be empirically validated under realistic synthetic load testing prior to production launch",
		},
	}
}

// evaluateOfflineSync defines offline synchronization, buffering, and conflict semantics.
func (e *Evaluator) evaluateOfflineSync(req *model.NormalizedRequirements) *model.OfflineSyncStrategy {
	if !req.Invariants.RequiresOfflineHandling {
		return nil
	}

	return &model.OfflineSyncStrategy{
		Policy:                         "SERVER_FIRST_WRITE_WINS_WITH_AUDIT_LOG",
		ScanEventIdentity:              "Stable client UUID v4 assigned uniquely per physical barcode/QR scan attempt and immutable across retries",
		RequestTransportIdempotency:    "HTTP transport Idempotency-Key protecting network batch uploads from duplicate transport transmission",
		CanonicalAttendanceIdentity:    "Database compound unique constraint UNIQUE (delegate_id, session_id) on the attendance table enforcing single-admission invariant",
		DistinctScanDistinguishability: "Repeated transmission of the same physical scan reuses scan_event_id and is deduplicated in scan_events audit log; distinct physical scan attempts receive new scan_event_ids and are logged individually as conflicting attempts without corrupting canonical attendance",
		ClientDeduplicationKey:         "Client-generated UUID v4 idempotency token assigned per scan attempt prior to network transmission",
		ClockSkewMitigation:            "Server does not trust client device timestamps for canonical ordering; server commit timestamp determines official attendance, but client scan timestamp is retained in scan_events for audit verification",
		RawEventVsCanonicalRecord:      "Raw scan attempts (including duplicates and conflicting offline scans) are appended to a raw 'scan_events' table; canonical attendance is enforced via database compound unique constraint on the 'attendance' table",
		AuditResolution:                "Conflicts and repeated scans are flagged with duplicate status in scan_events and visible in admin audit reports without violating the single attendance invariant",
		IsDefaultRecommendation:        true,
		UnresolvedQuestions: []string{
			"Confirm whether earliest real-world scan timestamp is required instead of server-commit order if network sync is delayed for extended periods",
			"Define maximum permissible offline buffering window before cached volunteer scan tokens expire",
		},
	}
}

// evaluateDecisionTraces links key architecture decisions to requirements and evidence.
func (e *Evaluator) evaluateDecisionTraces(req *model.NormalizedRequirements, summary *DecisionSummary) []model.DecisionTrace {
	traces := make([]model.DecisionTrace, 0)

	// Trace 1: Topology
	traces = append(traces, model.DecisionTrace{
		Decision: fmt.Sprintf("Recommended Topology: %s", summary.RecommendedTopology),
		SupportingRequirements: []string{
			fmt.Sprintf("Domain classification: %s", req.Domain),
			fmt.Sprintf("Traffic baseline %0.1f RPS, peak %0.1f RPS", req.Traffic.ExpectedRPS, req.Traffic.PeakRPS),
			"Single cloud environment deployment constraint",
		},
		EvidenceAndProvenance: "Calculated throughput profiles and simplicity-by-default architecture principle",
		AlternativesConsidered: []string{
			"Distributed Microservices",
			"Event-Driven Service Mesh",
		},
		RejectedReasons: []string{
			"Single unified team and cohesive operational boundary favor modular monolith; microservices introduce network serialization, distributed transaction failure modes, and deployment complexity without operational necessity",
		},
		KnownTradeoffs: []string{
			"Shared process boundary requires disciplined package separation and modular boundaries",
		},
		Confidence: 0.95,
	})

	// Trace 2: Database Uniqueness & Duplicate Prevention
	if req.Invariants.PreventDuplicates {
		traces = append(traces, model.DecisionTrace{
			Decision: "Database Compound Unique Constraint (e.g. UNIQUE (delegate_id, session_id)) with ATOMIC ON CONFLICT Handling",
			SupportingRequirements: []string{
				"Explicit requirement to prevent duplicate attendance records across simultaneous volunteer scans",
				"Reliable check-in under network retries and multi-device concurrency",
			},
			EvidenceAndProvenance: "Direct user invariant stating a delegate cannot check in multiple times for the same session",
			AlternativesConsidered: []string{
				"Application-level SELECT-then-INSERT checking",
				"Distributed Redis Lock (Redlock)",
				"Pessimistic Table/Row Locking",
			},
			RejectedReasons: []string{
				"Application-level check-then-act suffers from race conditions under concurrent volunteer scans",
				"Distributed locks introduce external point of failure and latency overhead",
				"Pessimistic row locking degrades throughput unnecessarily when conflict rate is low",
			},
			KnownTradeoffs: []string{
				"Requires relational database engine supporting unique constraints and atomic conflict clauses (e.g. PostgreSQL)",
			},
			Confidence: 1.0,
		})
	}

	// Trace 3: Transactional Outbox Pattern
	if summary.RecommendedDataRules.RequiresOutboxPattern {
		traces = append(traces, model.DecisionTrace{
			Decision: "Transactional Outbox Pattern for Asynchronous Side Effects",
			SupportingRequirements: []string{
				"Decoupling asynchronous tasks (certificate generation, bulk email notifications) from fast check-in responses",
				"Dual-write safety between business record insertion and asynchronous job publishing",
			},
			EvidenceAndProvenance: "Requirement for quick attendance verification while certificate generation and notifications run asynchronously",
			AlternativesConsidered: []string{
				"Synchronous inline task execution",
				"Direct message broker publish inside database transaction",
			},
			RejectedReasons: []string{
				"Synchronous PDF generation creates unacceptable latency for scanning volunteers",
				"Direct broker publish inside DB transaction risks dual-write inconsistency and connection starvation on network delay",
			},
			KnownTradeoffs: []string{
				"Requires background dispatcher worker to poll outbox table and at-least-once message handling",
			},
			Confidence: 0.95,
		})
	}

	return traces
}

// evaluateQueueRules configures bounded queues, backpressure, and dead-letter handling.
func (e *Evaluator) evaluateQueueRules(req *model.NormalizedRequirements, class *model.ClassificationResult, dataRules model.DataRules) model.QueueRules {
	needsQueue := dataRules.RequiresOutboxPattern ||
		class.HasTrait(model.TraitBackgroundIntensive) ||
		req.Domain == model.DomainECommerce ||
		req.Domain == model.DomainFinTech ||
		req.Domain == model.DomainAsyncPipeline ||
		len(req.Invariants.AsyncOperations) > 0

	if !needsQueue {
		return model.QueueRules{Enabled: false}
	}

	queue := model.QueueRules{
		Enabled:                         true,
		MaxWorkerConcurrency:            10,
		MaxRetryAttempts:                3,
		BackoffStrategy:                 "EXPONENTIAL_WITH_JITTER",
		DeadLetterHandling:              "Quarantine in failed_jobs table with structured error log and alert metric",
		DeliveryGuarantee:               "AT_LEAST_ONCE (handlers must be idempotent)",
		ExternalDispatchFailureHandling: "Worker crash after successful external dispatch prior to DB status commit is modeled as AT_LEAST_ONCE; downstream providers must support idempotency tokens or reconciliation",
		ProviderIdempotencyStrategy:     "Dispatcher includes deterministic external idempotency key (e.g. outbox_event_id or job_id) in external third-party API calls (e.g. Stripe, SendGrid) to prevent duplicate external side-effects",
		CrashRecoveryBehavior:           "Worker reclaims unacknowledged or timed-out in-flight jobs via SKIP LOCKED lease timeout; uncommitted external calls will be retried safely with the identical provider idempotency token",
	}

	// Scale-appropriate queue mechanism:
	// If throughput < 1000 RPS and using relational storage: use PostgreSQL SKIP LOCKED queue to avoid extra brokers!
	if req.Traffic.PeakRPS < 1000.0 && req.Data.PrimaryStorage == "RELATIONAL" {
		queue.QueueMechanism = "POSTGRES_SKIP_LOCKED"
	} else {
		queue.QueueMechanism = "REDIS_STREAMS"
	}

	if req.Traffic.PeakRPS > 500 {
		queue.MaxWorkerConcurrency = 25
		queue.MaxRetryAttempts = 5
		queue.WorkerSizingRationale = "High concurrency worker pool (25 workers) sized to process heavy burst queues without accumulating lag."
	} else if len(req.Invariants.AsyncOperations) > 0 {
		queue.MaxWorkerConcurrency = 8
		queue.WorkerSizingRationale = "Bounded concurrency (8 workers) protects CPU/memory from heavy PDF certificate generation and external email API latency, preserving capacity for HTTP handlers."
	} else {
		queue.WorkerSizingRationale = "Standard bounded concurrency (10 workers) ensures steady asynchronous processing without database connection starvation."
	}

	return queue
}

// selectPatterns matches active requirements against the pattern catalog with trade-off justification.
func (e *Evaluator) selectPatterns(req *model.NormalizedRequirements, class *model.ClassificationResult, summary *DecisionSummary) []CandidateEvaluation {
	var candidates []CandidateEvaluation

	// Modular Monolith
	if summary.RecommendedTopology == model.TopologyModularMonolith {
		candidates = append(candidates, CandidateEvaluation{
			PatternID:   "PAT-MODULAR-MONOLITH",
			PatternName: "Modular Monolith",
			Scores: EvaluationDimensions{
				ConsistencyFit:   0.95,
				PerformanceFit:   0.90,
				SimplicityScore:  0.95,
				FailureIsolation: 0.85,
				TotalScore:       0.91,
			},
			IsSelected:    true,
			Justification: "Single deployable unit with strict internal package boundaries; eliminates network hops and distributed consensus overhead.",
		})
	}

	// Outbox Pattern
	if summary.RecommendedDataRules.RequiresOutboxPattern {
		candidates = append(candidates, CandidateEvaluation{
			PatternID:   "PAT-TRANSACTIONAL-OUTBOX",
			PatternName: "Transactional Outbox Pattern",
			Scores: EvaluationDimensions{
				ConsistencyFit:   1.00,
				PerformanceFit:   0.85,
				SimplicityScore:  0.80,
				FailureIsolation: 0.95,
				TotalScore:       0.90,
			},
			IsSelected:    true,
			Justification: "Guarantees dual-write safety by atomically committing business data and outbound event records in the same DB transaction.",
		})
	}

	// Skip Locked Queue
	if summary.RecommendedQueueRules.QueueMechanism == "POSTGRES_SKIP_LOCKED" {
		candidates = append(candidates, CandidateEvaluation{
			PatternID:   "PAT-POSTGRES-SKIP-LOCKED-QUEUE",
			PatternName: "PostgreSQL SKIP LOCKED Job Queue",
			Scores: EvaluationDimensions{
				ConsistencyFit:   0.95,
				PerformanceFit:   0.80,
				SimplicityScore:  0.95,
				FailureIsolation: 0.90,
				TotalScore:       0.90,
			},
			IsSelected:    true,
			Justification: "Avoids introducing RabbitMQ or Kafka; leverages PostgreSQL row-level skip-locking for durable worker coordination.",
		})
	}

	// Cache-Aside
	if req.IsReadHeavy() || class.HasTrait(model.TraitReadHeavy) {
		candidates = append(candidates, CandidateEvaluation{
			PatternID:   "PAT-CACHE-ASIDE",
			PatternName: "Cache-Aside",
			Scores: EvaluationDimensions{
				ConsistencyFit:   0.75,
				PerformanceFit:   0.95,
				SimplicityScore:  0.85,
				FailureIsolation: 0.80,
				TotalScore:       0.84,
			},
			IsSelected:    true,
			Justification: "Protects database from high read volume with TTL-bounded caching; falls back gracefully to DB queries on cache miss.",
		})
	}

	// Concurrency Locking
	if summary.RecommendedDataRules.Locking == model.LockingPessimistic {
		candidates = append(candidates, CandidateEvaluation{
			PatternID:   "PAT-PESSIMISTIC-LOCKING",
			PatternName: "Pessimistic Concurrency Control (SELECT FOR UPDATE)",
			Scores: EvaluationDimensions{
				ConsistencyFit:   1.00,
				PerformanceFit:   0.75,
				SimplicityScore:  0.90,
				FailureIsolation: 0.90,
				TotalScore:       0.89,
			},
			IsSelected:    true,
			Justification: "Prevents race conditions and double-spending on shared inventory rows during burst concurrency.",
		})
	} else if summary.RecommendedDataRules.Locking == model.LockingOptimistic {
		candidates = append(candidates, CandidateEvaluation{
			PatternID:   "PAT-OPTIMISTIC-LOCKING",
			PatternName: "Optimistic Concurrency Control",
			Scores: EvaluationDimensions{
				ConsistencyFit:   0.90,
				PerformanceFit:   0.90,
				SimplicityScore:  0.85,
				FailureIsolation: 0.85,
				TotalScore:       0.88,
			},
			IsSelected:    true,
			Justification: "Uses entity version checking for low-contention entities without holding database locks.",
		})
	}

	return candidates
}
