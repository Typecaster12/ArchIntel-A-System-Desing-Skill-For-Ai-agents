package knowledge

import "github.com/sdskill/archintel/pkg/model"

// PatternCategory groups related architectural techniques.
type PatternCategory string

const (
	CategoryTopology    PatternCategory = "TOPOLOGY"
	CategoryData        PatternCategory = "DATA_AND_CONCURRENCY"
	CategoryMessaging   PatternCategory = "ASYNC_AND_MESSAGING"
	CategoryPerformance PatternCategory = "PERFORMANCE_AND_CACHE"
)

// ArchitecturalPattern represents a proven, structured design pattern.
type ArchitecturalPattern struct {
	// ID is the unique identifier (e.g. "PAT-MODULAR-MONOLITH").
	ID string `json:"id"`

	// Name is the human-readable pattern title.
	Name string `json:"name"`

	// Category classifies the pattern.
	Category PatternCategory `json:"category"`

	// Summary is a concise 1-line description of the pattern.
	Summary string `json:"summary"`

	// Prerequisites lists what infrastructure or capability is required.
	Prerequisites []string `json:"prerequisites"`

	// Benefits lists the advantages of adopting this pattern.
	Benefits []string `json:"benefits"`

	// Drawbacks lists operational or code trade-offs.
	Drawbacks []string `json:"drawbacks"`

	// InappropriateWhen explains when this pattern should NOT be used.
	InappropriateWhen string `json:"inappropriate_when"`

	// MatchedTraits maps this pattern to relevant workload traits.
	MatchedTraits []model.WorkloadTrait `json:"matched_traits"`
}
