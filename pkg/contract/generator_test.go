package contract

import (
	"testing"

	"github.com/sdskill/archintel/pkg/evaluator"
	"github.com/sdskill/archintel/pkg/model"
)

func TestContractGenerationSuccess(t *testing.T) {
	gen := New()

	req := &model.NormalizedRequirements{
		ProjectName: "PaymentGateway",
		Domain:      model.DomainFinTech,
		Traffic: model.TrafficProfile{
			ExpectedRPS: 150,
			PeakRPS:     400,
		},
		Data: model.DataProfile{
			PrimaryStorage: "RELATIONAL",
			RequiresACID:   true,
		},
		Assumptions: []string{"Defaulted to single-region PostgreSQL"},
	}

	class := &model.ClassificationResult{
		ActiveTraits: []model.WorkloadTrait{
			model.TraitTransactionHeavy,
			model.TraitConsistencySensitive,
		},
	}

	summary := &evaluator.DecisionSummary{
		RecommendedTopology: model.TopologyModularMonolith,
		TopologyRationale:   "Modular monolith ensures transactional integrity across ledger entries",
		RecommendedDataRules: model.DataRules{
			IsolationLevel:         "READ_COMMITTED",
			Locking:                model.LockingPessimistic,
			RequiresIdempotencyKey: true,
			RequiresOutboxPattern:  true,
			ConnectionPoolMax:      35,
			IdempotencyPolicy: model.IdempotencyPolicy{
				Required:              true,
				KeyScope:              "HEADER_IDEMPOTENCY_KEY_PER_OPERATION",
				FingerprintMechanism:  "SHA-256 hash of request body",
				StateStorageMechanism: "PostgreSQL idempotency_keys table",
			},
		},
		RecommendedQueueRules: model.QueueRules{
			Enabled:              true,
			QueueMechanism:       "POSTGRES_SKIP_LOCKED",
			MaxWorkerConcurrency: 10,
			MaxRetryAttempts:     3,
			BackoffStrategy:      "EXPONENTIAL_WITH_JITTER",
			DeadLetterHandling:   "Quarantine in failed_jobs table",
		},
	}

	contract, err := gen.Generate(req, class, summary)
	if err != nil {
		t.Fatalf("expected successful contract generation, got error: %v", err)
	}

	if contract.ProjectName != "PaymentGateway" {
		t.Errorf("expected ProjectName 'PaymentGateway', got %q", contract.ProjectName)
	}
	if len(contract.Components) != 2 {
		t.Errorf("expected 2 components (Core + OutboxDispatcherWorker), got %d", len(contract.Components))
	}
	if len(contract.Constraints) < 4 {
		t.Errorf("expected at least 4 constraints, got %d", len(contract.Constraints))
	}
	if len(contract.ValidationCriteria) < 3 {
		t.Errorf("expected at least 3 validation criteria, got %d", len(contract.ValidationCriteria))
	}
	if len(contract.RejectedAlternatives) == 0 {
		t.Errorf("expected rejected alternatives to be populated")
	}

	// Double check that schema validation passed
	if err := contract.Validate(); err != nil {
		t.Errorf("expected contract to pass Validate(), got: %v", err)
	}
}
