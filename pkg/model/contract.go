package model

import (
	"errors"
	"fmt"
	"strings"
)

// TopologyType specifies the recommended top-level application structure.
type TopologyType string

const (
	TopologyModularMonolith TopologyType = "MODULAR_MONOLITH" // Default recommended topology for simplicity
	TopologyEventDriven     TopologyType = "EVENT_DRIVEN_SERVICE"
	TopologyPipeline        TopologyType = "ASYNC_WORKER_PIPELINE"
	TopologyMicroservices   TopologyType = "MICROSERVICES" // Strictly when high scale / separate teams mandate it
)

// LockingStrategy defines concurrency control mechanisms for transactional tables.
type LockingStrategy string

const (
	LockingNone        LockingStrategy = "NONE"
	LockingOptimistic  LockingStrategy = "OPTIMISTIC"  // Version/updated_at check
	LockingPessimistic LockingStrategy = "PESSIMISTIC" // SELECT FOR UPDATE
)

// SystemTopology defines high-level architecture decisions and layout.
type SystemTopology struct {
	// Type is the chosen architectural style.
	Type TopologyType `json:"type"`

	// Rationale explains why this topology was chosen over alternatives.
	Rationale string `json:"rationale"`

	// ScalingStrategy describes how the system scales (e.g. "HORIZONTALLY_STATELESS").
	ScalingStrategy string `json:"scaling_strategy"`
}

// ComponentDefinition defines a distinct boundary, service, or module.
type ComponentDefinition struct {
	// Name of the component or module (e.g. "OrderService", "OutboxWorker").
	Name string `json:"name"`

	// Responsibilities summarizes what this component handles.
	Responsibilities []string `json:"responsibilities"`

	// Dependencies lists names of other internal components it interacts with.
	Dependencies []string `json:"dependencies"`

	// PersistenceAccess notes if it reads/writes the database directly.
	PersistenceAccess bool `json:"persistence_access"`
}

// DataRules governs database transaction boundaries, locking, and consistency.
type DataRules struct {
	// IsolationLevel recommended (e.g., "READ_COMMITTED", "REPEATABLE_READ", "SERIALIZABLE").
	IsolationLevel string `json:"isolation_level"`

	// Locking indicates concurrency control method.
	Locking LockingStrategy `json:"locking"`

	// RequiresIdempotencyKey marks if mutating endpoints must require idempotency tokens.
	RequiresIdempotencyKey bool `json:"requires_idempotency_key"`

	// RequiresOutboxPattern marks whether DB transactions writing events must use an outbox table.
	RequiresOutboxPattern bool `json:"requires_outbox_pattern"`

	// UniquenessStrategy defines database uniqueness mechanics (e.g. compound unique index).
	UniquenessStrategy string `json:"uniqueness_strategy,omitempty"`

	// ConflictHandling defines atomic conflict behavior (e.g. "ON CONFLICT DO NOTHING").
	ConflictHandling string `json:"conflict_handling,omitempty"`

	// ConnectionPoolMax recommended maximum open database connections.
	ConnectionPoolMax int `json:"connection_pool_max"`

	// PoolSizingRationale explains the mathematical or heuristic derivation of pool sizing.
	PoolSizingRationale string `json:"pool_sizing_rationale,omitempty"`
}

// QueueRules governs background jobs, queue capacity, retries, and failure recovery.
type QueueRules struct {
	// Enabled indicates if background queue/workers are recommended.
	Enabled bool `json:"enabled"`

	// QueueMechanism recommended (e.g. "POSTGRES_SKIP_LOCKED", "REDIS_STREAMS", "DEDICATED_BROKER").
	QueueMechanism string `json:"queue_mechanism,omitempty"`

	// DeliveryGuarantee describes delivery semantics (e.g. "AT_LEAST_ONCE (handlers must be idempotent)").
	DeliveryGuarantee string `json:"delivery_guarantee,omitempty"`

	// MaxWorkerConcurrency is the bounded number of concurrent job workers.
	MaxWorkerConcurrency int `json:"max_worker_concurrency,omitempty"`

	// WorkerSizingRationale explains worker concurrency bounds based on CPU/IO expectations.
	WorkerSizingRationale string `json:"worker_sizing_rationale,omitempty"`

	// MaxRetryAttempts is the maximum retry count before routing to DLQ.
	MaxRetryAttempts int `json:"max_retry_attempts,omitempty"`

	// BackoffStrategy defines the retry pacing (e.g. "EXPONENTIAL_WITH_JITTER").
	BackoffStrategy string `json:"backoff_strategy,omitempty"`

	// DeadLetterHandling describes dead-letter capture and alerting strategy.
	DeadLetterHandling string `json:"dead_letter_handling,omitempty"`
}

// ImplementationConstraint gives concrete instructions that the AI coding agent must obey.
type ImplementationConstraint struct {
	// Rule is a concise imperative guideline (e.g., "Do not call external payment gateway inside DB tx").
	Rule string `json:"rule"`

	// Category groups rules (e.g., "DATABASE", "CONCURRENCY", "NETWORKING", "STRUCTURE").
	Category string `json:"category"`

	// Severity indicates if violation is a hard failure ("ERROR") or advisory ("WARNING").
	Severity string `json:"severity"`

	// Rationale explains the architectural reason for this constraint.
	Rationale string `json:"rationale"`
}

