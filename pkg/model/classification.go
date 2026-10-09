package model

// WorkloadTrait represents a distinct operational characteristic of an application.
// Multiple traits can be active simultaneously (multi-label classification).
type WorkloadTrait string

const (
	// TraitTransactionHeavy: Financial ledger, checkout, balance deduction with strict ACID invariants.
	TraitTransactionHeavy WorkloadTrait = "TRANSACTION_HEAVY"

	// TraitReadHeavy: Content delivery, catalogs, search-dominated access (Read:Write >= 5:1).
	TraitReadHeavy WorkloadTrait = "READ_HEAVY"

	// TraitWriteHeavy: Telemetry, event ingestion, logs, IoT time-series (Write:Read >= 1.5:1).
	TraitWriteHeavy WorkloadTrait = "WRITE_HEAVY"

	// TraitLatencySensitive: Sub-100ms P99 SLAs, real-time gaming, financial execution.
	TraitLatencySensitive WorkloadTrait = "LATENCY_SENSITIVE"

	// TraitBackgroundIntensive: Heavy asynchronous jobs, file conversion, batch exports.
	TraitBackgroundIntensive WorkloadTrait = "BACKGROUND_PROCESSING_INTENSIVE"

	// TraitConsistencySensitive: Zero tolerance for stale reads or race conditions.
	TraitConsistencySensitive WorkloadTrait = "CONSISTENCY_SENSITIVE"

	// TraitResourceConstrained: Low RAM/CPU environments, serverless execution caps.
	TraitResourceConstrained WorkloadTrait = "RESOURCE_CONSTRAINED"

	// TraitBurstyTraffic: Severe traffic spikes during marketing drops or peak hours.
	TraitBurstyTraffic WorkloadTrait = "BURSTY_TRAFFIC"

	// TraitHighContention: Multiple concurrent users mutably locking the same records.
	TraitHighContention WorkloadTrait = "HIGH_CONTENTION"
)

// ClassifierSource identifies which engine computed the classification result.
type ClassifierSource string

const (
	ClassifierSourceRuleBased ClassifierSource = "RULE_BASED_HEURISTIC"
	ClassifierSourceLayaModel ClassifierSource = "LAYA_DECISION_MODEL"
	ClassifierSourceHybrid    ClassifierSource = "HYBRID_ENSEMBLE"
)

// ClassificationScore represents the calibrated probability score and explanation for a trait.
type ClassificationScore struct {
	// Trait is the workload dimension being evaluated.
	Trait WorkloadTrait `json:"trait"`

	// Score is the calibrated certainty between 0.0 (not present) and 1.0 (strongly present).
	Score float64 `json:"score"`

	// IsActive indicates if the score meets or exceeds the activation threshold (default >= 0.65).
	IsActive bool `json:"is_active"`

	// Rationale explains why this score was assigned.
	Rationale string `json:"rationale"`
}

// ClassificationResult captures the full multi-dimensional workload analysis.
type ClassificationResult struct {
	// Source identifies whether this came from heuristic rules or LAYA inference.
	Source ClassifierSource `json:"source"`

	// ModelVersion records the classifier engine version or model checkpoint used.
	ModelVersion string `json:"model_version"`

	// InferenceLatencyMs records evaluation duration in milliseconds.
	InferenceLatencyMs int64 `json:"inference_latency_ms"`

	// Scores maps each evaluated trait to its detailed score.
	Scores map[WorkloadTrait]ClassificationScore `json:"scores"`

	// ActiveTraits lists all traits that exceeded the activation threshold.
	ActiveTraits []WorkloadTrait `json:"active_traits"`

	// PrimaryTrait is the most prominent characteristic driving architectural decisions.
	PrimaryTrait WorkloadTrait `json:"primary_trait"`

	// FallbackApplied indicates if an external classifier failed and fell back to heuristics.
	FallbackApplied bool `json:"fallback_applied"`
}

// HasTrait checks whether a specific trait is among the active characteristics.
func (c *ClassificationResult) HasTrait(trait WorkloadTrait) bool {
	if score, exists := c.Scores[trait]; exists {
		return score.IsActive
	}
	return false
}

// GetScore returns the numerical score (0.0 to 1.0) for a trait, returning 0.0 if not evaluated.
func (c *ClassificationResult) GetScore(trait WorkloadTrait) float64 {
	if s, exists := c.Scores[trait]; exists {
		return s.Score
	}
	return 0.0
}

// FilterActiveTraits returns all traits with a score at or above the given threshold.
func (c *ClassificationResult) FilterActiveTraits(threshold float64) []WorkloadTrait {
	var active []WorkloadTrait
	for trait, s := range c.Scores {
		if s.Score >= threshold {
			active = append(active, trait)
		}
	}
	return active
}
