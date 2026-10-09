package validator

import (
	"fmt"
	"strings"

	"github.com/sdskill/archintel/pkg/model"
)

// Validator evaluates an implementation proposal or code snippets against an ArchitectureContract.
type Validator struct{}

// New creates a new Validator.
func New() *Validator {
	return &Validator{}
}

// Validate checks the proposal against the active contract, returning a structured report.
func (v *Validator) Validate(contract *model.ArchitectureContract, proposal ImplementationProposal) *ValidationReport {
	report := &ValidationReport{
		IsCompliant: true,
		Violations:  make([]Violation, 0),
	}

	// 1. Check explicit proposal flags against contract requirements
	v.checkDeclaredFlags(contract, proposal, report)

	// 2. Perform static analysis on submitted code files
	for _, file := range proposal.Files {
		v.inspectFile(contract, file, report)
	}

	// Calculate counts and compliance status
	for _, viol := range report.Violations {
		if viol.Severity == SeverityError {
			report.ErrorsCount++
			report.IsCompliant = false
		} else if viol.Severity == SeverityWarning {
			report.WarningsCount++
		}
	}

	if report.IsCompliant {
		report.Summary = "Implementation proposal complies with the ArchitectureContract."
	} else {
		report.Summary = fmt.Sprintf(
			"Implementation proposal contains %d contract violation(s) that must be remediated.",
			report.ErrorsCount,
		)
	}

	return report
}

// checkDeclaredFlags verifies high-level architectural commitments against contract rules.
func (v *Validator) checkDeclaredFlags(contract *model.ArchitectureContract, proposal ImplementationProposal, report *ValidationReport) {
	// Idempotency requirement check
	if contract.Data.RequiresIdempotencyKey && proposal.HasIdempotencyCheck != nil && !*proposal.HasIdempotencyCheck {
		report.Violations = append(report.Violations, Violation{
			RuleID:   "VIOL-MISSING-IDEMPOTENCY",
			Severity: SeverityError,
			Message:  "The contract mandates idempotency keys on mutating routes, but the proposal omits idempotency checking.",
			Remediation: "Implement an Idempotency-Key validation middleware checking request tokens against a dedicated idempotency table.",
		})
	}

	// Transactional Outbox requirement check
	if contract.Data.RequiresOutboxPattern && proposal.UsesTransactionalOutbox != nil && !*proposal.UsesTransactionalOutbox {
		report.Violations = append(report.Violations, Violation{
			RuleID:   "VIOL-MISSING-OUTBOX",
			Severity: SeverityError,
			Message:  "The contract requires a Transactional Outbox pattern to prevent dual-write inconsistencies, but it was omitted.",
			Remediation: "Record outbound events in an 'outbox_events' table within the same DB transaction as business mutations, and dispatch via a worker.",
		})
	}

	// Bounded worker requirement check
	if contract.Queue.Enabled && proposal.HasBoundedWorkers != nil && !*proposal.HasBoundedWorkers {
		report.Violations = append(report.Violations, Violation{
			RuleID:   "VIOL-UNBOUNDED-WORKERS",
			Severity: SeverityError,
			Message:  "The contract requires bounded worker concurrency, but the proposal uses unbounded worker dispatching.",
			Remediation: "Enforce a worker pool or channel semaphore with max concurrency (e.g. max 10-25 workers) and backpressure.",
		})
	}
}

// inspectFile analyzes code files for dangerous architectural anti-patterns.
func (v *Validator) inspectFile(contract *model.ArchitectureContract, file CodeFileProposal, report *ValidationReport) {
	lowerContent := strings.ToLower(file.Content)
	lowerPath := strings.ToLower(file.Path)

	// Check 1: Synchronous external HTTP inside active DB transaction
	if (strings.Contains(lowerContent, "tx.begin") || strings.Contains(lowerContent, "begintx")) &&
		(strings.Contains(lowerContent, "http.get") || strings.Contains(lowerContent, "http.post") || strings.Contains(lowerContent, "client.do")) {
		report.Violations = append(report.Violations, Violation{
			RuleID:      "VIOL-SYNC-HTTP-IN-TX",
			Severity:    SeverityError,
			File:        file.Path,
			Message:     "Detected outbound HTTP call within active database transaction block.",
			Remediation: "Move external HTTP calls outside the transaction. Commit state changes to the local DB (with Outbox row) before making network calls.",
		})
	}

	// Check 2: Direct SQL queries in HTTP handler files
	if (strings.Contains(lowerPath, "handler") || strings.Contains(lowerPath, "controller")) &&
		(strings.Contains(lowerContent, "select ") || strings.Contains(lowerContent, "update ") || strings.Contains(lowerContent, "db.query") || strings.Contains(lowerContent, "db.exec")) {
		report.Violations = append(report.Violations, Violation{
			RuleID:      "VIOL-DB-IN-HANDLER",
			Severity:    SeverityWarning,
			File:        file.Path,
			Message:     "Direct database query executed inside HTTP handler layer.",
			Remediation: "Decouple persistence logic into a dedicated repository or store package. Handlers should only orchestrate domain services.",
		})
	}

	// Check 3: Queue polling without SKIP LOCKED
	if contract.Queue.QueueMechanism == "POSTGRES_SKIP_LOCKED" {
		if strings.Contains(lowerContent, "for update") && !strings.Contains(lowerContent, "skip locked") {
			report.Violations = append(report.Violations, Violation{
				RuleID:      "VIOL-QUEUE-MISSING-SKIP-LOCKED",
				Severity:    SeverityError,
				File:        file.Path,
				Message:     "Queue polling query uses 'FOR UPDATE' without 'SKIP LOCKED'.",
				Remediation: "Append 'SKIP LOCKED' to 'SELECT ... FOR UPDATE' to prevent worker thread lock contention and serialization bottlenecks.",
			})
		}
	}

	// Check 4: Unbounded goroutines in handlers
	if strings.Contains(lowerPath, "handler") && (strings.Contains(lowerContent, "go func(") || strings.Contains(lowerContent, "go handle")) {
		if !strings.Contains(lowerContent, "semaphore") && !strings.Contains(lowerContent, "workerpool") && !strings.Contains(lowerContent, "pool.") {
			report.Violations = append(report.Violations, Violation{
				RuleID:      "VIOL-UNBOUNDED-HANDLER-GOROUTINE",
				Severity:    SeverityWarning,
				File:        file.Path,
				Message:     "Unbounded goroutine spawned inside request handler without visible concurrency bounds.",
				Remediation: "Enqueue background work to a bounded worker queue instead of spawning raw goroutines per HTTP request.",
			})
		}
	}
}
