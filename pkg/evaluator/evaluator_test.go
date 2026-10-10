package evaluator

import (
	"strings"
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

func TestEvaluatorDecisionTracesDomainTailoring(t *testing.T) {
	eval := New(knowledge.NewCatalog())

	domains := []struct {
		name               string
		domain             model.DomainCategory
		disallowedKeywords []string
		expectedTraceTerms []string
	}{
		{
			name:               "E-Commerce Domain",
			domain:             model.DomainECommerce,
			disallowedKeywords: []string{"volunteer", "certificate", "delegate", "attendance"},
			expectedTraceTerms: []string{"order", "checkout"},
		},
		{
			name:               "FinTech Domain",
			domain:             model.DomainFinTech,
			disallowedKeywords: []string{"volunteer", "certificate", "delegate", "attendance"},
			expectedTraceTerms: []string{"ledger", "transfer", "double spending"},
		},
		{
			name:               "Generic CRUD Domain",
			domain:             model.DomainGeneralCRUD,
			disallowedKeywords: []string{"volunteer", "certificate", "delegate", "attendance"},
			expectedTraceTerms: []string{"duplicate", "unique"},
		},
		{
			name:               "Event Management Domain",
			domain:             model.DomainEventManagement,
			disallowedKeywords: []string{},
			expectedTraceTerms: []string{"attendance", "check-in"},
		},
	}

	for _, tt := range domains {
		t.Run(tt.name, func(t *testing.T) {
			req := &model.NormalizedRequirements{
				ProjectName: "App_" + string(tt.domain),
				Domain:      tt.domain,
				Traffic:     model.TrafficProfile{ExpectedRPS: 50, PeakRPS: 150},
				Data:        model.DataProfile{RequiresACID: true},
				Invariants: model.BusinessInvariants{
					PreventDuplicates:       true,
					RequiresIdempotency:     true,
					RequiresOfflineHandling: true,
					AsyncOperations:         []string{"notification_dispatch"},
				},
			}
			class := &model.ClassificationResult{
				ActiveTraits: []model.WorkloadTrait{model.TraitTransactionHeavy},
			}

			summary := eval.Evaluate(req, class)

			for _, trace := range summary.DecisionTraces {
				allTraceText := trace.Decision + " " + trace.EvidenceAndProvenance
				for _, reqText := range trace.SupportingRequirements {
					allTraceText += " " + reqText
				}
				for _, rej := range trace.RejectedReasons {
					allTraceText += " " + rej
				}

				// Verify disallowed domain keywords do not leak into this trace
				for _, kw := range tt.disallowedKeywords {
					lowerTrace := trace.Decision + " " + trace.EvidenceAndProvenance
					for _, r := range trace.SupportingRequirements {
						lowerTrace += " " + r
					}
					for _, r := range trace.RejectedReasons {
						lowerTrace += " " + r
					}
					if strings.Contains(strings.ToLower(lowerTrace), kw) {
						t.Errorf("domain %s decision trace %q leaked forbidden conference keyword %q: %s",
							tt.domain, trace.Decision, kw, lowerTrace)
					}
				}
			}
		})
	}
}
