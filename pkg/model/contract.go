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
	LockingNone            LockingStrategy = "NONE"
	LockingDatabaseManaged LockingStrategy = "DATABASE_MANAGED_CONFLICT" // Atomic unique indexes, INSERT ... ON CONFLICT
	LockingOptimistic      LockingStrategy = "OPTIMISTIC"                // Version/updated_at check
	LockingPessimistic     LockingStrategy = "PESSIMISTIC"               // SELECT FOR UPDATE
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

// TransactionPolicy clarifies ACID transaction requirements, per-operation boundaries, and consistency tolerances.
type TransactionPolicy struct {
	// RequiresACID indicates if atomic multi-step transactions are required.
	RequiresACID bool `json:"requires_acid"`

	// Scope indicates the transaction boundary ("PER_OPERATION_ATOMIC", "NONE", "GLOBAL_STRONG").
	Scope string `json:"scope"`

	// AtomicOperations lists specific business operations requiring multi-step database atomicity.
	AtomicOperations []string `json:"atomic_operations,omitempty"`

	// DatabaseEnforcedInvariants lists invariants enforced directly by database constraints (unique indexes, foreign keys).
	DatabaseEnforcedInvariants []string `json:"database_enforced_invariants,omitempty"`

	// EventualConsistencyAllowed lists asynchronous side effects and read paths that can tolerate lag.
	EventualConsistencyAllowed []string `json:"eventual_consistency_allowed,omitempty"`

	// Rationale provides architectural justification for these transaction boundaries.
	Rationale string `json:"rationale,omitempty"`
}

// ConcurrencyControlDetails makes locking and conflict mechanics explicit.
type ConcurrencyControlDetails struct {
	// Mechanism describes the primary duplicate prevention and concurrency control mechanism.
	Mechanism string `json:"mechanism"`

	// ExplicitLockingRequired indicates if application-level row locks (e.g. SELECT FOR UPDATE) are needed.
	ExplicitLockingRequired bool `json:"explicit_locking_required"`

	// ConcurrentConflictBehavior explains what happens when concurrent requests write conflicting data.
	ConcurrentConflictBehavior string `json:"concurrent_conflict_behavior"`

	// IsolationLevel clarifies transaction isolation (e.g. "READ_COMMITTED").
	IsolationLevel string `json:"isolation_level"`

	// QueueWorkerClaimLocking documents how background workers safely claim jobs.
	QueueWorkerClaimLocking string `json:"queue_worker_claim_locking,omitempty"`

	// Rationale explains why this locking design was chosen over heavier alternatives.
	Rationale string `json:"rationale,omitempty"`
}

// IdempotencyPolicy defines comprehensive request deduplication and idempotency mechanics.
type IdempotencyPolicy struct {
	// Required indicates if request-level idempotency enforcement is mandatory.
	Required bool `json:"required"`

	// ApplicableOperations lists specific mutating operations requiring request deduplication tokens.
	ApplicableOperations []string `json:"applicable_operations,omitempty"`

	// KeyScope defines the namespace and uniqueness boundary (e.g. "HEADER_IDEMPOTENCY_KEY_SCOPED_BY_OPERATION").
	KeyScope string `json:"key_scope"`

	// FingerprintMechanism defines how request payload tampering or reuse across different bodies is detected.
	FingerprintMechanism string `json:"fingerprint_mechanism"`

	// StateStorageMechanism defines where and how idempotency records are stored.
	StateStorageMechanism string `json:"state_storage_mechanism"`

	// ConcurrentRequestHandling defines behavior when an in-flight request arrives with the same active key.
	ConcurrentRequestHandling string `json:"concurrent_request_handling"`

	// PayloadMismatchBehavior defines behavior when a key is reused with a different request payload fingerprint.
	PayloadMismatchBehavior string `json:"payload_mismatch_behavior"`

	// ResponseReplayBehavior defines how lost-response retries are fulfilled without re-executing business logic.
	ResponseReplayBehavior string `json:"response_replay_behavior"`

	// RetentionPeriod defines time-to-live before idempotency keys expire.
	RetentionPeriod string `json:"retention_period"`

	// EnforcementValidationCriteria describes how static or runtime verification proves actual enforcement beyond header presence.
	EnforcementValidationCriteria string `json:"enforcement_validation_criteria"`

	// DatabaseUniquenessAlternative explains which operations rely directly on database unique constraints instead of idempotency tokens.
	DatabaseUniquenessAlternative string `json:"database_uniqueness_alternative,omitempty"`
}

