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
				Path: "internal/service/payment.go",
				Content: "tx.BeginTx\n" + "http.Post(\"http://internal/pay\")\n" + "tx.Commit\n",
			},
			{
				Path: "internal/worker/queue.go",
				Content: "db.Query(\"SELECT * FROM jobs FOR UPDATE\")\n",
			},
			{
				Path: "internal/handler/user.go",
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
