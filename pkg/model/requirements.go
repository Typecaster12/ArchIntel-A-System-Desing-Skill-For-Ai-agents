package model

import "strings"

// ConsistencyLevel represents the data consistency model required by the system.
type ConsistencyLevel string

const (
	ConsistencyUnspecified  ConsistencyLevel = "UNSPECIFIED"
	ConsistencyStrong       ConsistencyLevel = "STRONG"       // Strict ACID / zero phantom reads / bank balances
	ConsistencyEventual     ConsistencyLevel = "EVENTUAL"     // Reads may lag writes / feed updates / cache
	ConsistencyLinearizable ConsistencyLevel = "LINEARIZABLE" // Global real-time ordering across distributed nodes
)

// DomainCategory classifies the high-level business domain of the project.
type DomainCategory string

const (
	DomainGeneralCRUD   DomainCategory = "CRUD"
	DomainECommerce     DomainCategory = "ECOMMERCE"
	DomainFinTech       DomainCategory = "FINTECH"
	DomainIoTTelemetry  DomainCategory = "IOT_TELEMETRY"
	DomainRealtimeChat  DomainCategory = "REALTIME_CHAT"
	DomainAnalytics     DomainCategory = "ANALYTICS"
	DomainContentMedia  DomainCategory = "CONTENT_MEDIA"
	DomainAsyncPipeline DomainCategory = "ASYNC_PIPELINE"
)

// RawRequirement is the raw payload submitted by the AI agent or developer.
type RawRequirement struct {
	// Prompt is the raw natural language description of the application.
	Prompt string `json:"prompt"`

	// DomainHint is an optional domain category suggested by the agent.
	DomainHint string `json:"domain_hint,omitempty"`

	// PreferredStack lists programming languages, databases, or frameworks requested.
	PreferredStack []string `json:"preferred_stack,omitempty"`

	// StatedTraffic is an optional string specifying user/traffic estimates (e.g. "10k DAU", "500 RPS").
	StatedTraffic string `json:"stated_traffic,omitempty"`

	// ExplicitFlags holds any explicit key-value parameters provided by the caller.
	ExplicitFlags map[string]string `json:"explicit_flags,omitempty"`
}

// TrafficProfile stores normalized throughput and user activity figures.
type TrafficProfile struct {
	// ExpectedRPS is the baseline requests per second under normal conditions.
	ExpectedRPS float64 `json:"expected_rps"`

	// PeakRPS is the estimated maximum burst requests per second.
	PeakRPS float64 `json:"peak_rps"`

	// DailyActiveUsers is the estimated count of active users per day.
	DailyActiveUsers int64 `json:"daily_active_users"`

	// ReadWriteRatio is the ratio of reads to writes (e.g., 10.0 means 10 reads per 1 write).
	ReadWriteRatio float64 `json:"read_write_ratio"`

	// IsBursty indicates if traffic comes in sudden spikes rather than steady streams.
	IsBursty bool `json:"is_bursty"`

	// IsExplicit indicates if these traffic figures were directly specified by the user.
	IsExplicit bool `json:"is_explicit"`

	// StatedAttendees is the total registered attendee or user count mentioned in requirements.
	StatedAttendees int64 `json:"stated_attendees,omitempty"`

	// PeakArrivalWindow is the arrival window text if specified (e.g. "300-500 attendees in 30 minutes").
	PeakArrivalWindow string `json:"peak_arrival_window,omitempty"`

	// DerivedRPSReason explains how RPS was calculated from stated figures or defaults.
	DerivedRPSReason string `json:"derived_rps_reason,omitempty"`

	// TrafficSource indicates whether traffic was EXPLICIT_RPS, DERIVED_FROM_ARRIVALS, or ASSUMED_DEFAULT.
	TrafficSource string `json:"traffic_source,omitempty"`
}

// DataProfile describes storage volume, schema nature, and database guarantees.
type DataProfile struct {
	// PrimaryStorage is the suggested or required storage paradigm (e.g., "RELATIONAL", "DOCUMENT").
	PrimaryStorage string `json:"primary_storage"`

	// DailyVolumeMB is the estimated incoming data volume per day in megabytes.
	DailyVolumeMB float64 `json:"daily_volume_mb"`

	// RequiresACID indicates if strict multi-row transaction isolation is mandatory.
	RequiresACID bool `json:"requires_acid"`

	// HasHighContention indicates whether multiple concurrent requests mutate the same rows (e.g. inventory).
	HasHighContention bool `json:"has_high_contention"`

	// RetentionDays is how long records must be kept before archival or deletion.
	RetentionDays int `json:"retention_days"`
}

// LatencyProfile describes time budgets for critical operations.
type LatencyProfile struct {
	// P95TargetMs is the 95th percentile target latency in milliseconds.
	P95TargetMs int `json:"p95_target_ms"`

	// P99TargetMs is the 99th percentile target latency in milliseconds.
	P99TargetMs int `json:"p99_target_ms"`

	// MaxAcceptableMs is the hard timeout threshold before client failure.
	MaxAcceptableMs int `json:"max_acceptable_ms"`
}

