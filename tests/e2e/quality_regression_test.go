package e2e

import (
	"context"
	"strings"
	"testing"

	"github.com/sdskill/archintel/pkg/engine"
	"github.com/sdskill/archintel/pkg/model"
)

func newTestEngine() *engine.Engine {
	return engine.NewEngine(engine.EngineOptions{})
}

// 1. International Conference Management System with concurrent QR check-ins
func TestConferenceManagementSystem(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	prompt := `I want to build a web-based International Conference Management System for a university hosting an international academic conference over three days.
The application must support delegate registration, login, profile management, conference schedules, session information, QR-code-based attendance, delegate check-in, certificate generation, and an administrative dashboard.
The conference is expected to have approximately 1,500 registered delegates, with around 300–500 attendees arriving within a 30-minute window during peak check-in periods. Multiple volunteers will scan QR codes simultaneously from different devices.
The system must prevent duplicate attendance records and ensure that a delegate cannot check in multiple times for the same session unless explicitly authorized.
Registration and check-in must remain reliable during temporary network interruptions or sudden traffic spikes. Certificate generation and bulk email notifications can happen asynchronously, but attendance verification should return a quick response to volunteers.
The initial team is small, the project has a limited infrastructure budget, and the application will initially be deployed on a single cloud environment. Avoid introducing unnecessary distributed infrastructure.`

	resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract

	// Verify Topology is Modular Monolith (not microservices)
	if contract.Topology.Type != model.TopologyModularMonolith {
		t.Errorf("expected MODULAR_MONOLITH, got %v", contract.Topology.Type)
	}

	// Verify Duplicate Attendance Prevention invariant is recognized
	if !resp.Normalized.Invariants.PreventDuplicates {
		t.Errorf("expected Invariants.PreventDuplicates to be true")
	}

	// Verify Database uniqueness strategy
	if contract.Data.UniquenessStrategy == "" {
		t.Errorf("expected Data.UniquenessStrategy to specify unique constraint")
	}
	if !strings.Contains(contract.Data.UniquenessStrategy, "UNIQUE") {
		t.Errorf("expected uniqueness strategy to mention UNIQUE constraint, got %q", contract.Data.UniquenessStrategy)
	}

	// Verify Idempotency is required for concurrent/retry QR scans
	if !contract.Data.RequiresIdempotencyKey {
		t.Errorf("expected RequiresIdempotencyKey to be true for multi-device concurrent scans")
	}

	// Verify Queue is enabled with SKIP LOCKED (not Kafka or RabbitMQ)
	if !contract.Queue.Enabled {
		t.Errorf("expected Queue to be enabled for async certificate/email tasks")
	}
	if contract.Queue.QueueMechanism != "POSTGRES_SKIP_LOCKED" {
		t.Errorf("expected POSTGRES_SKIP_LOCKED, got %v", contract.Queue.QueueMechanism)
	}

	// Verify Outbox & Worker agreement (no contradiction)
	if err := contract.ValidateConsistency(); err != nil {
		t.Errorf("contract consistency validation failed: %v", err)
	}

	// Verify arrival figures were extracted rather than lost
	if resp.Normalized.Traffic.StatedAttendees != 1500 {
		t.Errorf("expected StatedAttendees 1500, got %d", resp.Normalized.Traffic.StatedAttendees)
	}
	if resp.Normalized.Traffic.TrafficSource != "DERIVED_FROM_ARRIVALS" {
		t.Errorf("expected TrafficSource DERIVED_FROM_ARRIVALS, got %s", resp.Normalized.Traffic.TrafficSource)
	}

	// Verify offline synchronization gap or assumption was recorded
	foundOfflineNotice := false
	for _, a := range contract.ActiveAssumptions {
		if strings.Contains(strings.ToLower(a), "offline") || strings.Contains(strings.ToLower(a), "retries") || strings.Contains(strings.ToLower(a), "idempotency") {
			foundOfflineNotice = true
			break
		}
	}
	if !foundOfflineNotice {
		t.Errorf("expected offline/retry notice in active assumptions")
	}

	// Verify Microservices and dedicated brokers were rejected
	foundMicroReject := false
	foundBrokerReject := false
	for _, r := range contract.RejectedAlternatives {
		if strings.Contains(r.Pattern, "Microservices") {
			foundMicroReject = true
		}
		if strings.Contains(r.Pattern, "Dedicated Message Broker") {
			foundBrokerReject = true
		}
	}
	if !foundMicroReject {
		t.Errorf("expected Microservices rejection")
	}
	if !foundBrokerReject {
		t.Errorf("expected Dedicated Broker rejection")
	}
}