// DataRules governs database transaction boundaries, locking, and consistency.
type DataRules struct {
	// IsolationLevel recommended (e.g., "READ_COMMITTED", "REPEATABLE_READ", "SERIALIZABLE").
	IsolationLevel string `json:"isolation_level"`

	// Locking indicates concurrency control method.
	Locking LockingStrategy `json:"locking"`

	// RequiresACID marks if atomic transactions are required.
	RequiresACID bool `json:"requires_acid"`

	// TransactionPolicy details atomic boundaries and invariant enforcement.
	TransactionPolicy TransactionPolicy `json:"transaction_policy"`

	// ConcurrencyDetails details explicit vs database-managed locking mechanics.
	ConcurrencyDetails ConcurrencyControlDetails `json:"concurrency_details"`

	// RequiresIdempotencyKey marks if mutating endpoints must require idempotency tokens.
	RequiresIdempotencyKey bool `json:"requires_idempotency_key"`

	// IdempotencyPolicy defines complete request deduplication, payload fingerprinting, and replay rules.
	IdempotencyPolicy IdempotencyPolicy `json:"idempotency_policy"`

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

	// ExternalDispatchFailureHandling models worker crash after external call success prior to DB commit.
	ExternalDispatchFailureHandling string `json:"external_dispatch_failure_handling,omitempty"`

	// ProviderIdempotencyStrategy describes client tokens passed to external providers to prevent duplicate external side-effects.
	ProviderIdempotencyStrategy string `json:"provider_idempotency_strategy,omitempty"`

	// CrashRecoveryBehavior details how worker restart safely picks up pending or in-flight events.
	CrashRecoveryBehavior string `json:"crash_recovery_behavior,omitempty"`

	// JobClaimMechanism documents the SQL query and concurrency mechanics for atomic claiming.
	JobClaimMechanism string `json:"job_claim_mechanism,omitempty"`

	// ClaimLeaseDuration notes the duration after which an uncompleted job can be reclaimed by another worker.
	ClaimLeaseDuration string `json:"claim_lease_duration,omitempty"`

	// LeaseRenewalPolicy defines heartbeat renewal mechanics for long-running jobs (e.g. bulk certificates).
	LeaseRenewalPolicy string `json:"lease_renewal_policy,omitempty"`

	// FencingTokenMechanism defines stale worker rejection mechanics upon completion.
	FencingTokenMechanism string `json:"fencing_token_mechanism,omitempty"`

	// ProviderIntegrations details provider-specific idempotency, retention, and ambiguous timeout reconciliation.
	ProviderIntegrations []ProviderIdempotencySpec `json:"provider_integrations,omitempty"`
}

// ProviderIdempotencySpec documents integration-specific idempotency, retention windows, and reconciliation strategies.
type ProviderIdempotencySpec struct {
	// ProviderName identifies the external service or subsystem (e.g. "Stripe", "SendGrid", "S3/BlobStore").
	ProviderName string `json:"provider_name"`

	// Mechanism explains the deduplication method (e.g. "HEADER_IDEMPOTENCY_KEY", "DETERMINISTIC_BLOB_PATH", "WEBHOOK_RECONCILIATION").
	Mechanism string `json:"mechanism"`

	// TokenFormat describes the format or composition of the idempotency token.
	TokenFormat string `json:"token_format"`

	// RetentionWindow notes how long the external service retains deduplication records.
	RetentionWindow string `json:"retention_window"`

	// AmbiguousTimeoutPlan explains recovery when a network timeout leaves external execution state uncertain.
	AmbiguousTimeoutPlan string `json:"ambiguous_timeout_plan"`
}

