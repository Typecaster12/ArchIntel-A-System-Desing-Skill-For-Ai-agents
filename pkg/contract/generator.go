package contract

import (
	"fmt"

	"github.com/sdskill/archintel/pkg/evaluator"
	"github.com/sdskill/archintel/pkg/model"
)

// Generator transforms evaluated architectural decisions into a complete,
// schema-validated ArchitectureContract for consumption by AI coding agents.
type Generator struct {
	rejectionGen *evaluator.RejectionGenerator
}

// New creates a new contract Generator.
func New() *Generator {
	return &Generator{
		rejectionGen: evaluator.NewRejectionGenerator(),
	}
}

// Generate assembles the immutable ArchitectureContract.
func (g *Generator) Generate(
	req *model.NormalizedRequirements,
	class *model.ClassificationResult,
	summary *evaluator.DecisionSummary,
) (*model.ArchitectureContract, error) {
	contract := &model.ArchitectureContract{
		ContractVersion: "1.0.0",
		ProjectName:     req.ProjectName,
		Topology: model.SystemTopology{
			Type:            summary.RecommendedTopology,
			Rationale:       summary.TopologyRationale,
			ScalingStrategy: "HORIZONTALLY_STATELESS",
		},
		Components:            g.buildComponents(req, summary),
		Data:                  summary.RecommendedDataRules,
		Queue:                 summary.RecommendedQueueRules,
		Constraints:           g.buildConstraints(summary),
		RejectedAlternatives: g.rejectionGen.GenerateRejections(req, class, summary),
		ValidationCriteria:   g.buildValidationCriteria(summary),
		ActiveAssumptions:    req.Assumptions,
	}

	// Validate the contract prior to emission to guarantee schema compliance.
	if err := contract.Validate(); err != nil {
		return nil, fmt.Errorf("generated contract failed schema validation: %w", err)
	}

	return contract, nil
}

// buildComponents derives explicit component boundaries from the architecture topology.
func (g *Generator) buildComponents(req *model.NormalizedRequirements, summary *evaluator.DecisionSummary) []model.ComponentDefinition {
	components := make([]model.ComponentDefinition, 0)

	// Primary Core API Module
	coreName := fmt.Sprintf("%sCore", req.ProjectName)
	components = append(components, model.ComponentDefinition{
		Name: coreName,
		Responsibilities: []string{
			"Accept and validate inbound client requests",
			"Execute core domain business logic and state invariants",
			"Enforce transaction boundaries on persistent storage",
		},
		Dependencies:      []string{},
		PersistenceAccess: true,
	})

	// Add Background / Outbox Worker if queue or outbox is active
	if summary.RecommendedDataRules.RequiresOutboxPattern || summary.RecommendedQueueRules.Enabled {
		workerName := "AsyncWorker"
		var responsibilities []string

		if summary.RecommendedDataRules.RequiresOutboxPattern {
			workerName = "OutboxDispatcherWorker"
			responsibilities = []string{
				"Poll pending events from transactional outbox table",
				"Dispatch asynchronous notifications and external webhooks with retry backoff",
				"Mark processed outbox records and route persistent failures to Dead-Letter Queue",
			}
		} else {
			responsibilities = []string{
				"Poll queued background jobs from database job table",
				"Execute asynchronous tasks (e.g. notifications, exports) using bounded concurrency",
				"Record execution status and route persistent failures to Dead-Letter Queue",
			}
		}

		components = append(components, model.ComponentDefinition{
			Name:              workerName,
			Responsibilities: responsibilities,
			Dependencies:      []string{coreName},
			PersistenceAccess: true,
		})
	}

	return components
}

