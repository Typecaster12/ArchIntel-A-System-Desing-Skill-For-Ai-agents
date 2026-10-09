package evaluator

import (
	"testing"

	"github.com/sdskill/archintel/pkg/knowledge"
	"github.com/sdskill/archintel/pkg/model"
)

func TestEvaluatorEcommerceDecision(t *testing.T) {
	eval := New(knowledge.NewCatalog())

	req := &model.NormalizedRequirements{
		ProjectName: "CheckoutApp",
		Domain:      model.DomainECommerce,
		Traffic: model.TrafficProfile{
			ExpectedRPS: 200,
			PeakRPS:     600,
			IsBursty:    true,
		},
		Data: model.DataProfile{
			PrimaryStorage:    "RELATIONAL",
			RequiresACID:      true,
			HasHighContention: true,
		},
		Consistency: model.ConsistencyStrong,
	}

	class := &model.ClassificationResult{
		ActiveTraits: []model.WorkloadTrait{
			model.TraitTransactionHeavy,
			model.TraitConsistencySensitive,
			model.TraitHighContention,
			model.TraitBurstyTraffic,
		},
		Scores: map[model.WorkloadTrait]model.ClassificationScore{
			model.TraitHighContention: {IsActive: true},
		},
	}

	summary := eval.Evaluate(req, class)

	if summary.RecommendedTopology != model.TopologyModularMonolith {
		t.Errorf("expected TopologyModularMonolith, got %v", summary.RecommendedTopology)
	}
	if summary.RecommendedDataRules.Locking != model.LockingPessimistic {
		t.Errorf("expected LockingPessimistic for high contention e-commerce, got %v", summary.RecommendedDataRules.Locking)
	}
	if !summary.RecommendedDataRules.RequiresIdempotencyKey {
		t.Errorf("expected RequiresIdempotencyKey to be true")
	}
	if !summary.RecommendedDataRules.RequiresOutboxPattern {
		t.Errorf("expected RequiresOutboxPattern to be true")
	}
	if !summary.RecommendedQueueRules.Enabled {
		t.Errorf("expected QueueRules to be enabled")
	}
	if summary.RecommendedQueueRules.QueueMechanism != "POSTGRES_SKIP_LOCKED" {
		t.Errorf("expected POSTGRES_SKIP_LOCKED for < 1000 RPS, got %v", summary.RecommendedQueueRules.QueueMechanism)
	}
}

func TestEvaluatorTelemetryPipelineDecision(t *testing.T) {
	eval := New(knowledge.NewCatalog())

	req := &model.NormalizedRequirements{
		ProjectName: "TelemetryIngest",
		Domain:      model.DomainIoTTelemetry,
		Traffic: model.TrafficProfile{
			ExpectedRPS:    2500,
			PeakRPS:        6000,
			ReadWriteRatio: 0.1, // Write heavy
		},
		Data: model.DataProfile{
			PrimaryStorage: "TIMESERIES",
		},
	}

	class := &model.ClassificationResult{
		ActiveTraits: []model.WorkloadTrait{
			model.TraitWriteHeavy,
		},
	}

	summary := eval.Evaluate(req, class)

	if summary.RecommendedTopology != model.TopologyPipeline {
		t.Errorf("expected TopologyPipeline for IoT write-heavy stream, got %v", summary.RecommendedTopology)
	}
}
