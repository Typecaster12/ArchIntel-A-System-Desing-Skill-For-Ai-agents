package e2e

import (
	"context"
	"strings"
	"testing"

	"github.com/sdskill/archintel/pkg/engine"
	"github.com/sdskill/archintel/pkg/model"
	"github.com/sdskill/archintel/pkg/validator"
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

	// Verify ProjectName is domain-aware (not generic SystemDesignTarget)
	if contract.ProjectName == "SystemDesignTarget" {
		t.Errorf("expected domain-aware project name, got %q", contract.ProjectName)
	}

	// Verify ACID transaction consistency: requires_acid must be TRUE
	if !contract.Data.RequiresACID {
		t.Errorf("expected Data.RequiresACID to be true for duplicate prevention and outbox operations")
	}
	if !contract.Data.TransactionPolicy.RequiresACID {
		t.Errorf("expected TransactionPolicy.RequiresACID to be true")
	}
	if contract.Data.TransactionPolicy.Scope != "PER_OPERATION_ATOMIC" {
		t.Errorf("expected TransactionPolicy.Scope PER_OPERATION_ATOMIC, got %q", contract.Data.TransactionPolicy.Scope)
	}

	// Verify Concurrency Details: database unique constraint without explicit application locking
	if contract.Data.ConcurrencyDetails.ExplicitLockingRequired {
		t.Errorf("expected ExplicitLockingRequired to be false (PostgreSQL unique indexes handle conflict)")
	}
	if !strings.Contains(contract.Data.ConcurrencyDetails.Mechanism, "UNIQUE") {
		t.Errorf("expected ConcurrencyDetails.Mechanism to mention unique constraint, got %q", contract.Data.ConcurrencyDetails.Mechanism)
	}

	// Verify Capacity Plan via Little's Law
	if contract.CapacityPlan.RecommendedDBPoolMax <= 0 {
		t.Errorf("expected positive RecommendedDBPoolMax, got %d", contract.CapacityPlan.RecommendedDBPoolMax)
	}
	if !strings.Contains(contract.CapacityPlan.LittlesLawFormula, "L = lambda * W") {
		t.Errorf("expected Little's Law formula in capacity plan, got %q", contract.CapacityPlan.LittlesLawFormula)
	}

	// Verify Offline Synchronization strategy is defined
	if contract.OfflineSync == nil {
		t.Fatalf("expected contract.OfflineSync to be non-nil for offline-capable workload")
	}
	if contract.OfflineSync.Policy != "SERVER_FIRST_WRITE_WINS_WITH_AUDIT_LOG" {
		t.Errorf("expected SERVER_FIRST_WRITE_WINS_WITH_AUDIT_LOG, got %q", contract.OfflineSync.Policy)
	}
	if !contract.OfflineSync.IsDefaultRecommendation {
		t.Errorf("expected OfflineSync to be labeled as a recommendation, not a user decision")
	}

	// Verify Decision Traces are populated
	if len(contract.DecisionTraces) < 3 {
		t.Errorf("expected at least 3 decision traces, got %d", len(contract.DecisionTraces))
	}

	// Verify Field Provenance distinguishes user provided from derived
	if len(contract.FieldProvenance) == 0 {
		t.Errorf("expected FieldProvenance to be populated")
	}
	if contract.FieldProvenance["data.requires_acid"].Source != model.ProvenanceDerived {
		t.Errorf("expected data.requires_acid provenance DERIVED, got %v", contract.FieldProvenance["data.requires_acid"].Source)
	}
	if contract.FieldProvenance["traffic.expected_rps"].Source != model.ProvenanceDerived {
		t.Errorf("expected traffic.expected_rps provenance DERIVED, got %v", contract.FieldProvenance["traffic.expected_rps"].Source)
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

	// Regression checks: Bursty traffic trait is active from arrival window
	if !resp.Normalized.Traffic.IsBursty {
		t.Errorf("expected Normalized.Traffic.IsBursty to be true for concentrated arrival window")
	}
	if !resp.Classification.HasTrait(model.TraitBurstyTraffic) {
		t.Errorf("expected TraitBurstyTraffic to be active in classification result")
	}

	// Regression checks: Background Intensive trait is active from async certificate and email processing
	if !resp.Classification.HasTrait(model.TraitBackgroundIntensive) {
		t.Errorf("expected TraitBackgroundIntensive to be active in classification result")
	}

	// Regression checks: Consistency Sensitive trait is active from duplicate attendance prevention
	if !resp.Classification.HasTrait(model.TraitConsistencySensitive) {
		t.Errorf("expected TraitConsistencySensitive to be active in classification result")
	}

	// Regression checks: Idempotency policy coherence
	if !contract.Data.IdempotencyPolicy.Required {
		t.Errorf("expected contract.Data.IdempotencyPolicy.Required to be true")
	}
	if contract.Data.RequiresIdempotencyKey != contract.Data.IdempotencyPolicy.Required {
		t.Errorf("expected RequiresIdempotencyKey and IdempotencyPolicy.Required to agree")
	}

	// Regression checks: Worker concurrency agreement across queue and capacity plan
	if contract.Queue.MaxWorkerConcurrency != contract.CapacityPlan.WorkerConcurrencyMax {
		t.Errorf("worker concurrency mismatch: queue=%d, capacity_plan=%d",
			contract.Queue.MaxWorkerConcurrency, contract.CapacityPlan.WorkerConcurrencyMax)
	}

	// Regression checks: durable lease claim mechanism is defined
	if contract.Queue.JobClaimMechanism == "" {
		t.Errorf("expected non-empty Queue.JobClaimMechanism")
	}
	if contract.Queue.ClaimLeaseDuration == "" {
		t.Errorf("expected non-empty Queue.ClaimLeaseDuration")
	}

	// Regression checks: Offline Handling state is RECOMMENDED_PROVISIONAL
	if contract.OfflineSync.HandlingState != "RECOMMENDED_PROVISIONAL" {
		t.Errorf("expected OfflineSync.HandlingState RECOMMENDED_PROVISIONAL, got %s", contract.OfflineSync.HandlingState)
	}

	// Regression checks: constraint severity is WARNING for provisional recommendation
	foundOfflineWarning := false
	for _, c := range contract.Constraints {
		if c.Category == "DATA_INTEGRITY" && c.Severity == "WARNING" {
			foundOfflineWarning = true
			break
		}
	}
	if !foundOfflineWarning {
		t.Errorf("expected provisional offline sync constraint to have severity WARNING")
	}

	// Regression checks: Fencing token & lease renewal mechanisms defined
	if contract.Queue.FencingTokenMechanism == "" {
		t.Errorf("expected non-empty FencingTokenMechanism in QueueRules")
	}
	if contract.Queue.LeaseRenewalPolicy == "" {
		t.Errorf("expected non-empty LeaseRenewalPolicy in QueueRules")
	}

	// Regression checks: Provider-specific idempotency integrations populated
	if len(contract.Queue.ProviderIntegrations) < 2 {
		t.Fatalf("expected at least 2 provider integrations for conference (email, cert storage), got %d", len(contract.Queue.ProviderIntegrations))
	}
	foundEmailIntegration := false
	foundStorageIntegration := false
	for _, p := range contract.Queue.ProviderIntegrations {
		if strings.Contains(strings.ToLower(p.ProviderName), "email") {
			foundEmailIntegration = true
			if p.AmbiguousTimeoutPlan == "" {
				t.Errorf("email integration missing AmbiguousTimeoutPlan")
			}
		}
		if strings.Contains(strings.ToLower(p.ProviderName), "storage") || strings.Contains(strings.ToLower(p.ProviderName), "cert") {
			foundStorageIntegration = true
			if p.AmbiguousTimeoutPlan == "" {
				t.Errorf("certificate storage integration missing AmbiguousTimeoutPlan")
			}
		}
	}
	if !foundEmailIntegration {
		t.Errorf("expected email provider idempotency specification")
	}
	if !foundStorageIntegration {
		t.Errorf("expected certificate storage idempotency specification")
	}

	// Regression checks: Exact registration and attendance uniqueness differentiation
	if !strings.Contains(contract.Data.UniquenessStrategy, "UNIQUE(email)") {
		t.Errorf("expected UniquenessStrategy to specify UNIQUE(email) for registrations, got: %s", contract.Data.UniquenessStrategy)
	}
	if !strings.Contains(contract.Data.UniquenessStrategy, "UNIQUE(delegate_id, session_id)") {
		t.Errorf("expected UniquenessStrategy to specify UNIQUE(delegate_id, session_id) for attendance, got: %s", contract.Data.UniquenessStrategy)
	}

	// Regression checks: Explicit idempotency conflict handling (no 'A or B' ambiguity)
	if strings.Contains(contract.Data.IdempotencyPolicy.ConcurrentRequestHandling, " or ") {
		t.Errorf("ConcurrentRequestHandling must not contain ambiguous 'or' clauses: %s", contract.Data.IdempotencyPolicy.ConcurrentRequestHandling)
	}
	if !strings.Contains(contract.Data.IdempotencyPolicy.ConcurrentRequestHandling, "409") {
		t.Errorf("expected HTTP 409 in ConcurrentRequestHandling: %s", contract.Data.IdempotencyPolicy.ConcurrentRequestHandling)
	}
	if strings.Contains(contract.Data.IdempotencyPolicy.RetentionPeriod, "to") {
		t.Errorf("RetentionPeriod must be explicit rather than a range: %s", contract.Data.IdempotencyPolicy.RetentionPeriod)
	}

	// Regression checks: Physical arrival vs HTTP RPS semantics
	if resp.Normalized.Traffic.TrafficSource != "DERIVED_FROM_ARRIVALS" {
		t.Errorf("expected TrafficSource DERIVED_FROM_ARRIVALS, got %s", resp.Normalized.Traffic.TrafficSource)
	}
	if resp.Normalized.Traffic.ArrivalsPerSec < 0.20 || resp.Normalized.Traffic.ArrivalsPerSec > 0.30 {
		t.Errorf("expected physical arrival rate ~0.22 arr/sec, got %0.2f", resp.Normalized.Traffic.ArrivalsPerSec)
	}
	if resp.Normalized.Traffic.EstimatedHTTPRPS != 15.0 {
		t.Errorf("expected estimated HTTP RPS 15.0, got %0.1f", resp.Normalized.Traffic.EstimatedHTTPRPS)
	}
}

// 1b. Assumed default traffic must not trigger TraitBurstyTraffic
func TestAssumedDefaultTrafficDoesNotTriggerBurstyTrait(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	prompt := `Build a simple conference schedule viewer for attendees to browse speaker bios and talk abstracts.`
	resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	if resp.Normalized.Traffic.TrafficSource != "ASSUMED_DEFAULT" {
		t.Errorf("expected ASSUMED_DEFAULT, got %s", resp.Normalized.Traffic.TrafficSource)
	}

	if resp.Classification.HasTrait(model.TraitBurstyTraffic) {
		t.Errorf("TraitBurstyTraffic must not be activated based solely on default 10:30 RPS ratio")
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

// 9. Comprehensive Pipeline Invariant & Regression Hardening
func TestStage5QualityAndRegressionHardening(t *testing.T) {
	eng := newTestEngine()
	ctx := context.Background()

	t.Run("explicit peak RPS survives pipeline without heuristic overwrite", func(t *testing.T) {
		prompt := "Micro-auction bidding engine with baseline of 60 RPS and explicit peak of 95 RPS under burst bidding. Must prevent double bids."
		resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
		if err != nil {
			t.Fatalf("RecommendArchitecture failed: %v", err)
		}

		if resp.Normalized.Traffic.ExpectedRPS != 60.0 {
			t.Errorf("expected baseline 60.0, got %v", resp.Normalized.Traffic.ExpectedRPS)
		}
		if resp.Normalized.Traffic.PeakRPS != 95.0 {
			t.Errorf("expected explicit peak 95.0, got %v", resp.Normalized.Traffic.PeakRPS)
		}
		if !resp.Normalized.Traffic.IsPeakExplicit {
			t.Errorf("expected IsPeakExplicit to be true")
		}
		if resp.Contract.CapacityPlan.PlanningArrivalRate != 95.0 {
			t.Errorf("expected CapacityPlan planning rate 95.0, got %v", resp.Contract.CapacityPlan.PlanningArrivalRate)
		}
	})

	t.Run("gap detector assumptions and unresolved decisions reach final contract", func(t *testing.T) {
		prompt := "Inventory service for regional warehouses."
		resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
		if err != nil {
			t.Fatalf("RecommendArchitecture failed: %v", err)
		}

		if len(resp.Contract.ActiveAssumptions) == 0 {
			t.Errorf("expected active assumptions in contract for unspecified input")
		}
		if len(resp.Gaps.Questions) > 0 && len(resp.Contract.UnresolvedDecisions) == 0 {
			t.Errorf("expected unresolved decisions from gaps to reach contract")
		}
	})

	t.Run("ecommerce domain does not leak conference terms in decision traces", func(t *testing.T) {
		prompt := "E-commerce checkout service with payment capture, order placement, and inventory decrement. Prevent duplicate orders and retry asynchronously."
		resp, err := eng.RecommendArchitecture(ctx, model.RawRequirement{Prompt: prompt})
		if err != nil {
			t.Fatalf("RecommendArchitecture failed: %v", err)
		}

		for _, trace := range resp.Contract.DecisionTraces {
			fullTrace := trace.Decision + " " + trace.EvidenceAndProvenance + " " + strings.Join(trace.SupportingRequirements, " ") + " " + strings.Join(trace.RejectedReasons, " ")
			lower := strings.ToLower(fullTrace)
			for _, forbidden := range []string{"volunteer", "certificate", "delegate", "attendance"} {
				if strings.Contains(lower, forbidden) {
					t.Errorf("decision trace leaked forbidden term %q: %s", forbidden, fullTrace)
				}
			}
		}
	})

	t.Run("validator rejects nonstandard paths and catches bypass attempts", func(t *testing.T) {
		contract := &model.ArchitectureContract{
			ContractVersion: "1.0.0",
			ProjectName:     "TestApp",
			Data: model.DataRules{
				RequiresIdempotencyKey: true,
				RequiresOutboxPattern:  true,
				UniquenessStrategy:     "UNIQUE(user_id, account_id)",
			},
			Queue: model.QueueRules{
				Enabled:        true,
				QueueMechanism: "POSTGRES_SKIP_LOCKED",
			},
		}

		val := eng.ValidateArchitecture(contract, validator.ImplementationProposal{
			Files: []validator.CodeFileProposal{
				{
					Path: "routes/orders.go", // nonstandard path outside handler/controller
					Content: `package routes
func SubmitOrder(w http.ResponseWriter, r *http.Request) {
	// workerpool comment attempt to fool scanner
	go func() {
		saveOrder()
	}()
}`,
				},
			},
		})

		foundAST := false
		for _, v := range val.Violations {
			if v.RuleID == "VIOL-AST-UNBOUNDED-GOROUTINE" {
				foundAST = true
				break
			}
		}
		if !foundAST {
			t.Errorf("expected VIOL-AST-UNBOUNDED-GOROUTINE to catch unbounded goroutine in routes/orders.go")
		}
	})
}
