package model

import "errors"

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

	// ConnectionPoolMax recommended maximum open database connections.
	ConnectionPoolMax int `json:"connection_pool_max"`
}

// QueueRules governs background jobs, queue capacity, retries, and failure recovery.
type QueueRules struct {
	// Enabled indicates if background queue/workers are recommended.
	Enabled bool `json:"enabled"`

	// QueueMechanism recommended (e.g. "POSTGRES_SKIP_LOCKED", "REDIS_STREAMS", "DEDICATED_BROKER").
	QueueMechanism string `json:"queue_mechanism,omitempty"`

	// MaxWorkerConcurrency is the bounded number of concurrent job workers.
	MaxWorkerConcurrency int `json:"max_worker_concurrency,omitempty"`

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
	return nil
}
