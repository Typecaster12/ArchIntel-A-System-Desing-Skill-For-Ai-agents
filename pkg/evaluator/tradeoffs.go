package evaluator

import (
	"github.com/sdskill/archintel/pkg/model"
)

// EvaluationDimensions scores a candidate pattern across core architectural trade-offs.
type EvaluationDimensions struct {
	// ConsistencyFit is the alignment with data consistency and ACID requirements (0.0 to 1.0).
	ConsistencyFit float64 `json:"consistency_fit"`

	// PerformanceFit is the alignment with throughput and latency targets (0.0 to 1.0).
	PerformanceFit float64 `json:"performance_fit"`

	// SimplicityScore reflects lower operational overhead and cognitive complexity (0.0 to 1.0).
	// Higher score = simpler to deploy, operate, and maintain.
	SimplicityScore float64 `json:"simplicity_score"`

	// FailureIsolation is the resilience and recovery posture under faults (0.0 to 1.0).
	FailureIsolation float64 `json:"failure_isolation"`

	// TotalScore is the weighted aggregate score.
	TotalScore float64 `json:"total_score"`
}

// CandidateEvaluation represents an evaluated architectural candidate with its trade-offs.
type CandidateEvaluation struct {
	// PatternID identifies the evaluated pattern.
	PatternID string `json:"pattern_id"`

	// PatternName is the human-readable title.
	PatternName string `json:"pattern_name"`

	// Scores contains multi-dimensional evaluation results.
	Scores EvaluationDimensions `json:"scores"`

	// IsSelected indicates whether this candidate was selected as the recommendation.
	IsSelected bool `json:"is_selected"`

	// Justification explains why this pattern was selected or ranked.
	Justification string `json:"justification"`
}

// DecisionSummary captures the holistic architectural verdict.
type DecisionSummary struct {
	// RecommendedTopology specifies the primary application structure.
	RecommendedTopology model.TopologyType `json:"recommended_topology"`

	// TopologyRationale details why this layout was selected.
	TopologyRationale string `json:"topology_rationale"`

	// RecommendedDataRules specifies transaction, concurrency, and outbox rules.
	RecommendedDataRules model.DataRules `json:"recommended_data_rules"`

	// RecommendedQueueRules specifies background job, concurrency, and DLQ rules.
	RecommendedQueueRules model.QueueRules `json:"recommended_queue_rules"`

	// SelectedPatterns lists patterns incorporated into the design.
	SelectedPatterns []CandidateEvaluation `json:"selected_patterns"`
}
