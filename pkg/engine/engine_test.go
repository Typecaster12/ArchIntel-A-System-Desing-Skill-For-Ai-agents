package engine

import (
	"context"
	"testing"

	"github.com/sdskill/archintel/pkg/model"
	"github.com/sdskill/archintel/pkg/validator"
)

func TestEngineFullPipeline(t *testing.T) {
	eng := NewEngine(EngineOptions{})
	ctx := context.Background()

	raw := model.RawRequirement{
		Prompt: "Build an e-commerce checkout service handling 250 RPS with flash sale inventory reservation.",
	}

	// 1. Test AnalyzeRequirements
	analysis, err := eng.AnalyzeRequirements(raw)
	if err != nil {
		t.Fatalf("AnalyzeRequirements failed: %v", err)
	}
	if analysis.Normalized.Domain != model.DomainECommerce {
		t.Errorf("expected Domain ECommerce, got %v", analysis.Normalized.Domain)
	}

	// 2. Test ClassifyWorkload
	class, err := eng.ClassifyWorkload(ctx, raw)
	if err != nil {
		t.Fatalf("ClassifyWorkload failed: %v", err)
	}
	if !class.HasTrait(model.TraitTransactionHeavy) {
		t.Errorf("expected TraitTransactionHeavy to be active")
	}

	// 3. Test RecommendArchitecture
	rec, err := eng.RecommendArchitecture(ctx, raw)
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}
	if rec.Contract == nil {
		t.Fatalf("expected non-nil ArchitectureContract")
	}
	if rec.Contract.Topology.Type != model.TopologyModularMonolith {
		t.Errorf("expected TopologyModularMonolith, got %v", rec.Contract.Topology.Type)
	}
	if rec.Contract.Data.Locking != model.LockingPessimistic {
		t.Errorf("expected LockingPessimistic for flash sale inventory, got %v", rec.Contract.Data.Locking)
	}
	if !rec.Contract.Data.RequiresIdempotencyKey {
		t.Errorf("expected RequiresIdempotencyKey to be true")
	}
	if len(rec.RelevantAntiPatterns) == 0 {
		t.Errorf("expected relevant anti-patterns to be included")
	}

	// 4. Test EvaluateTradeoffs
	tradeoffs, err := eng.EvaluateTradeoffs(ctx, raw)
	if err != nil {
		t.Fatalf("EvaluateTradeoffs failed: %v", err)
	}
	if len(tradeoffs.RejectedAlternatives) == 0 {
		t.Errorf("expected rejected alternatives to be populated")
	}

	// 5. Test ValidateArchitecture
	badVal := false
	report := eng.ValidateArchitecture(rec.Contract, validator.ImplementationProposal{
		HasIdempotencyCheck: &badVal,
	})
	if report.IsCompliant {
		t.Errorf("expected compliance to fail when idempotency is omitted")
	}
}