// RejectedAlternative documents a pattern considered but rejected.
type RejectedAlternative struct {
	// Pattern is the rejected design option (e.g., "Distributed Microservices", "Kafka Broker").
	Pattern string `json:"pattern"`

	// Reason explains why this option was inappropriate or overengineered.
	Reason string `json:"reason"`
}

// ValidationCriterion describes a check that can verify implementation compliance.
type ValidationCriterion struct {
	// Identifier is a unique check code (e.g., "CHK-IDEMP-01").
	Identifier string `json:"identifier"`

	// Description explains what is inspected.
	Description string `json:"description"`

	// IsAutomated indicates if this can be statically checked by the validator.
	IsAutomated bool `json:"is_automated"`
}

// ArchitectureContract is the immutable, machine-readable contract returned to AI coding agents.
type ArchitectureContract struct {
	// ContractVersion identifies the schema specification version.
	ContractVersion string `json:"contract_version"`

	// ProjectName is the name of the evaluated project.
	ProjectName string `json:"project_name"`

	// Topology defines the top-level architectural style.
	Topology SystemTopology `json:"topology"`

	// Components lists the modules and boundaries.
	Components []ComponentDefinition `json:"components"`

	// Data defines transaction, concurrency, and persistence rules.
	Data DataRules `json:"data"`

	// Queue defines background job and worker rules.
	Queue QueueRules `json:"queue"`

	// Constraints lists concrete DOs and DONTs for the coding agent.
	Constraints []ImplementationConstraint `json:"constraints"`

	// RejectedAlternatives explains why competing designs were discarded.
	RejectedAlternatives []RejectedAlternative `json:"rejected_alternatives"`

	// ValidationCriteria describes how to verify the generated implementation.
	ValidationCriteria []ValidationCriterion `json:"validation_criteria"`

	// ActiveAssumptions notes non-blocking assumptions the agent must keep in mind.
	ActiveAssumptions []string `json:"active_assumptions"`
}

// Validate verifies that the contract contains all mandatory sections and valid values.
func (c *ArchitectureContract) Validate() error {
	if c.ContractVersion == "" {
		return errors.New("contract_version is required")
	}
	if c.ProjectName == "" {
		return errors.New("project_name is required")
	}
	if c.Topology.Type == "" {
		return errors.New("topology.type is required")
	}
	if len(c.Components) == 0 {
		return errors.New("at least one component definition is required")
	}
	if c.Data.IsolationLevel == "" {
		return errors.New("data.isolation_level is required")
	}
	return c.ValidateConsistency()
}

// ValidateConsistency checks for internal contradictions within the contract.
func (c *ArchitectureContract) ValidateConsistency() error {
	// 1. Outbox contradiction check:
	// If outbox pattern is NOT required, no component may claim to poll or dispatch an outbox table.
	if !c.Data.RequiresOutboxPattern {
		for _, comp := range c.Components {
			if strings.Contains(strings.ToLower(comp.Name), "outbox") {
				return fmt.Errorf("contract contradiction: component %q references outbox but data.requires_outbox_pattern is false", comp.Name)
			}
			for _, resp := range comp.Responsibilities {
				if strings.Contains(strings.ToLower(resp), "outbox table") || strings.Contains(strings.ToLower(resp), "transactional outbox") {
					return fmt.Errorf("contract contradiction: component %q responsibility %q references outbox but data.requires_outbox_pattern is false", comp.Name, resp)
				}
			}
		}
	}

	// 2. Queue contradiction check:
	// If queue is disabled, no worker component or queue poller should exist.
	if !c.Queue.Enabled {
		for _, comp := range c.Components {
			if strings.Contains(strings.ToLower(comp.Name), "worker") || strings.Contains(strings.ToLower(comp.Name), "dispatcher") {
				return fmt.Errorf("contract contradiction: component %q defined while queue.enabled is false", comp.Name)
			}
		}
	}

	// 3. Idempotency rule agreement:
	// If requires_idempotency_key is true, an idempotency constraint must exist.
	if c.Data.RequiresIdempotencyKey {
		hasIdempConstraint := false
		for _, rule := range c.Constraints {
			if strings.Contains(strings.ToLower(rule.Rule), "idempotency") {
				hasIdempConstraint = true
				break
			}
		}
		if !hasIdempConstraint {
			return errors.New("contract contradiction: data.requires_idempotency_key is true but no idempotency constraint is defined")
		}
	}

	// 4. Uniqueness constraint agreement:
	if c.Data.UniquenessStrategy != "" {
		hasUniqueConstraint := false
		for _, rule := range c.Constraints {
			if strings.Contains(strings.ToLower(rule.Rule), "unique") || strings.Contains(strings.ToLower(rule.Rule), "conflict") {
				hasUniqueConstraint = true
				break
			}
		}
		if !hasUniqueConstraint {
			return errors.New("contract contradiction: data.uniqueness_strategy is defined but no uniqueness constraint is included")
		}
	}

	return nil
}
