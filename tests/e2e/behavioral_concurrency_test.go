package e2e

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sdskill/archintel/pkg/model"
	"github.com/sdskill/archintel/pkg/validator"
)

// A. International Conference: Concurrent Check-In Duplicate Prevention
func TestSimulatedConcurrentCheckIns(t *testing.T) {
	// In-memory model of database compound unique constraint: UNIQUE(delegate_id, session_id)
	type AttendanceRecord struct {
		DelegateID string
		SessionID  string
		CreatedAt  time.Time
	}

	var mu sync.Mutex
	attendanceTable := make(map[string]AttendanceRecord) // key: delegate_id + ":" + session_id

	insertAttendanceOnConflictDoNothing := func(delegateID, sessionID string) bool {
		mu.Lock()
		defer mu.Unlock()

		key := delegateID + ":" + sessionID
		if _, exists := attendanceTable[key]; exists {
			return false // Conflict detected: DO NOTHING
		}

		attendanceTable[key] = AttendanceRecord{
			DelegateID: delegateID,
			SessionID:  sessionID,
			CreatedAt:  time.Now(),
		}
		return true // Inserted successfully
	}

	// 1. Concurrent check-ins for the same delegate and session (simulate 50 simultaneous scans)
	const concurrentScans = 50
	var wg sync.WaitGroup
	var successCount int64
	var duplicateCount int64

	for i := 0; i < concurrentScans; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if insertAttendanceOnConflictDoNothing("del-101", "session-01") {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&duplicateCount, 1)
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful check-in for del-101 in session-01, got %d", successCount)
	}
	if duplicateCount != concurrentScans-1 {
		t.Fatalf("expected %d duplicate rejections, got %d", concurrentScans-1, duplicateCount)
	}

	// 2. Concurrent check-ins for DIFFERENT sessions for the same delegate
	var diffSessionSuccess int64
	wg = sync.WaitGroup{}
	sessions := []string{"session-02", "session-03", "session-04", "session-05"}
	for _, sess := range sessions {
		sessID := sess
		wg.Add(1)
		go func() {
			defer wg.Done()
			if insertAttendanceOnConflictDoNothing("del-101", sessID) {
				atomic.AddInt64(&diffSessionSuccess, 1)
			}
		}()
	}
	wg.Wait()

	if diffSessionSuccess != int64(len(sessions)) {
		t.Fatalf("expected %d successful check-ins across distinct sessions, got %d", len(sessions), diffSessionSuccess)
	}
}

// A2. International Conference: Atomic Attendance and Outbox Persistence
func TestSimulatedAtomicAttendanceAndOutbox(t *testing.T) {
	type OutboxEvent struct {
		ID        string
		EventType string
		Payload   string
		Status    string
	}

	type DBState struct {
		mu         sync.Mutex
		Attendance map[string]string
		Outbox     map[string]OutboxEvent
	}

	state := &DBState{
		Attendance: make(map[string]string),
		Outbox:     make(map[string]OutboxEvent),
	}

	// Transaction function simulating atomic commit vs rollback
	performCheckInTx := func(delegateID, sessionID, eventID string, failTx bool) error {
		state.mu.Lock()
		defer state.mu.Unlock()

		// Staging transaction state
		if failTx {
			// Simulating DB error (e.g. disk or constraint violation rollback)
			return errors.New("simulated database transaction abort")
		}

		key := delegateID + ":" + sessionID
		state.Attendance[key] = "CONFIRMED"
		state.Outbox[eventID] = OutboxEvent{
			ID:        eventID,
			EventType: "DELEGATE_CHECKED_IN",
			Payload:   fmt.Sprintf(`{"delegate_id":"%s","session_id":"%s"}`, delegateID, sessionID),
			Status:    "PENDING",
		}
		return nil
	}

	// Test 1: Failed transaction leaves zero orphan attendance or outbox records
	err := performCheckInTx("del-201", "session-01", "evt-001", true)
	if err == nil {
		t.Fatal("expected transaction error, got nil")
	}
	if len(state.Attendance) != 0 || len(state.Outbox) != 0 {
		t.Fatalf("atomic rollback failed: state has attendance=%d, outbox=%d", len(state.Attendance), len(state.Outbox))
	}

	// Test 2: Successful transaction commits both atomically
	err = performCheckInTx("del-201", "session-01", "evt-002", false)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if state.Attendance["del-201:session-01"] != "CONFIRMED" {
		t.Fatal("expected attendance to be confirmed")
	}
	if state.Outbox["evt-002"].Status != "PENDING" {
		t.Fatal("expected outbox event to be pending")
	}
}

