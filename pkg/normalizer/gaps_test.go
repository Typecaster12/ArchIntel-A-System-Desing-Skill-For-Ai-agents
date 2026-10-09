package normalizer

import (
	"testing"

	"github.com/sdskill/archintel/pkg/model"
)

func TestGapsUnspecifiedTraffic(t *testing.T) {
	detector := NewGapDetector()

	norm := &model.NormalizedRequirements{
		ProjectName: "SimpleApp",
		Domain:      model.DomainGeneralCRUD,
		Traffic: model.TrafficProfile{
			IsExplicit: false,
		},
		Data: model.DataProfile{
			PrimaryStorage: "RELATIONAL",
		},
	}

	analysis := detector.Analyze(norm)

	if !analysis.CanProceed {
		t.Errorf("expected CanProceed to be true, gaps must never stall the pipeline")
	}
	if len(analysis.Gaps) == 0 {
		t.Fatalf("expected at least one gap for unstated traffic")
	}
	if analysis.Gaps[0].Field != "TRAFFIC_SCALE" {
		t.Errorf("expected gap field 'TRAFFIC_SCALE', got %q", analysis.Gaps[0].Field)
	}
	if len(analysis.Questions) == 0 {
		t.Errorf("expected clarification questions for unstated traffic")
	}
	if len(analysis.Assumptions) == 0 {
		t.Errorf("expected assumptions to be populated")
	}
}

func TestGapsCommerceConsistency(t *testing.T) {
	detector := NewGapDetector()

	norm := &model.NormalizedRequirements{
		ProjectName: "Store",
		Domain:      model.DomainECommerce,
		Consistency: model.ConsistencyEventual, // Eventual is risky for e-commerce
		Traffic: model.TrafficProfile{
			IsExplicit: true,
		},
		Data: model.DataProfile{
			HasHighContention: true,
		},
	}

	analysis := detector.Analyze(norm)

	foundConsistencyGap := false
	for _, gap := range analysis.Gaps {
		if gap.Field == "FINANCIAL_CONSISTENCY" {
			foundConsistencyGap = true
			if gap.Severity != ImpactHigh {
				t.Errorf("expected HIGH severity for financial consistency omission")
			}
		}
	}

	if !foundConsistencyGap {
		t.Errorf("expected FINANCIAL_CONSISTENCY gap to be flagged for e-commerce with eventual consistency")
	}
}
