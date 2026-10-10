package validator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"

	"github.com/sdskill/archintel/pkg/model"
)

// StaticAnalysisLimitations documents known boundaries of static AST and lexical inspection:
// 1. Lexical and AST checks inspect syntactic patterns (e.g. calls to http.Get inside functions with tx.Begin).
//    They cannot observe runtime execution order across asynchronous goroutines, dynamic interfaces, or reflection.
// 2. Static analysis cannot evaluate runtime database transaction isolation, dynamic lock timeouts, or network latency.
// 3. Static checks verify structural presence of idempotency stores and unique constraints, but cannot verify
//    runtime data consistency without empirical database integration and concurrency tests.

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
		Limitations: make([]string, 0),
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
			RuleID:      "VIOL-MISSING-IDEMPOTENCY",
			Severity:    SeverityError,
			Message:     "The contract mandates idempotency keys on mutating routes, but the proposal omits idempotency checking.",
			Remediation: "Implement an Idempotency-Key validation middleware checking request tokens against a dedicated idempotency table.",
		})
	}

	// Transactional Outbox requirement check
	if contract.Data.RequiresOutboxPattern && proposal.UsesTransactionalOutbox != nil && !*proposal.UsesTransactionalOutbox {
		report.Violations = append(report.Violations, Violation{
			RuleID:      "VIOL-MISSING-OUTBOX",
			Severity:    SeverityError,
			Message:     "The contract requires a Transactional Outbox pattern to prevent dual-write inconsistencies, but it was omitted.",
			Remediation: "Record outbound events in an 'outbox_events' table within the same DB transaction as business mutations, and dispatch via a worker.",
		})
	}

	// Bounded worker requirement check
	if contract.Queue.Enabled && proposal.HasBoundedWorkers != nil && !*proposal.HasBoundedWorkers {
		report.Violations = append(report.Violations, Violation{
			RuleID:      "VIOL-UNBOUNDED-WORKERS",
			Severity:    SeverityError,
			Message:     "The contract requires bounded worker concurrency, but the proposal uses unbounded worker dispatching.",
			Remediation: "Enforce a worker pool or channel semaphore with max concurrency (e.g. max 10-25 workers) and backpressure.",
		})
	}
}

var (
	commentSingleRe = regexp.MustCompile(`//[^\n]*`)
	commentMultiRe  = regexp.MustCompile(`/\*[\s\S]*?\*/`)
	insertQueryRe   = regexp.MustCompile(`(?i)insert\s+into\s+([a-zA-Z0-9_]+)`)
)

// stripComments removes Go line and block comments while preserving line count.
func stripComments(content string) string {
	cleaned := commentMultiRe.ReplaceAllStringFunc(content, func(m string) string {
		return strings.Repeat("\n", strings.Count(m, "\n"))
	})
	return commentSingleRe.ReplaceAllString(cleaned, "")
}

