package model

import (
	"encoding/json"
	"testing"
)

func TestClassificationJSONSerialization(t *testing.T) {
	result := ClassificationResult{
		Source:             ClassifierSourceRuleBased,
		ModelVersion:       "v1.0.0-heuristic",
		InferenceLatencyMs: 12,
		Scores: map[WorkloadTrait]ClassificationScore{
			TraitTransactionHeavy: {
				Trait:     TraitTransactionHeavy,
				Score:     0.95,
				IsActive:  true,
				Rationale: "Payment processing with ACID database requirements",
			},
			TraitReadHeavy: {
				Trait:     TraitReadHeavy,
				Score:     0.30,
				IsActive:  false,
				Rationale: "Read:write ratio is balanced (1:1)",
			},
			TraitConsistencySensitive: {
				Trait:     TraitConsistencySensitive,
				Score:     0.90,
				IsActive:  true,
				Rationale: "Ledger balance invariants require zero stale reads",
			},
		},
		ActiveTraits: []WorkloadTrait{
			TraitTransactionHeavy,
			TraitConsistencySensitive,
		},
		PrimaryTrait:    TraitTransactionHeavy,
		FallbackApplied: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal ClassificationResult: %v", err)
	}

	var decoded ClassificationResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ClassificationResult: %v", err)
	}

	if decoded.Source != ClassifierSourceRuleBased {
		t.Errorf("expected source %q, got %q", ClassifierSourceRuleBased, decoded.Source)
	}
	if decoded.PrimaryTrait != TraitTransactionHeavy {
		t.Errorf("expected primary trait %q, got %q", TraitTransactionHeavy, decoded.PrimaryTrait)
	}
	if len(decoded.ActiveTraits) != 2 {
		t.Errorf("expected 2 active traits, got %d", len(decoded.ActiveTraits))
	}
}

func TestClassificationHelpers(t *testing.T) {
	result := ClassificationResult{
		Scores: map[WorkloadTrait]ClassificationScore{
			TraitTransactionHeavy: {
				Trait:    TraitTransactionHeavy,
				Score:    0.92,
				IsActive: true,
			},
			TraitReadHeavy: {
				Trait:    TraitReadHeavy,
				Score:    0.40,
				IsActive: false,
			},
			TraitBurstyTraffic: {
				Trait:    TraitBurstyTraffic,
				Score:    0.75,
				IsActive: true,
			},
		},
	}

	if !result.HasTrait(TraitTransactionHeavy) {
		t.Errorf("expected HasTrait(TraitTransactionHeavy) to be true")
	}
	if result.HasTrait(TraitReadHeavy) {
		t.Errorf("expected HasTrait(TraitReadHeavy) to be false")
	}
	if result.HasTrait(TraitWriteHeavy) {
		t.Errorf("expected HasTrait(TraitWriteHeavy) for unassessed trait to be false")
	}

	if score := result.GetScore(TraitTransactionHeavy); score != 0.92 {
		t.Errorf("expected score 0.92, got %v", score)
	}
	if score := result.GetScore(TraitWriteHeavy); score != 0.0 {
		t.Errorf("expected score 0.0 for missing trait, got %v", score)
	}

	filtered := result.FilterActiveTraits(0.70)
	if len(filtered) != 2 {
		t.Errorf("expected 2 traits with score >= 0.70, got %d", len(filtered))
	}
}
