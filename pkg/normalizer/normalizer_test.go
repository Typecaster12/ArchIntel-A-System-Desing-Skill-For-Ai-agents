package normalizer

import (
	"testing"

	"github.com/sdskill/archintel/pkg/model"
)

func TestNormalizerExplicitRPSAndDomain(t *testing.T) {
	norm := New()

	raw := model.RawRequirement{
		Prompt: "Build an e-commerce checkout service handling 250 rps with flash sale inventory reservation and PCI-DSS compliance.",
	}

	result := norm.Normalize(raw)

	if result.Domain != model.DomainECommerce {
		t.Errorf("expected Domain %q, got %q", model.DomainECommerce, result.Domain)
	}
	if result.Traffic.ExpectedRPS != 250.0 {
		t.Errorf("expected ExpectedRPS 250.0, got %v", result.Traffic.ExpectedRPS)
	}
	if result.Traffic.PeakRPS != 750.0 {
		t.Errorf("expected PeakRPS 750.0, got %v", result.Traffic.PeakRPS)
	}
	if !result.Traffic.IsExplicit {
		t.Errorf("expected Traffic.IsExplicit to be true")
	}
	if !result.Traffic.IsBursty {
		t.Errorf("expected IsBursty to be true due to 'flash sale'")
	}
	if !result.Data.RequiresACID {
		t.Errorf("expected RequiresACID to be true")
	}
	if !result.Data.HasHighContention {
		t.Errorf("expected HasHighContention to be true due to inventory reservation")
	}
	if !result.HasComplianceNeed("PCI-DSS") {
		t.Errorf("expected PCI-DSS compliance need to be detected")
	}
	if result.Consistency != model.ConsistencyStrong {
		t.Errorf("expected Consistency %q, got %q", model.ConsistencyStrong, result.Consistency)
	}
}

func TestNormalizerDAUCalculation(t *testing.T) {
	norm := New()

	raw := model.RawRequirement{
		Prompt: "A social media feed catalog expecting 100k DAU with sub-100ms latency.",
	}

	result := norm.Normalize(raw)

	if result.Traffic.DailyActiveUsers != 100000 {
		t.Errorf("expected DailyActiveUsers 100000, got %v", result.Traffic.DailyActiveUsers)
	}
	if result.Traffic.ExpectedRPS <= 0 {
		t.Errorf("expected calculated ExpectedRPS > 0, got %v", result.Traffic.ExpectedRPS)
	}
	if result.Latency.P99TargetMs != 100 {
		t.Errorf("expected P99TargetMs 100, got %v", result.Latency.P99TargetMs)
	}
	if result.Traffic.ReadWriteRatio < 5.0 {
		t.Errorf("expected read-heavy ratio >= 5.0, got %v", result.Traffic.ReadWriteRatio)
	}
}

func TestNormalizerTelemetryWriteHeavy(t *testing.T) {
	norm := New()

	raw := model.RawRequirement{
		Prompt: "High-throughput IoT telemetry ingestion pipeline processing 2000 req/sec time-series sensor data.",
	}

	result := norm.Normalize(raw)

	if result.Domain != model.DomainIoTTelemetry {
		t.Errorf("expected Domain %q, got %q", model.DomainIoTTelemetry, result.Domain)
	}
	if result.Traffic.ExpectedRPS != 2000.0 {
		t.Errorf("expected ExpectedRPS 2000.0, got %v", result.Traffic.ExpectedRPS)
	}
	if !result.IsWriteHeavy() {
		t.Errorf("expected IsWriteHeavy() to be true")
	}
	if result.Data.PrimaryStorage != "TIMESERIES" {
		t.Errorf("expected PrimaryStorage 'TIMESERIES', got %q", result.Data.PrimaryStorage)
	}
}

func TestNormalizerVaguePromptDefaultsAndAssumptions(t *testing.T) {
	norm := New()

	raw := model.RawRequirement{
		Prompt: "A simple internal employee task tracker app.",
	}

	result := norm.Normalize(raw)

	if result.Domain != model.DomainGeneralCRUD {
		t.Errorf("expected Domain %q, got %q", model.DomainGeneralCRUD, result.Domain)
	}
	if result.Traffic.IsExplicit {
		t.Errorf("expected Traffic.IsExplicit to be false")
	}
	if len(result.Assumptions) == 0 {
		t.Errorf("expected non-empty assumptions when inputs are vague")
	}
}