// inspectFile analyzes code files for architectural anti-patterns with AST and hardened lexical checks.
func (v *Validator) inspectFile(contract *model.ArchitectureContract, file CodeFileProposal, report *ValidationReport) {
	cleanContent := stripComments(file.Content)
	lowerClean := strings.ToLower(cleanContent)
	lowerPath := strings.ToLower(file.Path)

	astSuccess := v.inspectFileWithAST(contract, file, cleanContent, report)

	// If AST parsing could not analyze Go syntax (e.g. raw code snippets), fall back to hardened lexical checks
	if !astSuccess {
		v.inspectLexicalFallback(contract, file, lowerClean, report)
	}

	// Check 3: Queue polling without SKIP LOCKED
	if contract.Queue.QueueMechanism == "POSTGRES_SKIP_LOCKED" {
		if strings.Contains(lowerClean, "for update") && !strings.Contains(lowerClean, "skip locked") {
			report.Violations = append(report.Violations, Violation{
				RuleID:      "VIOL-QUEUE-MISSING-SKIP-LOCKED",
				Severity:    SeverityError,
				File:        file.Path,
				Message:     "Queue polling query uses 'FOR UPDATE' without 'SKIP LOCKED'.",
				Remediation: "Append 'SKIP LOCKED' to 'SELECT ... FOR UPDATE' to prevent worker thread lock contention and serialization bottlenecks.",
			})
		}
	}

	// Check 5: Generalized non-atomic duplicate check
	if contract.Data.UniquenessStrategy != "" {
		if matches := insertQueryRe.FindAllStringSubmatch(cleanContent, -1); len(matches) > 0 {
			if !strings.Contains(lowerClean, "on conflict") && !strings.Contains(lowerClean, "for update") {
				for _, match := range matches {
					tbl := match[1]
					msg := "Attendance or registration INSERT query lacks atomic 'ON CONFLICT' duplicate handling."
					if tbl != "attendance" && tbl != "registration" {
						msg = fmt.Sprintf("Database INSERT query on table %q lacks atomic 'ON CONFLICT' duplicate handling.", tbl)
					}
					report.Violations = append(report.Violations, Violation{
						RuleID:      "VIOL-NON-ATOMIC-DUPLICATE-CHECK",
						Severity:    SeverityError,
						File:        file.Path,
						Message:     msg,
						Remediation: fmt.Sprintf("Use database-level unique constraint with atomic conflict handling: 'INSERT INTO %s (...) VALUES (...) ON CONFLICT (...) DO NOTHING'.", tbl),
					})
					break // Report once per file
				}
			}
		}
	}

	// Check 6: Missing or Shallow Idempotency Key validation on mutating HTTP routes
	isMutatingPathOrContent := isRequestHandlerPath(file.Path) ||
		strings.Contains(lowerClean, "checkin") || strings.Contains(lowerClean, "checkout") ||
		strings.Contains(lowerClean, "register") || strings.Contains(lowerClean, "payment") || strings.Contains(lowerClean, "order")

	if contract.Data.RequiresIdempotencyKey && isMutatingPathOrContent {
		isMutating := strings.Contains(lowerClean, "checkin") ||
			strings.Contains(lowerClean, "checkout") ||
			strings.Contains(lowerClean, "register") ||
			strings.Contains(lowerClean, "payment") ||
			strings.Contains(lowerClean, "order") ||
			strings.Contains(lowerPath, "checkout") ||
			strings.Contains(lowerPath, "payment") ||
			strings.Contains(lowerPath, "checkin")

		if isMutating {
			hasHeader := strings.Contains(lowerClean, "idempotency-key") ||
				strings.Contains(lowerClean, "x-idempotency-key") ||
				strings.Contains(lowerClean, "header.get(\"idempotency") ||
				strings.Contains(lowerClean, "header.get(\"x-idempotency")

			hasStoreOrFingerprint := strings.Contains(lowerClean, "store") ||
				strings.Contains(lowerClean, "fingerprint") ||
				strings.Contains(lowerClean, "sha256") ||
				strings.Contains(lowerClean, "replay") ||
				strings.Contains(lowerClean, "repo") ||
				strings.Contains(lowerClean, "cached") ||
				strings.Contains(lowerClean, "idempotency_keys")

			if !hasHeader {
				report.Violations = append(report.Violations, Violation{
					RuleID:      "VIOL-MISSING-HANDLER-IDEMPOTENCY",
					Severity:    SeverityWarning,
					File:        file.Path,
					Message:     "Mutating handler route does not extract or validate 'Idempotency-Key' header.",
					Remediation: "Inspect 'Idempotency-Key' request header and verify against processed request store before executing mutations.",
				})
			} else if !hasStoreOrFingerprint {
				report.Violations = append(report.Violations, Violation{
					RuleID:      "VIOL-SHALLOW-IDEMPOTENCY-CHECK",
					Severity:    SeverityWarning,
					File:        file.Path,
					Message:     "Mutating handler checks for Idempotency-Key header but lacks verification against a persistent store or payload fingerprint. Checking for header presence alone is insufficient.",
					Remediation: "Persist idempotency keys with payload SHA-256 fingerprint, lock concurrent requests (HTTP 409), and replay cached responses.",
				})
			}
		}
	}
}

