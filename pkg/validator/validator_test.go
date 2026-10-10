package validator

import (
	"testing"

	"github.com/sdskill/archintel/pkg/model"
)

func TestValidatorDetectsViolations(t *testing.T) {
	v := New()

	contract := &model.ArchitectureContract{
		ContractVersion: "1.0.0",
		ProjectName:     "ShopApp",
		Data: model.DataRules{
			RequiresIdempotencyKey: true,
			RequiresOutboxPattern:  true,
		},
		Queue: model.QueueRules{
			Enabled:        true,
			QueueMechanism: "POSTGRES_SKIP_LOCKED",
		},
	}

	falseVal := false
	proposal := ImplementationProposal{
		HasIdempotencyCheck:     &falseVal, // Violates contract
		UsesTransactionalOutbox: &falseVal, // Violates contract
		Files: []CodeFileProposal{
			{
				Path:    "internal/service/payment.go",
				Content: "tx.BeginTx\n" + "http.Post(\"http://internal/pay\")\n" + "tx.Commit\n",
			},
			{
				Path:    "internal/worker/queue.go",
				Content: "db.Query(\"SELECT * FROM jobs FOR UPDATE\")\n",
			},
			{
				Path:    "internal/handler/user.go",
				Content: "db.Exec(\"UPDATE accounts SET active = true\")\n",
			},
		},
	}

	report := v.Validate(contract, proposal)

	if report.IsCompliant {
		t.Errorf("expected proposal to fail compliance")
	}
	if report.ErrorsCount < 4 {
		t.Errorf("expected at least 4 errors (idempotency, outbox, sync http in tx, missing skip locked), got %d", report.ErrorsCount)
	}
	if report.WarningsCount < 1 {
		t.Errorf("expected at least 1 warning for SQL in handler, got %d", report.WarningsCount)
	}
}

func TestValidatorCompliantProposal(t *testing.T) {
	v := New()

	contract := &model.ArchitectureContract{
		ContractVersion: "1.0.0",
		ProjectName:     "CompliantApp",
		Data: model.DataRules{
			RequiresIdempotencyKey: true,
		},
	}

	trueVal := true
	proposal := ImplementationProposal{
		HasIdempotencyCheck: &trueVal,
		Files: []CodeFileProposal{
			{
				Path: "internal/handler/checkout.go",
				Content: `
					// Clean handler delegating to service
					if err := service.ProcessOrder(ctx, req); err != nil {
						http.Error(w, err.Error(), 500)
					}
				`,
			},
		},
	}

	report := v.Validate(contract, proposal)

	if !report.IsCompliant {
		t.Errorf("expected proposal to be compliant, got errors: %v", report.Violations)
	}
	if report.ErrorsCount != 0 {
		t.Errorf("expected 0 errors, got %d", report.ErrorsCount)
	}
}

func TestValidator_NegativeCases(t *testing.T) {
	v := New()
	contract := &model.ArchitectureContract{
		ContractVersion: "1.0.0",
		ProjectName:     "ConferenceApp",
		Data: model.DataRules{
			RequiresIdempotencyKey: true,
			RequiresOutboxPattern:  true,
			UniquenessStrategy:     "UNIQUE(delegate_id, session_id)",
		},
		Queue: model.QueueRules{
			Enabled:        true,
			QueueMechanism: "POSTGRES_SKIP_LOCKED",
		},
	}

	tests := []struct {
		name         string
		file         CodeFileProposal
		expectedRule string
		expectedSev  Severity
	}{
		{
			name: "synchronous HTTP in database transaction",
			file: CodeFileProposal{
				Path: "internal/service/order.go",
				Content: `package service
func Process() {
	tx, _ := db.Begin()
	http.Post("http://payment.svc", "application/json", nil)
	tx.Commit()
}`,
			},
			expectedRule: "VIOL-SYNC-HTTP-IN-TX",
			expectedSev:  SeverityError,
		},
		{
			name: "direct SQL in handler layer",
			file: CodeFileProposal{
				Path: "internal/handler/delegate_handler.go",
				Content: `package handler
func Handle(w http.ResponseWriter, r *http.Request) {
	db.Query("SELECT * FROM delegates WHERE id = 1")
}`,
			},
			expectedRule: "VIOL-DB-IN-HANDLER",
			expectedSev:  SeverityWarning,
		},
		{
			name: "missing SKIP LOCKED in worker polling query",
			file: CodeFileProposal{
				Path: "internal/worker/outbox.go",
				Content: `package worker
func Poll() {
	db.Query("SELECT * FROM outbox_events WHERE status = 'PENDING' FOR UPDATE")
}`,
			},
			expectedRule: "VIOL-QUEUE-MISSING-SKIP-LOCKED",
			expectedSev:  SeverityError,
		},
		{
			name: "non-atomic attendance insert without ON CONFLICT",
			file: CodeFileProposal{
				Path: "internal/repo/attendance.go",
				Content: `package repo
func Record() {
	db.Exec("INSERT INTO attendance (delegate_id, session_id) VALUES ($1, $2)")
}`,
			},
			expectedRule: "VIOL-NON-ATOMIC-DUPLICATE-CHECK",
			expectedSev:  SeverityError,
		},
		{
			name: "mutating handler missing idempotency header extraction",
			file: CodeFileProposal{
				Path: "internal/handler/checkout_handler.go",
				Content: `package handler
func Checkout(w http.ResponseWriter, r *http.Request) {
	// Mutating payment route without deduplication token
	service.ExecutePayment(r.Context())
}`,
			},
			expectedRule: "VIOL-MISSING-HANDLER-IDEMPOTENCY",
			expectedSev:  SeverityWarning,
		},
		{
			name: "shallow idempotency check reading header without store/fingerprint",
			file: CodeFileProposal{
				Path: "internal/handler/payment_handler.go",
				Content: `package handler
func ProcessPayment(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		http.Error(w, "missing key", 400)
		return
	}
	executeCharge()
}`,
			},
			expectedRule: "VIOL-SHALLOW-IDEMPOTENCY-CHECK",
			expectedSev:  SeverityWarning,
		},
		{
			name: "unbounded goroutine spawned inside HTTP handler",
			file: CodeFileProposal{
				Path: "internal/handler/notify_handler.go",
				Content: `package handler
func Notify(w http.ResponseWriter, r *http.Request) {
	go func() {
		sendNotification()
	}()
}`,
			},
			expectedRule: "VIOL-AST-UNBOUNDED-GOROUTINE",
			expectedSev:  SeverityWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop := ImplementationProposal{
				Files: []CodeFileProposal{tt.file},
			}
			report := v.Validate(contract, prop)
			found := false
			for _, viol := range report.Violations {
				if viol.RuleID == tt.expectedRule && viol.Severity == tt.expectedSev {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected violation %s (%s), but got violations: %+v", tt.expectedRule, tt.expectedSev, report.Violations)
			}
		})
	}
}

