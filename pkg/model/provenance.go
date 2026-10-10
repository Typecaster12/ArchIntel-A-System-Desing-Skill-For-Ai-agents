package model

// ValueProvenance indicates the origin of a requirement or metric value.
type ValueProvenance string

const (
	// ProvenanceUserProvided indicates the value was directly stated by the user.
	ProvenanceUserProvided ValueProvenance = "USER_PROVIDED"

	// ProvenanceDerived indicates the value was calculated or derived from stated figures.
	ProvenanceDerived ValueProvenance = "DERIVED"

	// ProvenanceAssumedDefault indicates a domain-specific fallback assumption was applied.
	ProvenanceAssumedDefault ValueProvenance = "ASSUMED_DEFAULT"

	// ProvenanceSystemDefault indicates a system default convention was applied in the absence of input.
	ProvenanceSystemDefault ValueProvenance = "SYSTEM_DEFAULT"

	// ProvenanceUnknown indicates the value is unresolved and unstated.
	ProvenanceUnknown ValueProvenance = "UNKNOWN"
)

// ProvenanceMeta records origin, derivation method, and constraint status for a requirement field.
type ProvenanceMeta struct {
	// Source identifies the provenance category.
	Source ValueProvenance `json:"source"`

	// Method describes how the value was extracted, derived, or assumed.
	Method string `json:"method,omitempty"`

	// IsHardConstraint indicates whether this is an immutable requirement or a provisional planning estimate.
	IsHardConstraint bool `json:"is_hard_constraint"`

	// Confidence expresses certainty between 0.0 (unconfirmed) and 1.0 (authoritative).
	Confidence float64 `json:"confidence,omitempty"`
}
