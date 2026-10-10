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

func TestNormalizerTrafficExtractionVariations(t *testing.T) {
	norm := New()

	t.Run("baseline only with 3x derived peak", func(t *testing.T) {
		res := norm.Normalize(model.RawRequirement{
			Prompt: "API service handling 100 RPS steady load",
		})
		if res.Traffic.ExpectedRPS != 100.0 {
			t.Errorf("expected baseline 100.0, got %v", res.Traffic.ExpectedRPS)
		}
		if res.Traffic.PeakRPS != 300.0 {
			t.Errorf("expected peak 300.0, got %v", res.Traffic.PeakRPS)
		}
		if res.Traffic.IsPeakExplicit {
			t.Errorf("expected IsPeakExplicit to be false")
		}
		if res.FieldProvenance["traffic.peak_rps"].Source != model.ProvenanceDerived {
			t.Errorf("expected provenance Derived, got %v", res.FieldProvenance["traffic.peak_rps"].Source)
		}
	})

	t.Run("explicit baseline plus peak preserves user values without 3x multiplier", func(t *testing.T) {
		res := norm.Normalize(model.RawRequirement{
			Prompt: "Conference backend expecting baseline 50 RPS and peak of 80 RPS during keynote check-in",
		})
		if res.Traffic.ExpectedRPS != 50.0 {
			t.Errorf("expected baseline 50.0, got %v", res.Traffic.ExpectedRPS)
		}
		if res.Traffic.PeakRPS != 80.0 {
			t.Errorf("expected explicit peak 80.0, got %v", res.Traffic.PeakRPS)
		}
		if !res.Traffic.IsPeakExplicit {
			t.Errorf("expected IsPeakExplicit to be true")
		}
		if res.FieldProvenance["traffic.peak_rps"].Source != model.ProvenanceUserProvided {
			t.Errorf("expected provenance UserProvided, got %v", res.FieldProvenance["traffic.peak_rps"].Source)
		}
		if !res.FieldProvenance["traffic.peak_rps"].IsHardConstraint {
			t.Errorf("expected explicit peak to be marked as hard constraint")
		}
	})

	t.Run("comma-separated numbers in baseline and peak", func(t *testing.T) {
		res := norm.Normalize(model.RawRequirement{
			Prompt: "Retail catalog system with baseline 2,500 RPS and peak 10,000 req/s",
		})
		if res.Traffic.ExpectedRPS != 2500.0 {
			t.Errorf("expected baseline 2500.0, got %v", res.Traffic.ExpectedRPS)
		}
		if res.Traffic.PeakRPS != 10000.0 {
			t.Errorf("expected peak 10000.0, got %v", res.Traffic.PeakRPS)
		}
		if !res.Traffic.IsPeakExplicit {
			t.Errorf("expected IsPeakExplicit to be true")
		}
	})

	t.Run("prompt without RPS information uses defaults", func(t *testing.T) {
		res := norm.Normalize(model.RawRequirement{
			Prompt: "A company intranet wiki document portal.",
		})
		if res.Traffic.ExpectedRPS != 10.0 {
			t.Errorf("expected default baseline 10.0, got %v", res.Traffic.ExpectedRPS)
		}
		if res.Traffic.PeakRPS != 30.0 {
			t.Errorf("expected default peak 30.0, got %v", res.Traffic.PeakRPS)
		}
		if res.Traffic.IsExplicit {
			t.Errorf("expected IsExplicit false")
		}
		if res.Traffic.TrafficSource != "ASSUMED_DEFAULT" {
			t.Errorf("expected ASSUMED_DEFAULT traffic source, got %v", res.Traffic.TrafficSource)
		}
	})

	t.Run("physical arrivals distinguish physical rate from HTTP RPS", func(t *testing.T) {
		res := norm.Normalize(model.RawRequirement{
			Prompt: "Registration venue with 300-500 attendees arriving within a 30-minute window for badge scanning",
		})
		if res.Traffic.TrafficSource != "DERIVED_FROM_ARRIVALS" {
			t.Errorf("expected DERIVED_FROM_ARRIVALS, got %v", res.Traffic.TrafficSource)
		}
		if res.Traffic.ArrivalsPerSec <= 0 || res.Traffic.ArrivalsPerSec > 1.0 {
			t.Errorf("expected physical arrival rate ~0.22/sec, got %v", res.Traffic.ArrivalsPerSec)
		}
		if res.Traffic.PeakRPS <= res.Traffic.ArrivalsPerSec {
			t.Errorf("expected Peak HTTP RPS (%v) to exceed physical arrivals/sec (%v) to account for retries and scans",
				res.Traffic.PeakRPS, res.Traffic.ArrivalsPerSec)
		}
	})
}