// 2. E-commerce checkout with payment idempotency and inventory consistency
func TestEcommerceCheckoutIdempotency(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	prompt := `Build an e-commerce checkout service handling 250 RPS with flash sale inventory reservation and payment processing.
Must prevent duplicate charges and ensure inventory invariants are maintained under concurrent purchases.`

	resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if contract.Data.Locking != model.LockingPessimistic {
		t.Errorf("expected PESSIMISTIC locking for flash sale inventory, got %v", contract.Data.Locking)
	}
	if !contract.Data.RequiresIdempotencyKey {
		t.Errorf("expected RequiresIdempotencyKey to be true for payment transactions")
	}
	if !contract.Data.RequiresOutboxPattern {
		t.Errorf("expected RequiresOutboxPattern to be true for payment event dispatch")
	}
}

// 3. Flash-sale system with bursty traffic and inventory contention
func TestFlashSaleHighContention(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	prompt := `High contention flash sale system with extreme traffic spikes of 2,000 RPS competing for 50 limited edition items.`

	resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if contract.Data.Locking != model.LockingPessimistic {
		t.Errorf("expected PESSIMISTIC locking for 50 items under 2,000 RPS burst, got %v", contract.Data.Locking)
	}
	if contract.Data.ConnectionPoolMax < 35 {
		t.Errorf("expected ConnectionPoolMax >= 35 for 2000 RPS, got %d", contract.Data.ConnectionPoolMax)
	}
}

// 4. Notification service with asynchronous email delivery
func TestNotificationServiceAsync(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	prompt := `Notification dispatcher service processing bulk emails and webhooks from incoming job queues with retries.`

	resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if !contract.Queue.Enabled {
		t.Errorf("expected Queue.Enabled to be true")
	}
	if contract.Queue.DeliveryGuarantee == "" {
		t.Errorf("expected DeliveryGuarantee to be specified")
	}
	if contract.Queue.MaxWorkerConcurrency <= 0 {
		t.Errorf("expected bounded MaxWorkerConcurrency > 0, got %d", contract.Queue.MaxWorkerConcurrency)
	}
}

// 5. URL shortener with read-heavy traffic
func TestURLShortenerReadHeavy(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	prompt := `Public URL shortener handling 10,000 read RPS with 20:1 read-to-write ratio and sub-15ms latency target.`

	resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if contract.Data.Locking != model.LockingNone {
		t.Errorf("expected LockingNone for read-heavy URL shortener, got %v", contract.Data.Locking)
	}
	foundCacheAside := false
	for _, p := range resp.DecisionSummary.SelectedPatterns {
		if p.PatternID == "PAT-CACHE-ASIDE" {
			foundCacheAside = true
		}
	}
	if !foundCacheAside {
		t.Errorf("expected Cache-Aside to be selected for read-heavy shortener")
	}
}

// 6. Simple CRUD application with a limited infrastructure budget
func TestSimpleCRUDAdminPanel(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	prompt := `Internal company employee directory CRUD admin tool for 50 HR staff with limited infrastructure budget.`

	resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if contract.Topology.Type != model.TopologyModularMonolith {
		t.Errorf("expected Modular Monolith for internal CRUD, got %v", contract.Topology.Type)
	}
	if contract.Queue.Enabled {
		t.Errorf("expected Queue to be disabled for simple internal CRUD")
	}
	if contract.Data.Locking != model.LockingNone {
		t.Errorf("expected LockingNone for low-scale admin panel, got %v", contract.Data.Locking)
	}
}

// 7. Job-processing service with retries and worker failures
func TestJobProcessingServiceRetries(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	prompt := `Background worker pipeline that ingests third-party data feeds, retries transient API failures, and routes corrupted payload items to DLQ.`

	resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if !contract.Queue.Enabled {
		t.Errorf("expected Queue to be enabled")
	}
	if contract.Queue.BackoffStrategy != "EXPONENTIAL_WITH_JITTER" {
		t.Errorf("expected EXPONENTIAL_WITH_JITTER backoff, got %v", contract.Queue.BackoffStrategy)
	}
	if contract.Queue.DeadLetterHandling == "" {
		t.Errorf("expected DeadLetterHandling to be configured")
	}
}

// 8. Application requiring temporary offline operation and later synchronization
func TestOfflineSyncApplication(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	prompt := `Mobile field inspection app used in remote areas with frequent network interruptions. Inspectors collect inspection records offline and sync back when connectivity resumes. Must prevent duplicate records on reconnect.`

	resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if !resp.Normalized.Invariants.RequiresOfflineHandling {
		t.Errorf("expected Invariants.RequiresOfflineHandling to be true")
	}
	if !contract.Data.RequiresIdempotencyKey {
		t.Errorf("expected RequiresIdempotencyKey to be true for offline replay")
	}
	if contract.Data.UniquenessStrategy == "" {
		t.Errorf("expected UniquenessStrategy to be defined for duplicate prevention on sync")
	}
	if err := contract.ValidateConsistency(); err != nil {
		t.Errorf("contract consistency validation failed: %v", err)
	}
}