// A3. International Conference: Outbox Worker Recovery and Dead-Letter Handling
func TestSimulatedOutboxWorkerRecoveryAndDLQ(t *testing.T) {
	type Job struct {
		ID          string
		Attempts    int
		MaxAttempts int
		Status      string // PENDING, PROCESSED, DEAD_LETTER
	}

	queue := map[string]*Job{
		"job-1": {ID: "job-1", Attempts: 0, MaxAttempts: 3, Status: "PENDING"},
		"job-2": {ID: "job-2", Attempts: 2, MaxAttempts: 3, Status: "PENDING"}, // will exhaust retries
	}

	processJob := func(j *Job, simulatePermanentFailure bool) {
		j.Attempts++
		if simulatePermanentFailure {
			if j.Attempts >= j.MaxAttempts {
				j.Status = "DEAD_LETTER"
			}
			return
		}
		j.Status = "PROCESSED"
	}

	// Simulate worker processing job-1 successfully
	processJob(queue["job-1"], false)
	if queue["job-1"].Status != "PROCESSED" {
		t.Fatalf("expected job-1 to be PROCESSED, got %s", queue["job-1"].Status)
	}

	// Simulate worker processing job-2 with failure, exhausting max retries
	processJob(queue["job-2"], true)
	if queue["job-2"].Status != "DEAD_LETTER" {
		t.Fatalf("expected job-2 to be routed to DEAD_LETTER after retry exhaustion, got %s", queue["job-2"].Status)
	}
}

// B. E-Commerce: Payment Idempotency Key Reuse and Payload Mismatch
func TestSimulatedPaymentIdempotency(t *testing.T) {
	type IdempotencyRecord struct {
		PayloadHash string
		Response    string
	}

	var mu sync.Mutex
	idempotencyStore := make(map[string]IdempotencyRecord)

	processPayment := func(key string, payloadHash string, amount int) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		if rec, exists := idempotencyStore[key]; exists {
			if rec.PayloadHash != payloadHash {
				return "", errors.New("idempotency key reused with conflicting request payload")
			}
			return rec.Response, nil // Return cached response
		}

		// Perform charge
		resp := fmt.Sprintf("CHARGE_SUCCESS_USD_%d", amount)
		idempotencyStore[key] = IdempotencyRecord{
			PayloadHash: payloadHash,
			Response:    resp,
		}
		return resp, nil
	}

	// Initial request
	res1, err := processPayment("idem-key-100", "hash-order-555", 150)
	if err != nil || res1 != "CHARGE_SUCCESS_USD_150" {
		t.Fatalf("initial charge failed: %v, res: %s", err, res1)
	}

	// Exact retry returns cached success without recharging
	res2, err := processPayment("idem-key-100", "hash-order-555", 150)
	if err != nil || res2 != "CHARGE_SUCCESS_USD_150" {
		t.Fatalf("idempotent retry failed: %v, res: %s", err, res2)
	}

	// Reusing key with different payload is rejected as conflict
	_, err = processPayment("idem-key-100", "hash-different-order", 500)
	if err == nil {
		t.Fatal("expected error on idempotency key payload conflict, got nil")
	}
}

