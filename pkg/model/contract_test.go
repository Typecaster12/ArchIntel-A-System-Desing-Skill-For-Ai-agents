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
			TransactionPolicy:      TransactionPolicy{Scope: "PER_OPERATION_ATOMIC"},
			RequiresIdempotencyKey: true,
			IdempotencyPolicy: IdempotencyPolicy{
				Required:              true,
				ApplicableOperations:  []string{"capture_payment"},
				FingerprintMechanism:  "SHA-256",
				StateStorageMechanism: "idempotency_keys",
			},
			RequiresOutboxPattern: true,
			ConnectionPoolMax:     25,
		},
		Queue: QueueRules{
			Enabled:              true,
			QueueMechanism:       "POSTGRES_SKIP_LOCKED",
			MaxWorkerConcurrency: 10,
			MaxRetryAttempts:     5,
			BackoffStrategy:      "EXPONENTIAL_WITH_JITTER",
			DeadLetterHandling:   "Store in failed_jobs table and fire alert",
		},
		CapacityPlan: CapacityPlan{
			WorkerConcurrencyMax: 10,
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
				c.Data.IdempotencyPolicy = IdempotencyPolicy{
					Required:              true,
					FingerprintMechanism:  "SHA-256 hash of request body",
					StateStorageMechanism: "PostgreSQL idempotency_keys table",
				}
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
		{
			name: "Idempotency flag and policy requirement disagreement",
			mutate: func(c *ArchitectureContract) {
				c.Data.RequiresIdempotencyKey = true
				c.Data.IdempotencyPolicy = IdempotencyPolicy{
					Required: false,
				}
			},
			wantErr: "contract contradiction: data.requires_idempotency_key does not agree with data.idempotency_policy.required",
		},
		{
			name: "Worker concurrency mismatch across queue and capacity plan",
			mutate: func(c *ArchitectureContract) {
				c.Queue.Enabled = true
				c.Queue.MaxWorkerConcurrency = 8
				c.CapacityPlan.WorkerConcurrencyMax = 20
			},
			wantErr: "contract contradiction: queue.max_worker_concurrency (8) does not match capacity_plan.worker_concurrency_max (20)",
		},
		{
			name: "Required idempotency with empty applicable operations",
			mutate: func(c *ArchitectureContract) {
				c.Data.RequiresIdempotencyKey = true
				c.Data.IdempotencyPolicy = IdempotencyPolicy{
					Required:              true,
					ApplicableOperations:  []string{},
					FingerprintMechanism:  "SHA-256",
					StateStorageMechanism: "idempotency_keys",
				}
				c.Constraints = []ImplementationConstraint{
					{Rule: "All mutating endpoints must require idempotency tokens"},
				}
			},
			wantErr: "contract contradiction: data.requires_idempotency_key is true but data.idempotency_policy.applicable_operations is empty",
		},
		{
			name: "Outbox pattern true while queue is disabled",
			mutate: func(c *ArchitectureContract) {
				c.Data.RequiresOutboxPattern = true
				c.Data.RequiresACID = true
				c.Data.TransactionPolicy.RequiresACID = true
				c.Data.TransactionPolicy.Scope = "PER_OPERATION_ATOMIC"
				c.Queue.Enabled = false
			},
			wantErr: "contract contradiction: data.requires_outbox_pattern is true but queue.enabled is false (transactional outbox requires an active queue/worker mechanism to dispatch events)",
		},
		{
			name: "Queue enabled without queue mechanism",
			mutate: func(c *ArchitectureContract) {
				c.Queue.Enabled = true
				c.Queue.MaxWorkerConcurrency = 5
				c.Queue.QueueMechanism = ""
			},
			wantErr: "contract contradiction: queue.enabled is true but queue.queue_mechanism is missing or empty",
		},
		{
			name: "Queue enabled without worker component",
			mutate: func(c *ArchitectureContract) {
				c.Queue.Enabled = true
				c.Queue.MaxWorkerConcurrency = 5
				c.Queue.QueueMechanism = "POSTGRES_SKIP_LOCKED"
				c.Components = []ComponentDefinition{{Name: "WebAPI", Responsibilities: []string{"Handle HTTP"}}}
			},
			wantErr: "contract contradiction: queue.enabled is true but no worker or background processor component is configured in components",
		},
		{
			name: "Operational connection pool starvation: ConnectionPoolMax < MaxWorkerConcurrency with DB workers",
			mutate: func(c *ArchitectureContract) {
				c.Queue.Enabled = true
				c.Queue.MaxWorkerConcurrency = 15
				c.Queue.QueueMechanism = "POSTGRES_SKIP_LOCKED"
				c.Components = []ComponentDefinition{
					{Name: "JobWorker", Responsibilities: []string{"Poll queue"}, PersistenceAccess: true},
				}
				c.Data.ConnectionPoolMax = 10
			},
			wantErr: "contract contradiction: data.connection_pool_max (10) is strictly less than queue.max_worker_concurrency (15) while workers access the database; worker pool will starve connection pool",
		},
		{
			name: "Operational connection pool equal sizing with shared DB",
			mutate: func(c *ArchitectureContract) {
				c.Queue.Enabled = true
				c.Queue.MaxWorkerConcurrency = 10
				c.Queue.QueueMechanism = "POSTGRES_SKIP_LOCKED"
				c.Components = []ComponentDefinition{
					{Name: "WebAPI", Responsibilities: []string{"Handle HTTP"}, PersistenceAccess: true},
					{Name: "JobWorker", Responsibilities: []string{"Poll queue"}, PersistenceAccess: true},
				}
				c.Data.ConnectionPoolMax = 10
			},
			wantErr: "contract contradiction: data.connection_pool_max (10) equals queue.max_worker_concurrency (10) while web handlers and workers share the database connection pool; leaves no headroom for web traffic during worker saturation",
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
