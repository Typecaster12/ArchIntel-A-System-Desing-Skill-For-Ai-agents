package classifier

import (
	"context"
	"testing"

	"github.com/sdskill/archintel/pkg/model"
)

func TestHeuristicClassifierEcommerceCheckout(t *testing.T) {
	c := NewRuleBasedClassifier()
	ctx := context.Background()

	req := &model.NormalizedRequirements{
		ProjectName: "CheckoutService",
		Domain:      model.DomainECommerce,
		Traffic: model.TrafficProfile{
			ExpectedRPS: 250,
			PeakRPS:     1000,
			IsBursty:    true,
		},
		Data: model.DataProfile{
			RequiresACID:      true,
			HasHighContention: true,
		},
		Consistency: model.ConsistencyStrong,
		Latency: model.LatencyProfile{
			P99TargetMs: 80,
		},
	}

	res, err := c.Classify(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error during classification: %v", err)
	}

	if !res.HasTrait(model.TraitTransactionHeavy) {
		t.Errorf("expected TraitTransactionHeavy to be active")
	}
	if !res.HasTrait(model.TraitConsistencySensitive) {
		t.Errorf("expected TraitConsistencySensitive to be active")
	}
	if !res.HasTrait(model.TraitBurstyTraffic) {
		t.Errorf("expected TraitBurstyTraffic to be active")
	}
	if !res.HasTrait(model.TraitHighContention) {
		t.Errorf("expected TraitHighContention to be active")
	}
	if res.Source != model.ClassifierSourceRuleBased {
		t.Errorf("expected source %q, got %q", model.ClassifierSourceRuleBased, res.Source)
	}
}

func TestHeuristicClassifierTelemetryPipeline(t *testing.T) {
	c := NewRuleBasedClassifier()
	ctx := context.Background()

	req := &model.NormalizedRequirements{
		ProjectName: "DeviceIngest",
		Domain:      model.DomainIoTTelemetry,
		Traffic: model.TrafficProfile{
			ExpectedRPS:    2500,
			PeakRPS:        5000,
			ReadWriteRatio: 0.1, // 10 writes per read
		},
		Data: model.DataProfile{
			PrimaryStorage: "TIMESERIES",
		},
		Consistency: model.ConsistencyEventual,
	}

	res, err := c.Classify(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.HasTrait(model.TraitWriteHeavy) {
		t.Errorf("expected TraitWriteHeavy to be active for IoT telemetry")
	}
	if res.HasTrait(model.TraitReadHeavy) {
		t.Errorf("expected TraitReadHeavy to be inactive for write-skewed stream")
	}
}