// C. Flash Sale: Atomic Inventory Contention and Overselling Prevention
func TestSimulatedFlashSaleAtomicReservation(t *testing.T) {
	var availableStock int64 = 20 // Limited stock of 20 items
	const totalShoppers = 100

	var successfulReservations int64
	var soldOutRejections int64

	var wg sync.WaitGroup
	for i := 0; i < totalShoppers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Atomic decrement: UPDATE inventory SET stock = stock - 1 WHERE id = 1 AND stock > 0
			for {
				current := atomic.LoadInt64(&availableStock)
				if current <= 0 {
					atomic.AddInt64(&soldOutRejections, 1)
					return
				}
				if atomic.CompareAndSwapInt64(&availableStock, current, current-1) {
					atomic.AddInt64(&successfulReservations, 1)
					return
				}
			}
		}()
	}
	wg.Wait()

	if successfulReservations != 20 {
		t.Fatalf("expected exactly 20 items reserved, got %d", successfulReservations)
	}
	if soldOutRejections != 80 {
		t.Fatalf("expected 80 shoppers rejected, got %d", soldOutRejections)
	}
	if availableStock != 0 {
		t.Fatalf("expected remaining stock to be 0, got %d", availableStock)
	}
}

// F. Validator: Detection of Anti-Patterns in Go Code
func TestValidatorDetectsViolations(t *testing.T) {
	v := validator.New()

	contract := &model.ArchitectureContract{
		ContractVersion: "1.0.0",
		ProjectName:     "TestApp",
		Topology: model.SystemTopology{
			Type: model.TopologyModularMonolith,
		},
		Data: model.DataRules{
			RequiresOutboxPattern:  true,
			RequiresACID:           true,
			RequiresIdempotencyKey: true,
			UniquenessStrategy:     "UNIQUE(delegate_id, session_id)",
		},
		Queue: model.QueueRules{
			Enabled:              true,
			QueueMechanism:       "POSTGRES_SKIP_LOCKED",
			MaxWorkerConcurrency: 10,
		},
	}

	// Bad code proposal containing HTTP call inside tx and unbounded goroutine
	badProposal := validator.ImplementationProposal{
		Files: []validator.CodeFileProposal{
			{
				Path: "handlers/checkout_handler.go",
				Content: `package handlers

import (
	"net/http"
	"database/sql"
)

func CheckoutHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	tx, _ := db.Begin()
	defer tx.Rollback()

	// ANTI-PATTERN 1: External HTTP call inside DB transaction
	http.Post("https://payment.gateway.internal/charge", "application/json", nil)

	// ANTI-PATTERN 2: Direct unconstrained goroutine in handler
	go func() {
		println("async task")
	}()

	tx.Commit()
}
`,
			},
		},
	}

	report := v.Validate(contract, badProposal)
	if report.IsCompliant {
		t.Fatal("expected validator to flag anti-patterns, but reported compliant")
	}

	foundTxHTTP := false
	foundASTGoroutine := false
	for _, viol := range report.Violations {
		if viol.RuleID == "VIOL-SYNC-HTTP-IN-TX" {
			foundTxHTTP = true
		}
		if viol.RuleID == "VIOL-AST-UNBOUNDED-GOROUTINE" || viol.RuleID == "VIOL-UNBOUNDED-HANDLER-GOROUTINE" {
			foundASTGoroutine = true
		}
	}

	if !foundTxHTTP {
		t.Errorf("expected VIOL-SYNC-HTTP-IN-TX violation to be caught")
	}
	if !foundASTGoroutine {
		t.Errorf("expected unbounded goroutine violation to be caught")
	}
}