func TestValidator_PositiveCompliantArchitecture(t *testing.T) {
	v := New()
	contract := &model.ArchitectureContract{
		ContractVersion: "1.0.0",
		ProjectName:     "ConferenceApp",
		Data: model.DataRules{
			RequiresIdempotencyKey: true,
			RequiresOutboxPattern:  true,
			UniquenessStrategy:     "UNIQUE(delegate_id, session_id)",
		},
		Queue: model.QueueRules{
			Enabled:        true,
			QueueMechanism: "POSTGRES_SKIP_LOCKED",
		},
	}

	hasIdem := true
	hasOutbox := true
	hasWorkers := true

	compliantProposal := ImplementationProposal{
		HasIdempotencyCheck:     &hasIdem,
		UsesTransactionalOutbox: &hasOutbox,
		HasBoundedWorkers:       &hasWorkers,
		Files: []CodeFileProposal{
			{
				Path: "internal/handler/checkin_handler.go",
				Content: `package handler
func CheckInHandler(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if cached, ok := idempotencyStore.Get(key); ok {
		w.Write(cached)
		return
	}
	service.RecordCheckIn(r.Context(), key)
}`,
			},
			{
				Path: "internal/repo/attendance_repo.go",
				Content: `package repo
func RecordAttendance(ctx context.Context, delegateID, sessionID string) error {
	_, err := db.ExecContext(ctx, "INSERT INTO attendance (delegate_id, session_id) VALUES ($1, $2) ON CONFLICT (delegate_id, session_id) DO NOTHING")
	return err
}`,
			},
			{
				Path: "internal/worker/outbox_worker.go",
				Content: `package worker
func PollOutbox(ctx context.Context) {
	db.QueryContext(ctx, "SELECT id, payload FROM outbox_events WHERE status = 'PENDING' FOR UPDATE SKIP LOCKED LIMIT 10")
}`,
			},
		},
	}

	report := v.Validate(contract, compliantProposal)
	if !report.IsCompliant {
		t.Fatalf("expected compliant proposal, but got errors: %+v", report.Violations)
	}
	if report.ErrorsCount != 0 {
		t.Errorf("expected 0 errors, got %d", report.ErrorsCount)
	}
}

