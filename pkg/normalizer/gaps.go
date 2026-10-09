package normalizer

import (
	"fmt"

	"github.com/sdskill/archintel/pkg/model"
)

// ImpactSeverity indicates how seriously a missing requirement influences architecture.
type ImpactSeverity string

const (
	ImpactHigh   ImpactSeverity = "HIGH"   // Determines whether distributed infrastructure or ACID is needed
	ImpactMedium ImpactSeverity = "MEDIUM" // Influences caching or queue sizing
	ImpactLow    ImpactSeverity = "LOW"    // Operational detail, easy to change later
)

// ArchitectureGap represents an identified omission in the user's project requirements.
type ArchitectureGap struct {
	// Field identifies what is missing (e.g., "TRAFFIC_SCALE", "CONSISTENCY_REQUIREMENT").
	Field string `json:"field"`

	// Severity is the architectural impact level.
	Severity ImpactSeverity `json:"severity"`

	// Description explains why this missing information is important.
	Description string `json:"description"`

	// DefaultAssumption states the fallback choice used to avoid blocking the pipeline.
	DefaultAssumption string `json:"default_assumption"`
}

// ClarificationQuestion formats a targeted question for the AI agent or developer.
type ClarificationQuestion struct {
	// Topic identifies the decision area (e.g., "Peak Concurrency").
	Topic string `json:"topic"`

	// Question is the targeted question.
	Question string `json:"question"`

	// SuggestedOptions provides concrete, mutually exclusive choices.
	SuggestedOptions []string `json:"suggested_options"`

	// AppliedDefault is the choice used if the user doesn't answer.
	AppliedDefault string `json:"applied_default"`
}

// GapAnalysis holds the detected omissions, generated questions, and fallback assumptions.
type GapAnalysis struct {
	// Gaps lists all identified requirement omissions.
	Gaps []ArchitectureGap `json:"gaps"`

	// Questions provides user-friendly questions to clarify important unknowns.
	Questions []ClarificationQuestion `json:"questions"`

	// Assumptions lists the fallback assumptions incorporated into the pipeline.
	Assumptions []string `json:"assumptions"`

	// CanProceed is always true; ambiguity does not stall architecture generation.
	CanProceed bool `json:"can_proceed"`
}

// GapDetector scans NormalizedRequirements for critical omissions that alter architecture.
type GapDetector struct{}

// NewGapDetector creates a new gap detector instance.
func NewGapDetector() *GapDetector {
	return &GapDetector{}
}

// Analyze detects missing requirements and generates non-blocking assumptions and questions.
func (d *GapDetector) Analyze(norm *model.NormalizedRequirements) *GapAnalysis {
	analysis := &GapAnalysis{
		Gaps:        make([]ArchitectureGap, 0),
		Questions:   make([]ClarificationQuestion, 0),
		Assumptions: make([]string, 0),
		CanProceed:  true,
	}

	// 1. Check for unstated traffic figures
	if !norm.Traffic.IsExplicit {
		gap := ArchitectureGap{
			Field:             "TRAFFIC_SCALE",
			Severity:          ImpactHigh,
			Description:       "Expected throughput (RPS) and peak user count are unspecified.",
			DefaultAssumption: "Assuming modest early-stage load (< 50 peak RPS). Avoiding distributed clusters.",
		}
		analysis.Gaps = append(analysis.Gaps, gap)
		analysis.Assumptions = append(analysis.Assumptions, gap.DefaultAssumption)

		analysis.Questions = append(analysis.Questions, ClarificationQuestion{
			Topic:    "Throughput and Traffic Scale",
			Question: "What is the expected baseline requests per second (RPS) and anticipated peak burst?",
			SuggestedOptions: []string{
				"Early stage / Low: < 50 RPS",
				"Moderate: 50 - 500 RPS",
				"High: 500 - 5,000 RPS",
				"Very High: > 5,000 RPS",
			},
			AppliedDefault: "Early stage (< 50 RPS)",
		})
	}

	// 2. Check for ambiguous consistency in financial or commerce workflows
	if (norm.Domain == model.DomainECommerce || norm.Domain == model.DomainFinTech) &&
		norm.Consistency != model.ConsistencyStrong {
		gap := ArchitectureGap{
			Field:             "FINANCIAL_CONSISTENCY",
			Severity:          ImpactHigh,
			Description:       fmt.Sprintf("Domain is %s, but strict ACID consistency was not explicitly requested.", norm.Domain),
			DefaultAssumption: "Enforcing strict ACID consistency and row-level locking to prevent double-spending.",
		}
		analysis.Gaps = append(analysis.Gaps, gap)
		analysis.Assumptions = append(analysis.Assumptions, gap.DefaultAssumption)
	}

	// 3. Check for unspecified high contention in inventory or booking
	if norm.Data.HasHighContention && !norm.Traffic.IsBursty {
		gap := ArchitectureGap{
			Field:             "BURST_AND_CONTENTION",
			Severity:          ImpactMedium,
			Description:       "Inventory or balance deduction detected, but burst spikes were not detailed.",
			DefaultAssumption: "Recommending pessimistic concurrency control (SELECT FOR UPDATE) on inventory rows.",
		}
		analysis.Gaps = append(analysis.Gaps, gap)
		analysis.Assumptions = append(analysis.Assumptions, gap.DefaultAssumption)
	}

	// 4. Check for storage preference omission
	if norm.Data.PrimaryStorage == "RELATIONAL" && len(norm.Constraints.PreferredStack) == 0 {
		analysis.Assumptions = append(analysis.Assumptions,
			"Defaulting to PostgreSQL for relational persistence due to native JSON, transactional outbox, and SKIP LOCKED queue capabilities.")
	}

	return analysis
}