// G. Offline Scan Identity, Retries, and Audit Log Invariants
func TestOfflineScanIdentityAndRetries(t *testing.T) {
	// Models the three distinct identities:
	// 1. ScanEventID: stable UUID per physical scan
	// 2. TransportIdempotencyKey: HTTP request token per network upload attempt
	// 3. CanonicalAttendanceIdentity: UNIQUE(delegate_id, session_id)

	type ScanEventRecord struct {
		ScanEventID  string
		DelegateID   string
		SessionID    string
		DeviceID     string
		ClientScanAt time.Time
		ServerSyncAt time.Time
		Result       string // FIRST_SEEN, DUPLICATE_SCAN_EVENT, CONFLICTING_ATTENDANCE
	}

	type AttendanceRecord struct {
		DelegateID  string
		SessionID   string
		FirstScanID string
		AdmittedAt  time.Time
	}

	var mu sync.Mutex
	scanEventsLog := make(map[string]ScanEventRecord) // Key: ScanEventID
	attendanceDB := make(map[string]AttendanceRecord) // Key: DelegateID + ":" + SessionID

	// Process an incoming scan upload
	processScanUpload := func(scanID, delID, sessID, deviceID string, clientTime time.Time) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		now := time.Now()

		// 1. Check if this exact physical scan was already received (transport retry / lost response)
		if existing, seen := scanEventsLog[scanID]; seen {
			// Repeated transmission of the same physical scan MUST NOT create duplicate audit record
			return existing.Result, nil
		}

		// Record the scan in audit log
		canonKey := delID + ":" + sessID
		if _, alreadyAdmitted := attendanceDB[canonKey]; alreadyAdmitted {
			// Distinct physical scan attempt for delegate who already checked in
			scanEventsLog[scanID] = ScanEventRecord{
				ScanEventID:  scanID,
				DelegateID:   delID,
				SessionID:    sessID,
				DeviceID:     deviceID,
				ClientScanAt: clientTime,
				ServerSyncAt: now,
				Result:       "CONFLICTING_ATTENDANCE",
			}
			return "CONFLICTING_ATTENDANCE", nil
		}

		// First admission
		attendanceDB[canonKey] = AttendanceRecord{
			DelegateID:  delID,
			SessionID:   sessID,
			FirstScanID: scanID,
			AdmittedAt:  now,
		}
		scanEventsLog[scanID] = ScanEventRecord{
			ScanEventID:  scanID,
			DelegateID:   delID,
			SessionID:    sessID,
			DeviceID:     deviceID,
			ClientScanAt: clientTime,
			ServerSyncAt: now,
			Result:       "FIRST_SEEN",
		}
		return "FIRST_SEEN", nil
	}

	// 1. Initial scan: succeeds
	tClient1 := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	res1, err := processScanUpload("scan-uuid-001", "del-500", "sess-100", "dev-A", tClient1)
	if err != nil || res1 != "FIRST_SEEN" {
		t.Fatalf("expected FIRST_SEEN, got res=%s, err=%v", res1, err)
	}

	// 2. Retry of SAME physical scan (e.g. volunteer device lost HTTP 200 response and retries upload)
	res2, err := processScanUpload("scan-uuid-001", "del-500", "sess-100", "dev-A", tClient1)
	if err != nil || res2 != "FIRST_SEEN" {
		t.Fatalf("expected idempotent response on retry of same scan_event_id, got res=%s", res2)
	}
	// Verify audit log did not duplicate or corrupt record
	if len(scanEventsLog) != 1 {
		t.Fatalf("expected audit log to hold exactly 1 entry for duplicate delivery of same scan, got %d", len(scanEventsLog))
	}

	// 3. Distinct physical scan attempt for SAME delegate and session (e.g. scanned at different door later)
	tClient2 := time.Date(2026, 10, 10, 9, 5, 0, 0, time.UTC)
	res3, err := processScanUpload("scan-uuid-002", "del-500", "sess-100", "dev-B", tClient2)
	if err != nil || res3 != "CONFLICTING_ATTENDANCE" {
		t.Fatalf("expected CONFLICTING_ATTENDANCE for distinct scan, got res=%s", res3)
	}
	// Verify both physical scans are preserved in audit log for administrative verification
	if len(scanEventsLog) != 2 {
		t.Fatalf("expected audit log to hold both distinct physical scans, got %d", len(scanEventsLog))
	}
	// Verify canonical attendance holds exactly 1 record
	if len(attendanceDB) != 1 {
		t.Fatalf("canonical attendance invariant violated: expected 1 record, got %d", len(attendanceDB))
	}

	// 4. Concurrent uploads of the SAME physical scan ID (simulate simultaneous network flush)
	const concurrentRetries = 20
	var wg sync.WaitGroup
	var initialSuccesses int64
	for i := 0; i < concurrentRetries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := processScanUpload("scan-uuid-003", "del-600", "sess-100", "dev-C", tClient1)
			if e == nil && r == "FIRST_SEEN" {
				atomic.AddInt64(&initialSuccesses, 1)
			}
		}()
	}
	wg.Wait()
	// All retries must succeed idempotently
	if initialSuccesses != concurrentRetries {
		t.Fatalf("expected all %d concurrent retries of same scan to return success, got %d", concurrentRetries, initialSuccesses)
	}
	if len(attendanceDB) != 2 {
		t.Fatalf("expected 2 unique admitted delegates, got %d", len(attendanceDB))
	}
}

