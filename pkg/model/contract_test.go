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
			RequiresACID:           true,
			RequiresIdempotencyKey: true,
			RequiresOutboxPattern:  true,
			ConnectionPoolMax:      25,
		},
		Queue: QueueRules{
			Enabled:              true,
			QueueMechanism:       "POSTGRES_SKIP_LOCKED",
			MaxWorkerConcurrency: 10,
			MaxRetryAttempts:     5,
			BackoffStrategy:      "EXPONENTIAL_WITH_JITTER",
			DeadLetterHandling:   "Store in failed_jobs table and fire alert",
		},
		Constraints: []ImplementationConstraint{
			{
				Rule:      "Do not perform external HTTP calls within an active database transaction",
				Category:  "DATABASE",
				Severity:  "ERROR",
				Rationale: "External network latency holds database connection open, risking pool exhaustion",
			},
			{
				Rule:      "All mutating transactional endpoints must require and validate an 'Idempotency-Key' header",
				Category:  "API_DESIGN",
				Severity:  "ERROR",
				Rationale: "Guarantees safe client retries without creating duplicate orders or double-spending financial balances",
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
		{
			name: "Outbox Contradiction: outbox worker defined when outbox is false",
			mutate: func(c *ArchitectureContract) {
				c.Data.RequiresOutboxPattern = false
				c.Components = append(c.Components, ComponentDefinition{
					Name:             "OutboxDispatcherWorker",
					Responsibilities: []string{"Poll transactional outbox table"},
				})
			},
			wantErr: "contract contradiction: component \"OutboxDispatcherWorker\" references outbox but data.requires_outbox_pattern is false",
		},
		{
			name: "Queue Contradiction: worker defined when queue is disabled",
			mutate: func(c *ArchitectureContract) {
				c.Queue.Enabled = false
				c.Components = append(c.Components, ComponentDefinition{
					Name:             "AsyncWorker",
					Responsibilities: []string{"Process background tasks"},
				})
			},
			wantErr: "contract contradiction: component \"AsyncWorker\" defined while queue.enabled is false",
		},
		{
			name: "ACID Outbox Contradiction: outbox is true but requires_acid is false",
			mutate: func(c *ArchitectureContract) {
				c.Data.RequiresOutboxPattern = true
				c.Data.RequiresACID = false
				c.Data.TransactionPolicy.RequiresACID = false
			},
			wantErr: "contract contradiction: data.requires_outbox_pattern is true but data.requires_acid is false (outbox requires atomic multi-step database transactions)",
		},
		{
			name: "Idempotency Rule Agreement: requires_idempotency_key without constraint",
			mutate: func(c *ArchitectureContract) {
				c.Data.RequiresIdempotencyKey = true
				c.Constraints = []ImplementationConstraint{}
			},
			wantErr: "contract contradiction: data.requires_idempotency_key is true but no idempotency constraint is defined",
		},
		{
			name: "Uniqueness Agreement: uniqueness_strategy without unique constraint",
			mutate: func(c *ArchitectureContract) {
				c.Data.UniquenessStrategy = "DATABASE_UNIQUE_CONSTRAINT"
				c.Constraints = []ImplementationConstraint{}
			},
			wantErr: "contract contradiction: data.uniqueness_strategy is defined but no uniqueness constraint is included",
		},
		{
			name: "At-least-once delivery without worker idempotency or deduplication",
			mutate: func(c *ArchitectureContract) {
				c.Queue.Enabled = true
				c.Queue.MaxWorkerConcurrency = 5
				c.Queue.DeliveryGuarantee = "AT_LEAST_ONCE"
				c.Constraints = []ImplementationConstraint{}
			},
			wantErr: "contract contradiction: queue delivery guarantee is AT_LEAST_ONCE but no worker idempotency or deduplication strategy is documented",
		},
		{
			name: "Queue retry without dead letter handling",
			mutate: func(c *ArchitectureContract) {
				c.Queue.Enabled = true
				c.Queue.MaxWorkerConcurrency = 5
				c.Queue.MaxRetryAttempts = 3
				c.Queue.DeadLetterHandling = ""
			},
			wantErr: "contract contradiction: queue retries are enabled but queue.dead_letter_handling is undefined",
		},
		{
			name: "Queue concurrency unbounded (<= 0)",
			mutate: func(c *ArchitectureContract) {
				c.Queue.Enabled = true
				c.Queue.MaxWorkerConcurrency = 0
			},
			wantErr: "contract contradiction: queue is enabled but queue.max_worker_concurrency is not bounded (must be > 0)",
		},
		{
			name: "External call prohibited in transaction while responsibility places it inside tx",
			mutate: func(c *ArchitectureContract) {
				c.Constraints = append(c.Constraints, ImplementationConstraint{
					Rule: "Do not perform external network calls inside an active database transaction",
				})
				c.Components = []ComponentDefinition{
					{
						Name: "PaymentGatewayModule",
						Responsibilities: []string{
							"Call external payment gateway inside DB tx to charge card",
						},
					},
				}
			},
			wantErr: "contract contradiction: component \"PaymentGatewayModule\" responsibility \"Call external payment gateway inside DB tx to charge card\" places external network call inside DB transaction, violating constraint",
		},
		{
			name: "Offline handling in component without offline_sync policy",
			mutate: func(c *ArchitectureContract) {
				c.Components = []ComponentDefinition{
					{
						Name: "ScannerModule",
						Responsibilities: []string{
							"Buffer offline check-in scans on mobile device",
						},
					},
				}
				c.OfflineSync = nil
			},
			wantErr: "contract contradiction: component \"ScannerModule\" supports offline check-in but contract.offline_sync policy is missing",
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