// inspectFileWithAST analyzes structured Go AST nodes.
func (v *Validator) inspectFileWithAST(contract *model.ArchitectureContract, file CodeFileProposal, cleanContent string, report *ValidationReport) bool {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file.Path, cleanContent, 0)
	if err != nil {
		// Try wrapping snippet into valid Go package and function
		wrapped := "package main\nfunc _snippetWrapper() {\n" + cleanContent + "\n}"
		node, err = parser.ParseFile(fset, file.Path, wrapped, 0)
		if err != nil {
			return false
		}
	}

	// 1. Map in-file helper functions that execute outbound HTTP calls
	helperHasHTTP := make(map[string]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			if isHTTPCall(inner) {
				helperHasHTTP[fn.Name.Name] = true
				return false
			}
			return true
		})
		return true
	})

	// 2. Inspect function declarations and function literals
	isHandlerPath := isRequestHandlerPath(file.Path)

	ast.Inspect(node, func(n ast.Node) bool {
		var body *ast.BlockStmt
		var isHandler bool
		var funcName string

		switch fn := n.(type) {
		case *ast.FuncDecl:
			body = fn.Body
			funcName = fn.Name.Name
			isHandler = isHandlerPath || isHTTPHandlerSignature(fn.Type)
		case *ast.FuncLit:
			body = fn.Body
			funcName = "anonymous_function"
			isHandler = isHandlerPath || isHTTPHandlerSignature(fn.Type)
		}

		if body == nil {
			return true
		}

		// A. Scope-based Check 1: Synchronous HTTP inside DB transaction
		hasTx := false
		hasDirectHTTP := false
		calledHTTPHelper := ""
		hasUnresolvedCall := false

		ast.Inspect(body, func(bn ast.Node) bool {
			if isTxBeginCall(bn) {
				hasTx = true
			}
			if isHTTPCall(bn) {
				hasDirectHTTP = true
			}
			if call, ok := bn.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok {
					if helperHasHTTP[id.Name] {
						calledHTTPHelper = id.Name
					}
				} else if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					pkgName := fmt.Sprintf("%s.%s", exprToString(sel.X), sel.Sel.Name)
					if !strings.HasPrefix(pkgName, "http.") && !strings.HasPrefix(pkgName, "tx.") && !strings.HasPrefix(pkgName, "db.") {
						hasUnresolvedCall = true
					}
				}
			}
			return true
		})

		if hasTx && (hasDirectHTTP || calledHTTPHelper != "") {
			detail := "Detected outbound HTTP call within active database transaction block."
			if calledHTTPHelper != "" {
				detail = fmt.Sprintf("Detected outbound HTTP call within active database transaction block via helper function %q.", calledHTTPHelper)
			}
			report.Violations = append(report.Violations, Violation{
				RuleID:      "VIOL-SYNC-HTTP-IN-TX",
				Severity:    SeverityError,
				File:        file.Path,
				Message:     detail,
				Remediation: "Move external HTTP calls outside the transaction. Commit state changes to the local DB (with Outbox row) before making network calls.",
			})
		} else if hasTx && hasUnresolvedCall {
			report.Limitations = append(report.Limitations, fmt.Sprintf(
				"%s: Function %q begins a DB transaction and calls external helper; interprocedural analysis cannot prove external network call safety statically",
				file.Path, funcName,
			))
		}

		// B. Check 2: Direct SQL in HTTP handler
		if isHandler {
			hasDirectSQL := false
			ast.Inspect(body, func(bn ast.Node) bool {
				if isDirectDBCall(bn) {
					hasDirectSQL = true
					return false
				}
				return true
			})
			if hasDirectSQL {
				report.Violations = append(report.Violations, Violation{
					RuleID:      "VIOL-DB-IN-HANDLER",
					Severity:    SeverityWarning,
					File:        file.Path,
					Message:     "Direct database query executed inside HTTP handler layer.",
					Remediation: "Decouple persistence logic into a dedicated repository or store package. Handlers should only orchestrate domain services.",
				})
			}
		}

		// C. Check 4: Unbounded goroutine in HTTP handler
		if isHandler {
			ast.Inspect(body, func(bn ast.Node) bool {
				goStmt, ok := bn.(*ast.GoStmt)
				if !ok {
					return true
				}
				if !isBoundedGoroutine(body, goStmt) {
					report.Violations = append(report.Violations, Violation{
						RuleID:      "VIOL-AST-UNBOUNDED-GOROUTINE",
						Severity:    SeverityWarning,
						File:        file.Path,
						Message:     "Go AST analysis detected direct 'go' goroutine dispatch in HTTP handler layer.",
						Remediation: "Pass background tasks to a bounded worker pool or channel queue to prevent memory exhaustion.",
					})
				}
				return true
			})
		}

		return true
	})

	return true
}

