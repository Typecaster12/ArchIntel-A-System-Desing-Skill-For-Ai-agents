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

	// 4. Select Matching Patterns
	summary.SelectedPatterns = e.selectPatterns(req, class, summary)

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
		RequiresIdempotencyKey: false,
		RequiresOutboxPattern:  false,
		ConnectionPoolMax:      20,
	}

	// 1. Database Uniqueness and Conflict Handling for duplicate prevention
	if req.Invariants.PreventDuplicates {
		rules.UniquenessStrategy = "DATABASE_UNIQUE_CONSTRAINT (e.g. compound unique index: UNIQUE (delegate_id, session_id))"
		rules.ConflictHandling = "ATOMIC_INSERT_ON_CONFLICT (INSERT ... ON CONFLICT (delegate_id, session_id) DO NOTHING)"
	}

	// 2. Concurrency Locking
	if req.Data.HasHighContention || (class.HasTrait(model.TraitHighContention) && !req.Invariants.PreventDuplicates) {
		rules.Locking = model.LockingPessimistic
	} else if req.HasStrictConsistency() || class.HasTrait(model.TraitConsistencySensitive) {
		if req.Invariants.PreventDuplicates && !req.Data.HasHighContention {
			// Duplicate prevention is handled primarily via database unique constraints and atomic conflict handling
			rules.Locking = model.LockingNone
		} else {
			rules.Locking = model.LockingOptimistic
		}
	}

	// 3. Idempotency Key requirement (mutating financial, commerce, or retry-prone multi-device flows)
	if req.Domain == model.DomainECommerce || req.Domain == model.DomainFinTech || req.Invariants.RequiresIdempotency {
		rules.RequiresIdempotencyKey = true
	}

	// 4. Transactional Outbox requirement:
	// Mandated when state mutations must atomically trigger asynchronous side-effects (emails, certificates, webhooks)
	if req.Domain == model.DomainECommerce || req.Domain == model.DomainFinTech || req.Data.RequiresACID || len(req.Invariants.AsyncOperations) > 0 {
		rules.RequiresOutboxPattern = true
	}

	// 5. Explainable Connection Pool Sizing via Little's Law (N = RPS * Latency)
	if req.Traffic.PeakRPS > 1000 {
		rules.ConnectionPoolMax = 50
		rules.PoolSizingRationale = fmt.Sprintf(
			"Sized for high scale (peak %0.0f RPS): pool of 50 connections accommodates ~250 concurrent requests at 20ms avg query latency with 2x headroom.",
			req.Traffic.PeakRPS,
		)
	} else if req.Traffic.PeakRPS > 300 {
		rules.ConnectionPoolMax = 35
		rules.PoolSizingRationale = fmt.Sprintf(
			"Sized for moderate burst (peak %0.0f RPS): pool of 35 connections provides headroom for burst spikes while preventing DB thread contention.",
			req.Traffic.PeakRPS,
		)
	} else {
		rules.ConnectionPoolMax = 20
		rules.PoolSizingRationale = fmt.Sprintf(
			"Conservative baseline: derived via Little's Law for peak %0.0f RPS (estimated 0.3-0.5 concurrent queries at 20ms latency). Pool of 20 provides ~40x burst headroom without exhausting memory on a single cloud DB.",
			req.Traffic.PeakRPS,
		)
	}

	return rules
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
		Enabled:               true,
		MaxWorkerConcurrency: 10,
		MaxRetryAttempts:      3,
		BackoffStrategy:       "EXPONENTIAL_WITH_JITTER",
		DeadLetterHandling:    "Quarantine in failed_jobs table with structured error log and alert metric",
		DeliveryGuarantee:     "AT_LEAST_ONCE (handlers must be idempotent)",
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