func TestValidator_HardenedASTAndFalsePositiveResistance(t *testing.T) {
	v := New()
	contract := &model.ArchitectureContract{
		ContractVersion: "1.0.0",
		ProjectName:     "HardenedApp",
		Data: model.DataRules{
			RequiresIdempotencyKey: true,
			RequiresOutboxPattern:  true,
			UniquenessStrategy:     "UNIQUE(order_id, user_id)",
		},
		Queue: model.QueueRules{
			Enabled:        true,
			QueueMechanism: "POSTGRES_SKIP_LOCKED",
		},
	}

	t.Run("false positive resistance: separate functions in same file do not trigger sync http in tx", func(t *testing.T) {
		prop := ImplementationProposal{
			Files: []CodeFileProposal{
				{
					Path: "internal/service/account.go",
					Content: `package service
func UpdateBalance() {
	tx, _ := db.Begin()
	tx.Commit()
}
func FetchExchangeRate() {
	http.Get("https://api.rates.com")
}`,
				},
			},
		}
		rep := v.Validate(contract, prop)
		for _, viol := range rep.Violations {
			if viol.RuleID == "VIOL-SYNC-HTTP-IN-TX" {
				t.Fatalf("unexpected false positive VIOL-SYNC-HTTP-IN-TX across separate functions")
			}
		}
	})

	t.Run("false positive resistance: comment containing http.get and tx.begin does not trigger violation", func(t *testing.T) {
		prop := ImplementationProposal{
			Files: []CodeFileProposal{
				{
					Path: "internal/service/clean.go",
					Content: `package service
// Note: never invoke http.Get inside tx.Begin block
func CleanLogic() {
	log.Println("performing safe operation")
}`,
				},
			},
		}
		rep := v.Validate(contract, prop)
		for _, viol := range rep.Violations {
			if viol.RuleID == "VIOL-SYNC-HTTP-IN-TX" {
				t.Fatalf("unexpected false positive VIOL-SYNC-HTTP-IN-TX triggered by comment")
			}
		}
	})

	t.Run("positive detection: helper function inside transaction making outbound HTTP call is caught", func(t *testing.T) {
		prop := ImplementationProposal{
			Files: []CodeFileProposal{
				{
					Path: "internal/service/checkout.go",
					Content: `package service
func callStripeAPI() {
	http.Post("https://stripe.com/api", "application/json", nil)
}
func ProcessCheckout() {
	tx, _ := db.Begin()
	callStripeAPI()
	tx.Commit()
}`,
				},
			},
		}
		rep := v.Validate(contract, prop)
		found := false
		for _, viol := range rep.Violations {
			if viol.RuleID == "VIOL-SYNC-HTTP-IN-TX" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected VIOL-SYNC-HTTP-IN-TX for helper call inside transaction")
		}
	})

	t.Run("bypass resistance: comment containing workerpool does not bypass unbounded goroutine check", func(t *testing.T) {
		prop := ImplementationProposal{
			Files: []CodeFileProposal{
				{
					Path: "internal/handler/order_handler.go",
					Content: `package handler
func HandleOrder(w http.ResponseWriter, r *http.Request) {
	// workerpool comment attempt to bypass check
	go func() {
		processAsync()
	}()
}`,
				},
			},
		}
		rep := v.Validate(contract, prop)
		found := false
		for _, viol := range rep.Violations {
			if viol.RuleID == "VIOL-AST-UNBOUNDED-GOROUTINE" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected VIOL-AST-UNBOUNDED-GOROUTINE despite misleading comment")
		}
	})

	t.Run("bounded concurrency: semaphore channel in handler is permitted", func(t *testing.T) {
		prop := ImplementationProposal{
			Files: []CodeFileProposal{
				{
					Path: "internal/handler/bounded_handler.go",
					Content: `package handler
var sem = make(chan struct{}, 10)
func HandleBounded(w http.ResponseWriter, r *http.Request) {
	sem <- struct{}{}
	go func() {
		defer func() { <-sem }()
		processAsync()
	}()
}`,
				},
			},
		}
		rep := v.Validate(contract, prop)
		for _, viol := range rep.Violations {
			if viol.RuleID == "VIOL-AST-UNBOUNDED-GOROUTINE" {
				t.Fatalf("unexpected violation: semaphore-bounded goroutine should be permitted")
			}
		}
	})

	t.Run("nonstandard path: api package handler caught for direct SQL and idempotency", func(t *testing.T) {
		prop := ImplementationProposal{
			Files: []CodeFileProposal{
				{
					Path: "pkg/api/payments.go",
					Content: `package api
func ChargePayment(w http.ResponseWriter, r *http.Request) {
	db.Query("SELECT * FROM payments WHERE id = 1")
}`,
				},
			},
		}
		rep := v.Validate(contract, prop)
		foundSQL := false
		foundIdemp := false
		for _, viol := range rep.Violations {
			if viol.RuleID == "VIOL-DB-IN-HANDLER" {
				foundSQL = true
			}
			if viol.RuleID == "VIOL-MISSING-HANDLER-IDEMPOTENCY" {
				foundIdemp = true
			}
		}
		if !foundSQL {
			t.Fatalf("expected VIOL-DB-IN-HANDLER in nonstandard path pkg/api")
		}
		if !foundIdemp {
			t.Fatalf("expected VIOL-MISSING-HANDLER-IDEMPOTENCY in mutating endpoint in pkg/api")
		}
	})

	t.Run("generalized table: non-conference table caught for missing ON CONFLICT", func(t *testing.T) {
		prop := ImplementationProposal{
			Files: []CodeFileProposal{
				{
					Path: "internal/repo/order_repo.go",
					Content: `package repo
func CreateOrder() {
	db.Exec("INSERT INTO orders (order_id, user_id) VALUES ($1, $2)")
}`,
				},
			},
		}
		rep := v.Validate(contract, prop)
		found := false
		for _, viol := range rep.Violations {
			if viol.RuleID == "VIOL-NON-ATOMIC-DUPLICATE-CHECK" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected VIOL-NON-ATOMIC-DUPLICATE-CHECK on generalized orders table")
		}
	})
}
