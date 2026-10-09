package model

import (
	"encoding/json"
	"testing"
)

func TestRequirementsJSONSerialization(t *testing.T) {
	req := NormalizedRequirements{
		ProjectName: "EcommercePaymentGateway",
		Summary:     "High volume payment processing service with webhook dispatches",
		Domain:      DomainECommerce,
		Traffic: TrafficProfile{
			ExpectedRPS:      250.0,
			PeakRPS:          1000.0,
			DailyActiveUsers: 500000,
			ReadWriteRatio:   1.0, // 1:1 read/write
			IsBursty:         true,
			IsExplicit:       true,
		},
		Data: DataProfile{
			PrimaryStorage:    "RELATIONAL",
			DailyVolumeMB:     5000.0,
			RequiresACID:      true,
			HasHighContention: true,
			RetentionDays:     365,
		},
		Latency: LatencyProfile{
			P95TargetMs:     100,
			P99TargetMs:     250,
			MaxAcceptableMs: 1000,
		},
		Consistency: ConsistencyStrong,
		Constraints: Constraints{
			PreferredStack:   []string{"Go", "PostgreSQL"},
			DeploymentTarget: "CONTAINER",
			ComplianceNeeds:  []string{"PCI-DSS"},
		},
		InferredTraits: []InferredTrait{
			{
				Name:       "BURSTY_TRAFFIC",
				Rationale:  "Flash sales create sudden traffic spikes",
				Confidence: 0.9,
			},
		},
		Assumptions: []string{
			"Assuming single-region deployment initially based on budget tier",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal NormalizedRequirements: %v", err)
	}

	var decoded NormalizedRequirements
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal NormalizedRequirements: %v", err)
	}

	if decoded.ProjectName != req.ProjectName {
		t.Errorf("expected ProjectName %q, got %q", req.ProjectName, decoded.ProjectName)
	}
	if decoded.Consistency != ConsistencyStrong {
		t.Errorf("expected Consistency %q, got %q", ConsistencyStrong, decoded.Consistency)
	}
	if !decoded.Traffic.IsBursty {
		t.Errorf("expected IsBursty to be true")
	}
}

func TestRequirementsHelperMethods(t *testing.T) {
	tests := []struct {
		name                 string
		req                  NormalizedRequirements
		wantReadHeavy        bool
		wantWriteHeavy       bool
		wantStrictConsistent bool
		wantHighThroughput   bool
	}{
		{
			name: "Read-heavy content catalog",
			req: NormalizedRequirements{
				Traffic: TrafficProfile{
					ExpectedRPS:    50,
					PeakRPS:        150,
					ReadWriteRatio: 20.0, // 20:1
				},
				Consistency: ConsistencyEventual,
				Data:        DataProfile{RequiresACID: false},
			},
			wantReadHeavy:        true,
			wantWriteHeavy:       false,
			wantStrictConsistent: false,
			wantHighThroughput:   false,
		},
		{
			name: "Write-heavy telemetry pipeline",
			req: NormalizedRequirements{
				Traffic: TrafficProfile{
					ExpectedRPS:    1200,
					PeakRPS:        3500,
					ReadWriteRatio: 0.1, // 10 writes per 1 read
				},
				Consistency: ConsistencyEventual,
				Data:        DataProfile{RequiresACID: false},
			},
			wantReadHeavy:        false,
			wantWriteHeavy:       true,
			wantStrictConsistent: false,
			wantHighThroughput:   true,
		},
		{
			name: "Strict consistency financial ledger",
			req: NormalizedRequirements{
				Traffic: TrafficProfile{
					ExpectedRPS:    100,
					PeakRPS:        300,
					ReadWriteRatio: 2.0,
				},
				Consistency: ConsistencyStrong,
				Data:        DataProfile{RequiresACID: true},
			},
			wantReadHeavy:        false,
			wantWriteHeavy:       false,
			wantStrictConsistent: true,
			wantHighThroughput:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.IsReadHeavy(); got != tt.wantReadHeavy {
				t.Errorf("IsReadHeavy() = %v, want %v", got, tt.wantReadHeavy)
			}
			if got := tt.req.IsWriteHeavy(); got != tt.wantWriteHeavy {
				t.Errorf("IsWriteHeavy() = %v, want %v", got, tt.wantWriteHeavy)
			}
			if got := tt.req.HasStrictConsistency(); got != tt.wantStrictConsistent {
				t.Errorf("HasStrictConsistency() = %v, want %v", got, tt.wantStrictConsistent)
			}
			if got := tt.req.HasHighThroughput(); got != tt.wantHighThroughput {
				t.Errorf("HasHighThroughput() = %v, want %v", got, tt.wantHighThroughput)
			}
		})
	}
}

func TestComplianceNeeds(t *testing.T) {
	req := NormalizedRequirements{
		Constraints: Constraints{
			ComplianceNeeds: []string{"PCI-DSS", "GDPR"},
		},
	}

	if !req.HasComplianceNeed("pci-dss") {
		t.Errorf("expected HasComplianceNeed('pci-dss') to be true (case-insensitive)")
	}
	if !req.HasComplianceNeed("GDPR") {
		t.Errorf("expected HasComplianceNeed('GDPR') to be true")
	}
	if req.HasComplianceNeed("HIPAA") {
		t.Errorf("expected HasComplianceNeed('HIPAA') to be false")
	}
}