// H. External Side-Effect Delivery, Worker Crash, and Provider Idempotency
func TestExternalSideEffectDeliveryAndWorkerCrash(t *testing.T) {
	// Models:
	// - Local outbox table (with PENDING, IN_PROGRESS, PROCESSED)
	// - External Provider (SendGrid / Stripe) with provider-side idempotency cache
	// - Worker crashing:
	//   Case 1: Crash before dispatch
	//   Case 2: Crash after dispatch succeeds on external provider, but before local DB marks PROCESSED

	type OutboxRow struct {
		ID        string
		Payload   string
		Status    string // PENDING, IN_PROGRESS, PROCESSED
		ClaimedAt time.Time
	}

	type ExternalProviderCall struct {
		IdempotencyToken string
		Body             string
	}

	var mu sync.Mutex
	outboxTable := map[string]*OutboxRow{
		"event-email-1": {ID: "event-email-1", Payload: "send welcome email", Status: "PENDING"},
	}

	providerCalls := make([]ExternalProviderCall, 0)
	providerProcessedTokens := make(map[string]string) // provider idempotency table

	callExternalProvider := func(idempotencyToken, body string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		providerCalls = append(providerCalls, ExternalProviderCall{
			IdempotencyToken: idempotencyToken,
			Body:             body,
		})

		// Provider-side idempotency: if already seen token, return cached response without duplicate side-effect!
		if resp, seen := providerProcessedTokens[idempotencyToken]; seen {
			return resp, nil // Deduplicated by external provider
		}

		// First execution
		resp := "EMAIL_SENT_ID_" + idempotencyToken
		providerProcessedTokens[idempotencyToken] = resp
		return resp, nil
	}

	// Worker run loop
	dispatchOutbox := func(eventID string, crashAfterDispatch bool) error {
		mu.Lock()
		row := outboxTable[eventID]
		row.Status = "IN_PROGRESS"
		payload := row.Payload
		token := row.ID // Using deterministic event ID as provider idempotency token
		mu.Unlock()

		// Step 2: Call external provider
		_, err := callExternalProvider(token, payload)
		if err != nil {
			return err
		}

		// Step 3: Simulate worker crash after provider success but BEFORE DB status update
		if crashAfterDispatch {
			return errors.New("WORKER_PROCESS_CRASHED")
		}

		// Step 4: Record success in local DB
		mu.Lock()
		row.Status = "PROCESSED"
		mu.Unlock()
		return nil
	}

	// 1. Worker attempt 1: dispatches successfully to provider, but crashes before DB commit!
	err1 := dispatchOutbox("event-email-1", true)
	if err1 == nil || err1.Error() != "WORKER_PROCESS_CRASHED" {
		t.Fatalf("expected simulated worker crash, got: %v", err1)
	}

	// In local DB, status remains IN_PROGRESS / uncommitted
	if outboxTable["event-email-1"].Status == "PROCESSED" {
		t.Fatal("status should not be PROCESSED because worker crashed before local commit")
	}

	// 2. Recovery: Next worker picks up unacknowledged job and retries dispatch
	err2 := dispatchOutbox("event-email-1", false)
	if err2 != nil {
		t.Fatalf("recovery dispatch failed: %v", err2)
	}

	// Now status in local DB is PROCESSED
	if outboxTable["event-email-1"].Status != "PROCESSED" {
		t.Fatalf("expected outbox row to be PROCESSED after recovery, got %s", outboxTable["event-email-1"].Status)
	}

	// Verify provider was invoked twice across the wire (AT_LEAST_ONCE dispatch)
	if len(providerCalls) != 2 {
		t.Fatalf("expected 2 transport attempts to provider, got %d", len(providerCalls))
	}

	// BUT verify provider executed the actual side effect exactly ONCE due to provider idempotency token!
	if len(providerProcessedTokens) != 1 {
		t.Fatalf("provider side-effect duplicated: expected 1 unique token executed, got %d", len(providerProcessedTokens))
	}
}