// inspectLexicalFallback runs when Go AST parsing cannot succeed.
func (v *Validator) inspectLexicalFallback(contract *model.ArchitectureContract, file CodeFileProposal, lowerClean string, report *ValidationReport) {
	// Check 1: Synchronous external HTTP inside active DB transaction
	if (strings.Contains(lowerClean, "tx.begin") || strings.Contains(lowerClean, "begintx") || strings.Contains(lowerClean, "db.begin") || strings.Contains(lowerClean, ".begin()")) &&
		(strings.Contains(lowerClean, "http.get") || strings.Contains(lowerClean, "http.post") || strings.Contains(lowerClean, "client.do")) {
		report.Violations = append(report.Violations, Violation{
			RuleID:      "VIOL-SYNC-HTTP-IN-TX",
			Severity:    SeverityError,
			File:        file.Path,
			Message:     "Detected outbound HTTP call within active database transaction block.",
			Remediation: "Move external HTTP calls outside the transaction. Commit state changes to the local DB (with Outbox row) before making network calls.",
		})
	}

	// Check 2: Direct SQL queries in handler layer
	if isRequestHandlerPath(file.Path) &&
		(strings.Contains(lowerClean, "select ") || strings.Contains(lowerClean, "update ") || strings.Contains(lowerClean, "db.query") || strings.Contains(lowerClean, "db.exec")) {
		report.Violations = append(report.Violations, Violation{
			RuleID:      "VIOL-DB-IN-HANDLER",
			Severity:    SeverityWarning,
			File:        file.Path,
			Message:     "Direct database query executed inside HTTP handler layer.",
			Remediation: "Decouple persistence logic into a dedicated repository or store package. Handlers should only orchestrate domain services.",
		})
	}

	// Check 4: Unbounded goroutine fallback
	if isRequestHandlerPath(file.Path) && (strings.Contains(lowerClean, "go func(") || strings.Contains(lowerClean, "go handle")) {
		if !strings.Contains(lowerClean, "semaphore") && !strings.Contains(lowerClean, "<-sem") && !strings.Contains(lowerClean, "pool.submit") {
			report.Violations = append(report.Violations, Violation{
				RuleID:      "VIOL-AST-UNBOUNDED-GOROUTINE",
				Severity:    SeverityWarning,
				File:        file.Path,
				Message:     "Unbounded goroutine spawned inside request handler without visible concurrency bounds.",
				Remediation: "Enqueue background work to a bounded worker queue instead of spawning raw goroutines per HTTP request.",
			})
		}
	}
}

func isTxBeginCall(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	method := strings.ToLower(sel.Sel.Name)
	return method == "begin" || method == "begintx"
}

func isHTTPCall(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg := strings.ToLower(exprToString(sel.X))
	method := strings.ToLower(sel.Sel.Name)
	if pkg == "http" && (method == "get" || method == "post" || method == "head" || method == "postform") {
		return true
	}
	if (pkg == "client" || strings.Contains(pkg, "http")) && method == "do" {
		return true
	}
	return false
}

func isDirectDBCall(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	target := strings.ToLower(exprToString(sel.X))
	method := strings.ToLower(sel.Sel.Name)
	if (target == "db" || target == "database") && (method == "query" || method == "queryrow" || method == "exec" || method == "querycontext" || method == "execcontext") {
		return true
	}
	return false
}

func isRequestHandlerPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "handler") || strings.Contains(lower, "controller") ||
		strings.Contains(lower, "api") || strings.Contains(lower, "routes") ||
		strings.Contains(lower, "endpoint") || strings.Contains(lower, "web")
}

func isHTTPHandlerSignature(ft *ast.FuncType) bool {
	if ft == nil || ft.Params == nil {
		return false
	}
	for _, field := range ft.Params.List {
		typeStr := strings.ToLower(exprToString(field.Type))
		if strings.Contains(typeStr, "responsewriter") || strings.Contains(typeStr, "request") ||
			strings.Contains(typeStr, "gin.context") || strings.Contains(typeStr, "echo.context") {
			return true
		}
	}
	return false
}

func isBoundedGoroutine(block *ast.BlockStmt, goStmt *ast.GoStmt) bool {
	hasBounds := false
	ast.Inspect(block, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SendStmt:
			hasBounds = true
		case *ast.UnaryExpr:
			if node.Op == token.ARROW {
				hasBounds = true
			}
		case *ast.CallExpr:
			fnName := strings.ToLower(exprToString(node.Fun))
			if strings.Contains(fnName, "pool.") || strings.Contains(fnName, "submit") || strings.Contains(fnName, "acquire") {
				hasBounds = true
			}
		}
		return true
	})
	return hasBounds
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	default:
		return ""
	}
}