// CapacityPlan documents connection pool and worker sizing derivations using Little's Law.
type CapacityPlan struct {
	// QuantityBeingEstimated states the specific quantity modeled (e.g. "Concurrent Active Database Transactions").
	QuantityBeingEstimated string `json:"quantity_being_estimated"`

	// SelectedScenario names the specific traffic scenario modeled (e.g. "Conservative Peak Burst Scenario").
	SelectedScenario string `json:"selected_scenario"`

	// ScenarioRationale explains why this scenario was selected from the planning traffic range.
	ScenarioRationale string `json:"scenario_rationale"`

	// EstimatedArrivalRateRange notes the derived planning arrival range (e.g. "15.0 - 25.0 HTTP RPS").
	EstimatedArrivalRateRange string `json:"estimated_arrival_rate_range"`

	// PlanningArrivalRate notes the single arrival rate lambda used for sizing calculations.
	PlanningArrivalRate float64 `json:"planning_arrival_rate"`

	// LittlesLawFormula shows the formula applied (L = lambda * W).
	LittlesLawFormula string `json:"littles_law_formula"`

	// EstimatedServiceTime notes the average duration W per query or transaction.
	EstimatedServiceTime string `json:"estimated_service_time"`

	// EstimatedAverageConcurrency notes the derived average concurrent load L.
	EstimatedAverageConcurrency float64 `json:"estimated_average_concurrency"`

	// RecommendedDBPoolMax is the safe maximum database connection pool size.
	RecommendedDBPoolMax int `json:"recommended_db_pool_max"`

	// WorkerConcurrencyMax is the bounded number of background workers.
	WorkerConcurrencyMax int `json:"worker_concurrency_max"`

	// HeadroomRationale explains burst protection without claiming unverified capacity guarantees.
	HeadroomRationale string `json:"headroom_rationale"`

	// CapacityCaveats lists assumptions, limitations, and recommendation for empirical load testing.
	CapacityCaveats []string `json:"capacity_caveats"`
}

// OfflineSyncStrategy defines offline check-in buffering, synchronization, and conflict resolution semantics.
type OfflineSyncStrategy struct {
	// Policy is the chosen or proposed conflict resolution policy.
	Policy string `json:"policy"`

	// HandlingState categorizes requirement certainty ("CONFIRMED_REQUIRED", "RECOMMENDED_PROVISIONAL").
	HandlingState string `json:"handling_state,omitempty"`

	// ScanEventIdentity defines the stable client UUID assigned per physical scan event.
	ScanEventIdentity string `json:"scan_event_identity"`

	// RequestTransportIdempotency defines the HTTP transport retry key protecting batch uploads.
	RequestTransportIdempotency string `json:"request_transport_idempotency"`

	// CanonicalAttendanceIdentity defines database-enforced attendance identity.
	CanonicalAttendanceIdentity string `json:"canonical_attendance_identity"`

	// DistinctScanDistinguishability defines how repeated physical scans remain distinguishable from transport retries.
	DistinctScanDistinguishability string `json:"distinct_scan_distinguishability"`

	// ClientDeduplicationKey describes client-side deduplication key format.
	ClientDeduplicationKey string `json:"client_deduplication_key"`

	// ClockSkewMitigation explains handling of client clock drift, tampering, and replay.
	ClockSkewMitigation string `json:"clock_skew_mitigation"`

	// RawEventVsCanonicalRecord documents separation of raw scan attempts from canonical attendance records.
	RawEventVsCanonicalRecord string `json:"raw_event_vs_canonical_record"`

	// AuditResolution explains how conflicts and duplicate scans are recorded for administrative auditing.
	AuditResolution string `json:"audit_resolution"`

	// IsDefaultRecommendation indicates whether this is an applied default rather than an explicit user decision.
	IsDefaultRecommendation bool `json:"is_default_recommendation"`

	// UnresolvedQuestions lists gaps or questions if the policy is provisional.
	UnresolvedQuestions []string `json:"unresolved_questions,omitempty"`
}