// buildConstraints builds actionable DOs and DONTs for the AI coding agent.
func (g *Generator) buildConstraints(summary *evaluator.DecisionSummary) []model.ImplementationConstraint {
	constraints := make([]model.ImplementationConstraint, 0)

	// 1. Mandatory rule against external calls in transactions
	constraints = append(constraints, model.ImplementationConstraint{
		Rule:      "Do not perform external network or third-party API calls inside an active database transaction",
		Category:  "DATABASE",
		Severity:  "ERROR",
		Rationale: "External network latency holds open database connections and locks, risking connection pool starvation and cascading failure.",
	})

	// 2. Mandatory rule against unbounded goroutines
	constraints = append(constraints, model.ImplementationConstraint{
		Rule:      "Do not spawn unbounded goroutines (e.g. 'go handler()') for incoming traffic; worker concurrency must be strictly bounded",
		Category:  "CONCURRENCY",
		Severity:  "ERROR",
		Rationale: "Unbounded concurrency during traffic bursts exhausts memory, CPU, and database connection limits.",
	})

	// 3. Database Uniqueness & Atomic Conflict Handling
	if summary.RecommendedDataRules.UniquenessStrategy != "" {
		constraints = append(constraints, model.ImplementationConstraint{
			Rule:      "Enforce compound database unique constraint (e.g. UNIQUE (delegate_id, session_id)) with atomic ON CONFLICT handling on mutating records",
			Category:  "DATABASE",
			Severity:  "ERROR",
			Rationale: "Guarantees absolute protection against duplicate attendance/registration writes across concurrent client requests without relying on application-level read-then-write checks.",
		})
	}

	// 4. Idempotency Key requirement
	if summary.RecommendedDataRules.RequiresIdempotencyKey {
		constraints = append(constraints, model.ImplementationConstraint{
			Rule:      "All mutating transactional endpoints must require and validate an 'Idempotency-Key' header",
			Category:  "API_DESIGN",
			Severity:  "ERROR",
			Rationale: "Guarantees safe client retries without creating duplicate orders or double-spending financial balances.",
		})
	}

	// 5. Concurrency Locking requirement
	if summary.RecommendedDataRules.Locking == model.LockingPessimistic {
		constraints = append(constraints, model.ImplementationConstraint{
			Rule:      "Use 'SELECT ... FOR UPDATE' with deterministic row ordering on mutable inventory/balance records",
			Category:  "DATABASE",
			Severity:  "ERROR",
			Rationale: "Prevents race conditions and deadlocks when concurrent transactions update shared records.",
		})
	}

	// 6. Queue row-locking requirement
	if summary.RecommendedQueueRules.QueueMechanism == "POSTGRES_SKIP_LOCKED" {
		constraints = append(constraints, model.ImplementationConstraint{
			Rule:      "Worker queries fetching background jobs must use 'SELECT ... FOR UPDATE SKIP LOCKED'",
			Category:  "QUEUE",
			Severity:  "ERROR",
			Rationale: "Ensures competing workers lock distinct jobs immediately without blocking each other or causing duplicate job processing.",
		})
	}

	return constraints
}

// buildValidationCriteria creates static and testable verification rules for the implementation.
func (g *Generator) buildValidationCriteria(summary *evaluator.DecisionSummary) []model.ValidationCriterion {
	criteria := make([]model.ValidationCriterion, 0)

	criteria = append(criteria, model.ValidationCriterion{
		Identifier:  "CHK-TX-ISOLATION",
		Description: "Verify database transaction blocks contain zero external HTTP/gRPC client calls",
		IsAutomated: true,
	})

	criteria = append(criteria, model.ValidationCriterion{
		Identifier:  "CHK-BOUNDED-CONCURRENCY",
		Description: "Verify all background worker loops use a bounded channel buffer or worker pool",
		IsAutomated: true,
	})

	if summary.RecommendedDataRules.UniquenessStrategy != "" {
		criteria = append(criteria, model.ValidationCriterion{
			Identifier:  "CHK-UNIQUE-CONSTRAINTS",
			Description: "Verify database schema defines compound UNIQUE constraint and atomic conflict handling",
			IsAutomated: true,
		})
	}

	if summary.RecommendedDataRules.RequiresIdempotencyKey {
		criteria = append(criteria, model.ValidationCriterion{
			Identifier:  "CHK-IDEMPOTENCY-KEYS",
			Description: "Verify mutating routes inspect the Idempotency-Key header against the idempotency table",
			IsAutomated: true,
		})
	}

	if summary.RecommendedDataRules.RequiresOutboxPattern {
		criteria = append(criteria, model.ValidationCriterion{
			Identifier:  "CHK-TRANSACTIONAL-OUTBOX",
			Description: "Verify domain state updates and outbound event rows are inserted within the same database transaction",
			IsAutomated: true,
		})
	}

	return criteria
}