// Constraints defines resource, budgetary, and technology limits.
type Constraints struct {
	// PreferredStack contains languages, frameworks, or DBs preferred by the caller.
	PreferredStack []string `json:"preferred_stack"`

	// DeploymentTarget represents deployment environment (e.g. "SINGLE_SERVER", "CONTAINER", "SERVERLESS").
	DeploymentTarget string `json:"deployment_target"`

	// MaxMonthlyBudgetUSD is an optional budgetary constraint if provided.
	MaxMonthlyBudgetUSD float64 `json:"max_monthly_budget_usd,omitempty"`

	// ComplianceNeeds lists legal or regulatory rules (e.g., "PCI-DSS", "HIPAA", "GDPR").
	ComplianceNeeds []string `json:"compliance_needs,omitempty"`
}

// InferredTrait represents an architectural characteristic derived by the normalizer or classifier.
type InferredTrait struct {
	// Name is the trait identifier (e.g. "TRANSACTION_HEAVY", "BURSTY_WRITE").
	Name string `json:"name"`

	// Rationale explains why this trait was inferred from the requirements.
	Rationale string `json:"rationale"`

	// Confidence is a calibrated certainty score between 0.0 and 1.0.
	Confidence float64 `json:"confidence"`
}

// BusinessInvariants captures domain invariants and correctness rules explicitly stated in the requirements.
type BusinessInvariants struct {
	// PreventDuplicates is true if unique records (e.g. attendance, registration, payment) must be guaranteed.
	PreventDuplicates bool `json:"prevent_duplicates"`

	// UniquenessTargets lists entities requiring database uniqueness (e.g. "attendance(delegate_id, session_id)").
	UniquenessTargets []string `json:"uniqueness_targets,omitempty"`

	// RequiresIdempotency is true if mutating operations need idempotency keys for safe retries.
	RequiresIdempotency bool `json:"requires_idempotency"`

	// RequiresOfflineHandling is true if client operations must buffer locally and sync later.
	RequiresOfflineHandling bool `json:"requires_offline_handling"`

	// ExternalCallsInTxProhibited is true if network calls must not block DB transactions.
	ExternalCallsInTxProhibited bool `json:"external_calls_in_tx_prohibited"`

	// AsyncOperations lists background jobs to decouple (e.g. "certificate generation", "bulk email").
	AsyncOperations []string `json:"async_operations,omitempty"`
}

// NormalizedRequirements is the canonical internal representation of project requirements.
// It explicitly distinguishes stated facts from inferred traits and default assumptions.
type NormalizedRequirements struct {
	// ProjectName is a normalized identifier for the project.
	ProjectName string `json:"project_name"`

	// Summary is a concise 1-2 sentence description of the system's purpose.
	Summary string `json:"summary"`

	// Domain represents the primary business domain.
	Domain DomainCategory `json:"domain"`

	// Traffic contains calculated throughput metrics.
	Traffic TrafficProfile `json:"traffic"`

	// Data defines persistence and transaction needs.
	Data DataProfile `json:"data"`

	// Latency defines response time goals.
	Latency LatencyProfile `json:"latency"`

	// Consistency is the required data consistency model.
	Consistency ConsistencyLevel `json:"consistency"`

	// Constraints lists technology and deployment limitations.
	Constraints Constraints `json:"constraints"`

	// Invariants captures explicit business invariants that must not be overridden by generic heuristics.
	Invariants BusinessInvariants `json:"invariants"`

	// InferredTraits lists traits derived automatically from the prompt.
	InferredTraits []InferredTrait `json:"inferred_traits"`

	// Assumptions lists fallback assumptions made when the user prompt lacked specific data.
	Assumptions []string `json:"assumptions"`
}

// IsReadHeavy returns true if read operations strongly dominate writes (ratio >= 5.0).
func (n *NormalizedRequirements) IsReadHeavy() bool {
	return n.Traffic.ReadWriteRatio >= 5.0
}

// IsWriteHeavy returns true if write operations dominate or equal reads (ratio <= 1.5).
func (n *NormalizedRequirements) IsWriteHeavy() bool {
	return n.Traffic.ReadWriteRatio <= 1.5 && n.Traffic.ReadWriteRatio > 0.0
}

// HasStrictConsistency returns true when financial or strict ACID consistency is required.
func (n *NormalizedRequirements) HasStrictConsistency() bool {
	return n.Consistency == ConsistencyStrong || n.Consistency == ConsistencyLinearizable || n.Data.RequiresACID
}

// HasHighThroughput returns true if expected baseline RPS exceeds 500 or peak exceeds 2000.
func (n *NormalizedRequirements) HasHighThroughput() bool {
	return n.Traffic.ExpectedRPS >= 500 || n.Traffic.PeakRPS >= 2000
}

// HasComplianceNeed checks if a specific regulatory requirement is present.
func (n *NormalizedRequirements) HasComplianceNeed(need string) bool {
	for _, c := range n.Constraints.ComplianceNeeds {
		if strings.EqualFold(c, need) {
			return true
		}
	}
	return false
}