// DecisionTrace provides end-to-end traceability from requirements to an architectural decision.
type DecisionTrace struct {
	// Decision is the architectural choice (e.g. "PostgreSQL Compound Unique Index for Attendance").
	Decision string `json:"decision"`

	// SupportingRequirements lists prompt requirements and domain invariants justifying the decision.
	SupportingRequirements []string `json:"supporting_requirements"`

	// EvidenceAndProvenance states the source and certainty of supporting evidence.
	EvidenceAndProvenance string `json:"evidence_and_provenance"`

	// AlternativesConsidered lists discarded alternative patterns.
	AlternativesConsidered []string `json:"alternatives_considered"`

	// RejectedReasons explains why alternatives were not selected.
	RejectedReasons []string `json:"rejected_reasons"`

	// Assumptions notes any provisional assumptions made.
	Assumptions []string `json:"assumptions,omitempty"`

	// KnownTradeoffs highlights trade-offs accepted by this decision.
	KnownTradeoffs []string `json:"known_tradeoffs"`

	// Confidence expresses certainty between 0.0 and 1.0.
	Confidence float64 `json:"confidence"`
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

	// CapacityPlan details connection pool sizing and concurrency limits.
	CapacityPlan CapacityPlan `json:"capacity_plan"`

	// OfflineSync details offline synchronization semantics when offline capability is active.
	OfflineSync *OfflineSyncStrategy `json:"offline_sync,omitempty"`

	// Constraints lists concrete DOs and DONTs for the coding agent.
	Constraints []ImplementationConstraint `json:"constraints"`

	// RejectedAlternatives explains why competing designs were discarded.
	RejectedAlternatives []RejectedAlternative `json:"rejected_alternatives"`

	// ValidationCriteria describes how to verify the generated implementation.
	ValidationCriteria []ValidationCriterion `json:"validation_criteria"`

	// DecisionTraces connects major architectural decisions to explicit requirements and evidence.
	DecisionTraces []DecisionTrace `json:"decision_traces,omitempty"`

	// FieldProvenance maps numeric and architectural parameters to their provenance.
	FieldProvenance map[string]ProvenanceMeta `json:"field_provenance,omitempty"`

	// ActiveAssumptions notes non-blocking assumptions the agent must keep in mind.
	ActiveAssumptions []string `json:"active_assumptions"`

	// UnresolvedDecisions records material questions and provisional assumptions identified by gap analysis.
	UnresolvedDecisions []string `json:"unresolved_decisions,omitempty"`
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
	// Top-level flag and detailed policy must agree
	if c.Data.RequiresIdempotencyKey != c.Data.IdempotencyPolicy.Required {
		return errors.New("contract contradiction: data.requires_idempotency_key does not agree with data.idempotency_policy.required")
	}
	if c.Data.RequiresIdempotencyKey {
		if c.Data.IdempotencyPolicy.FingerprintMechanism == "" {
			return errors.New("contract contradiction: idempotency is required but idempotency_policy.fingerprint_mechanism is missing")
		}
		if c.Data.IdempotencyPolicy.StateStorageMechanism == "" {
			return errors.New("contract contradiction: idempotency is required but idempotency_policy.state_storage_mechanism is missing")
		}
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
		if len(c.Data.IdempotencyPolicy.ApplicableOperations) == 0 {
			return errors.New("contract contradiction: data.requires_idempotency_key is true but data.idempotency_policy.applicable_operations is empty")
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

	// 5. ACID outbox consistency:
	// Transactional outbox requires atomic database transaction capability and an enabled queue.
	if c.Data.RequiresOutboxPattern {
		if !c.Data.RequiresACID && !c.Data.TransactionPolicy.RequiresACID {
			return errors.New("contract contradiction: data.requires_outbox_pattern is true but data.requires_acid is false (outbox requires atomic multi-step database transactions)")
		}
		if !c.Queue.Enabled {
			return errors.New("contract contradiction: data.requires_outbox_pattern is true but queue.enabled is false (transactional outbox requires an active queue/worker mechanism to dispatch events)")
		}
	}

	// 5b. Required persistence and transaction settings:
	if c.Data.RequiresACID {
		if strings.TrimSpace(c.Data.TransactionPolicy.Scope) == "" {
			return errors.New("contract contradiction: data.requires_acid is true but data.transaction_policy.scope is missing or empty")
		}
		if strings.TrimSpace(c.Data.IsolationLevel) == "" && strings.TrimSpace(c.Data.ConcurrencyDetails.IsolationLevel) == "" {
			return errors.New("contract contradiction: data.requires_acid is true but transaction isolation_level is missing or empty")
		}
	}

	// 6. At-least-once delivery requires idempotency or deduplication
	if c.Queue.Enabled && strings.Contains(strings.ToUpper(c.Queue.DeliveryGuarantee), "AT_LEAST_ONCE") {
		hasIdempOrDedup := strings.Contains(strings.ToLower(c.Queue.DeliveryGuarantee), "idempotent")
		for _, rule := range c.Constraints {
			if strings.Contains(strings.ToLower(rule.Rule), "idempotent") || strings.Contains(strings.ToLower(rule.Rule), "deduplication") {
				hasIdempOrDedup = true
				break
			}
		}
		if !hasIdempOrDedup {
			return errors.New("contract contradiction: queue delivery guarantee is AT_LEAST_ONCE but no worker idempotency or deduplication strategy is documented")
		}
	}

	// 7. Queue retry without dead-letter handling or retry limit
	if c.Queue.Enabled && c.Queue.MaxRetryAttempts > 0 && strings.TrimSpace(c.Queue.DeadLetterHandling) == "" {
		return errors.New("contract contradiction: queue retries are enabled but queue.dead_letter_handling is undefined")
	}

	// 8. Bounded concurrency enforcement
	if c.Queue.Enabled && c.Queue.MaxWorkerConcurrency <= 0 {
		return errors.New("contract contradiction: queue is enabled but queue.max_worker_concurrency is not bounded (must be > 0)")
	}

	// 9. External calls prohibited in transactions while execution plan requires them inside tx
	hasProhibition := false
	for _, rule := range c.Constraints {
		if strings.Contains(strings.ToLower(rule.Rule), "external") && strings.Contains(strings.ToLower(rule.Rule), "inside an active database transaction") {
			hasProhibition = true
			break
		}
	}
	if hasProhibition {
		for _, comp := range c.Components {
			for _, resp := range comp.Responsibilities {
				respLower := strings.ToLower(resp)
				if (strings.Contains(respLower, "external payment") || strings.Contains(respLower, "third-party api") || strings.Contains(respLower, "webhook")) &&
					(strings.Contains(respLower, "inside transaction") || strings.Contains(respLower, "inside db tx")) {
					return fmt.Errorf("contract contradiction: component %q responsibility %q places external network call inside DB transaction, violating constraint", comp.Name, resp)
				}
			}
		}
	}

	// 10. Offline handling required while contract has no synchronization or conflict policy
	for _, comp := range c.Components {
		for _, resp := range comp.Responsibilities {
			if strings.Contains(strings.ToLower(resp), "offline") {
				if c.OfflineSync == nil || c.OfflineSync.Policy == "" {
					return fmt.Errorf("contract contradiction: component %q supports offline check-in but contract.offline_sync policy is missing", comp.Name)
				}
			}
		}
	}

	// 11. Provenance consistency: System and assumed defaults must not silently become hard constraints
	for field, meta := range c.FieldProvenance {
		if (meta.Source == ProvenanceAssumedDefault || meta.Source == ProvenanceSystemDefault) && meta.IsHardConstraint {
			return fmt.Errorf("contract contradiction: field %q has provenance %q but is marked as an immutable hard constraint without user justification", field, meta.Source)
		}
	}

	// 12. Worker concurrency consistency across queue and capacity plan
	if c.Queue.Enabled && c.CapacityPlan.WorkerConcurrencyMax > 0 {
		if c.Queue.MaxWorkerConcurrency != c.CapacityPlan.WorkerConcurrencyMax {
			return fmt.Errorf("contract contradiction: queue.max_worker_concurrency (%d) does not match capacity_plan.worker_concurrency_max (%d)",
				c.Queue.MaxWorkerConcurrency, c.CapacityPlan.WorkerConcurrencyMax)
		}
	}

	// 13. Queue worker viability:
	// An enabled queue requires a defined queue mechanism and at least one worker component.
	if c.Queue.Enabled {
		if strings.TrimSpace(c.Queue.QueueMechanism) == "" {
			return errors.New("contract contradiction: queue.enabled is true but queue.queue_mechanism is missing or empty")
		}
		hasWorkerComp := false
		for _, comp := range c.Components {
			nameLower := strings.ToLower(comp.Name)
			if strings.Contains(nameLower, "worker") || strings.Contains(nameLower, "dispatcher") || strings.Contains(nameLower, "consumer") {
				hasWorkerComp = true
				break
			}
			for _, resp := range comp.Responsibilities {
				respLower := strings.ToLower(resp)
				if strings.Contains(respLower, "worker") || strings.Contains(respLower, "process queue") || strings.Contains(respLower, "consume") || strings.Contains(respLower, "dispatch") {
					hasWorkerComp = true
					break
				}
			}
		}
		if !hasWorkerComp {
			return errors.New("contract contradiction: queue.enabled is true but no worker or background processor component is configured in components")
		}
	}

	// 14. Operational connection pool and worker concurrency compatibility
	// Account for whether web handlers and workers share a connection pool, whether connections are held concurrently,
	// and whether limits are operationally compatible.
	if c.Queue.Enabled && c.Data.ConnectionPoolMax > 0 && c.Queue.MaxWorkerConcurrency > 0 {
		workersUseDB := c.Queue.QueueMechanism == "POSTGRES_SKIP_LOCKED"
		webHandlersUseDB := false
		for _, comp := range c.Components {
			nameLower := strings.ToLower(comp.Name)
			isWorker := strings.Contains(nameLower, "worker") || strings.Contains(nameLower, "dispatcher") || strings.Contains(nameLower, "consumer")
			if isWorker && comp.PersistenceAccess {
				workersUseDB = true
			}
			if !isWorker && comp.PersistenceAccess {
				webHandlersUseDB = true
			}
		}

		if workersUseDB {
			if c.Data.ConnectionPoolMax < c.Queue.MaxWorkerConcurrency {
				return fmt.Errorf("contract contradiction: data.connection_pool_max (%d) is strictly less than queue.max_worker_concurrency (%d) while workers access the database; worker pool will starve connection pool",
					c.Data.ConnectionPoolMax, c.Queue.MaxWorkerConcurrency)
			}
			if webHandlersUseDB && c.Data.ConnectionPoolMax == c.Queue.MaxWorkerConcurrency {
				return fmt.Errorf("contract contradiction: data.connection_pool_max (%d) equals queue.max_worker_concurrency (%d) while web handlers and workers share the database connection pool; leaves no headroom for web traffic during worker saturation",
					c.Data.ConnectionPoolMax, c.Queue.MaxWorkerConcurrency)
			}
		}
	}

	return nil
}
