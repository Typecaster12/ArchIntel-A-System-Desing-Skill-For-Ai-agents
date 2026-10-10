package validator

// Severity indicates how critical an architectural violation is.
type Severity string

const (
	SeverityError   Severity = "ERROR"   // Hard architectural invariant violation (must be fixed)
	SeverityWarning Severity = "WARNING" // Potential risk or suboptimal pattern (recommend review)
)

// CodeFileProposal represents a proposed source file submitted for validation.
type CodeFileProposal struct {
	// Path is the proposed file location (e.g., "internal/handler/checkout.go").
	Path string `json:"path"`

	// Content is the code snippet or file body.
	Content string `json:"content"`
}

// ImplementationProposal represents an AI agent's proposed architecture,
// file layout, or code snippets to be validated against an active contract.
type ImplementationProposal struct {
	// Files lists proposed code files and snippets.
	Files []CodeFileProposal `json:"files,omitempty"`

	// DeclaredPatterns lists patterns the agent intends to use.
	DeclaredPatterns []string `json:"declared_patterns,omitempty"`

	// HasIdempotencyCheck indicates whether mutating handlers check idempotency.
	HasIdempotencyCheck *bool `json:"has_idempotency_check,omitempty"`

	// HasBoundedWorkers indicates whether concurrency pools are used.
	HasBoundedWorkers *bool `json:"has_bounded_workers,omitempty"`

	// UsesTransactionalOutbox indicates whether DB outbox pattern is implemented.
	UsesTransactionalOutbox *bool `json:"uses_transactional_outbox,omitempty"`
}

// Violation represents an identified breach of the ArchitectureContract.
type Violation struct {
	// RuleID is the check identifier (e.g. "VIOL-SYNC-IN-TX").
	RuleID string `json:"rule_id"`

	// Severity is ERROR or WARNING.
	Severity Severity `json:"severity"`

	// Message describes the violation.
	Message string `json:"message"`

	// File notes which file or module breached the contract.
	File string `json:"file,omitempty"`

	// Remediation provides the concrete correction the coding agent should implement.
	Remediation string `json:"remediation"`
}

// ValidationReport is the verdict emitted after checking a proposal against a contract.
type ValidationReport struct {
	// IsCompliant is true if no ERROR-level violations were found.
	IsCompliant bool `json:"is_compliant"`

	// ErrorsCount is the number of hard contract errors.
	ErrorsCount int `json:"errors_count"`

	// WarningsCount is the number of advisory warnings.
	WarningsCount int `json:"warnings_count"`

	// Violations lists all detected architectural breaches.
	Violations []Violation `json:"violations"`

	// Summary is a human-readable summary of the evaluation.
	Summary string `json:"summary"`

	// Limitations notes known boundaries or interprocedural uncertainties encountered during analysis.
	Limitations []string `json:"limitations,omitempty"`
}
