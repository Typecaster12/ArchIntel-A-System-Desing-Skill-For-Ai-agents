package model

import (
	"encoding/json"
	"testing"
)

func TestContractJSONSerializationAndValidation(t *testing.T) {
	contract := ArchitectureContract{
		ContractVersion: "1.0.0",
		ProjectName:     "CheckoutService",
		Topology: SystemTopology{
			Type:            TopologyModularMonolith,
			Rationale:       "Simplicity and transactional reliability inside a single deployment unit",
			ScalingStrategy: "HORIZONTALLY_STATELESS",
		},
		Components: []ComponentDefinition{
			{
				Name: "PaymentModule",
				Responsibilities: []string{
					"Capture credit card charges",
					"Publish payment success events to transactional outbox",
				},
				Dependencies:      []string{"OrderModule"},
				PersistenceAccess: true,
			},
			{
				Name: "OutboxWorker",
				Responsibilities: []string{
					"Poll outbox table with FOR UPDATE SKIP LOCKED",
					"Dispatch events to external webhooks",
				},
				Dependencies:      []string{},
				PersistenceAccess: true,
			},
		},
		Data: DataRules{
			IsolationLevel:         "READ_COMMITTED",
			Locking:                LockingPessimistic,
			RequiresIdempotencyKey: true,
			RequiresOutboxPattern:  true,
			ConnectionPoolMax:      25,
		},
		Queue: QueueRules{
			Enabled:               true,
			QueueMechanism:        "POSTGRES_SKIP_LOCKED",
			MaxWorkerConcurrency: 10,
			MaxRetryAttempts:      5,
			BackoffStrategy:       "EXPONENTIAL_WITH_JITTER",
			DeadLetterHandling:    "Store in failed_jobs table and fire alert",
		},
		Constraints: []ImplementationConstraint{
			{
				Rule:      "Do not perform external HTTP calls within an active database transaction",
				Category:  "DATABASE",
				Severity:  "ERROR",
				Rationale: "External network latency holds database connection open, risking pool exhaustion",
			},
		},
		RejectedAlternatives: []RejectedAlternative{
			{
				Pattern: "Microservices with Kafka Broker",
				Reason:  "Scale of 200 RPS does not justify distributed system overhead and multi-service deployments",
			},
		},
		ValidationCriteria: []ValidationCriterion{
			{
				Identifier:  "CHK-OUTBOX-01",
				Description: "Verify outbox table schema exists alongside payments",
				IsAutomated: true,
			},
		},
		ActiveAssumptions: []string{
			"Target database is PostgreSQL 14+",
		},
	}

	if err := contract.Validate(); err != nil {
		t.Fatalf("expected contract to be valid, got error: %v", err)
	}

	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("failed to marshal ArchitectureContract: %v", err)
	}

	var decoded ArchitectureContract
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ArchitectureContract: %v", err)
	}

	if decoded.ProjectName != "CheckoutService" {
		t.Errorf("expected ProjectName 'CheckoutService', got %q", decoded.ProjectName)
	}
	if decoded.Topology.Type != TopologyModularMonolith {
		t.Errorf("expected Topology %q, got %q", TopologyModularMonolith, decoded.Topology.Type)
	}
	if !decoded.Data.RequiresIdempotencyKey {
		t.Errorf("expected RequiresIdempotencyKey to be true")
	}
}

func TestContractValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ArchitectureContract)
		wantErr string
	}{
		{
			name: "Missing ContractVersion",
			mutate: func(c *ArchitectureContract) {
				c.ContractVersion = ""
			},
			wantErr: "contract_version is required",
		},
		{
			name: "Missing ProjectName",
			mutate: func(c *ArchitectureContract) {
				c.ProjectName = ""
			},
			wantErr: "project_name is required",
		},
		{
			name: "Missing Components",
			mutate: func(c *ArchitectureContract) {
				c.Components = []ComponentDefinition{}
			},
			wantErr: "at least one component definition is required",
		},
		{
			name: "Missing IsolationLevel",
			mutate: func(c *ArchitectureContract) {
				c.Data.IsolationLevel = ""
			},
			wantErr: "data.isolation_level is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := ArchitectureContract{
				ContractVersion: "1.0.0",
				ProjectName:     "TestApp",
				Topology:        SystemTopology{Type: TopologyModularMonolith},
				Components:      []ComponentDefinition{{Name: "Core"}},
				Data:            DataRules{IsolationLevel: "READ_COMMITTED"},
			}
			tt.mutate(&base)
			err := base.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("got error %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
