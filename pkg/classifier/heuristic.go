package classifier

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sdskill/archintel/pkg/model"
)

const (
	// DefaultActivationThreshold is the minimum score required to activate a trait.
	DefaultActivationThreshold = 0.65
)

// RuleBasedClassifier evaluates workload traits deterministically using domain rules and metrics.
type RuleBasedClassifier struct {
	threshold float64
}

// NewRuleBasedClassifier creates a deterministic heuristic classifier.
func NewRuleBasedClassifier() *RuleBasedClassifier {
	return &RuleBasedClassifier{
		threshold: DefaultActivationThreshold,
	}
}

// Classify deterministically evaluates multi-label workload characteristics.
func (c *RuleBasedClassifier) Classify(ctx context.Context, req *model.NormalizedRequirements) (*model.ClassificationResult, error) {
	start := time.Now()

	scores := make(map[model.WorkloadTrait]model.ClassificationScore)

	// 1. Transaction Heavy
	txScore, txRationale := evaluateTransactionHeavy(req)
	scores[model.TraitTransactionHeavy] = model.ClassificationScore{
		Trait:     model.TraitTransactionHeavy,
		Score:     txScore,
		IsActive:  txScore >= c.threshold,
		Rationale: txRationale,
	}

	// 2. Read Heavy
	readScore, readRationale := evaluateReadHeavy(req)
	scores[model.TraitReadHeavy] = model.ClassificationScore{
		Trait:     model.TraitReadHeavy,
		Score:     readScore,
		IsActive:  readScore >= c.threshold,
		Rationale: readRationale,
	}

	// 3. Write Heavy
	writeScore, writeRationale := evaluateWriteHeavy(req)
	scores[model.TraitWriteHeavy] = model.ClassificationScore{
		Trait:     model.TraitWriteHeavy,
		Score:     writeScore,
		IsActive:  writeScore >= c.threshold,
		Rationale: writeRationale,
	}

	// 4. Latency Sensitive
	latScore, latRationale := evaluateLatencySensitive(req)
	scores[model.TraitLatencySensitive] = model.ClassificationScore{
		Trait:     model.TraitLatencySensitive,
		Score:     latScore,
		IsActive:  latScore >= c.threshold,
		Rationale: latRationale,
	}

	// 5. Background Processing Intensive
	bgScore, bgRationale := evaluateBackgroundIntensive(req)
	scores[model.TraitBackgroundIntensive] = model.ClassificationScore{
		Trait:     model.TraitBackgroundIntensive,
		Score:     bgScore,
		IsActive:  bgScore >= c.threshold,
		Rationale: bgRationale,
	}

	// 6. Consistency Sensitive
	consScore, consRationale := evaluateConsistencySensitive(req)
	scores[model.TraitConsistencySensitive] = model.ClassificationScore{
		Trait:     model.TraitConsistencySensitive,
		Score:     consScore,
		IsActive:  consScore >= c.threshold,
		Rationale: consRationale,
	}

	// 7. Bursty Traffic
	burstScore, burstRationale := evaluateBurstyTraffic(req)
	scores[model.TraitBurstyTraffic] = model.ClassificationScore{
		Trait:     model.TraitBurstyTraffic,
		Score:     burstScore,
		IsActive:  burstScore >= c.threshold,
		Rationale: burstRationale,
	}

	// 8. High Contention
	contScore, contRationale := evaluateHighContention(req)
	scores[model.TraitHighContention] = model.ClassificationScore{
		Trait:     model.TraitHighContention,
		Score:     contScore,
		IsActive:  contScore >= c.threshold,
		Rationale: contRationale,
	}

	// Collect active traits and identify the primary (highest scoring) trait.
	var activeTraits []model.WorkloadTrait
	var primaryTrait model.WorkloadTrait
	var maxScore float64

	for trait, sc := range scores {
		if sc.IsActive {
			activeTraits = append(activeTraits, trait)
			if sc.Score > maxScore {
				maxScore = sc.Score
				primaryTrait = trait
			}
		}
	}

	// If no trait passed threshold, default primary trait to CRUD / READ_HEAVY.
	if primaryTrait == "" {
		primaryTrait = model.TraitReadHeavy
	}

	elapsed := time.Since(start).Milliseconds()

	return &model.ClassificationResult{
		Source:             model.ClassifierSourceRuleBased,
		ModelVersion:       "v1.0.0-deterministic",
		InferenceLatencyMs: elapsed,
		Scores:             scores,
		ActiveTraits:       activeTraits,
		PrimaryTrait:       primaryTrait,
		FallbackApplied:    false,
	}, nil
}

func evaluateTransactionHeavy(req *model.NormalizedRequirements) (float64, string) {
	if req.Domain == model.DomainFinTech {
		return 0.95, "FinTech domains require strict ACID transactions and double-entry invariants"
	}
	if req.Domain == model.DomainECommerce && req.Data.RequiresACID {
		return 0.90, "E-Commerce checkout with ACID inventory and payment coordination"
	}
	if req.Data.RequiresACID && req.Data.AtomicOutboxRequired {
		return 0.85, "Explicit requirement for multi-step ACID mutations and atomic transactional outbox dispatch"
	}
	if req.Data.RequiresACID {
		return 0.80, "Explicit requirement for multi-row ACID database transactions"
	}
	return 0.20, "Workload does not indicate high transaction isolation requirements"
}

func evaluateReadHeavy(req *model.NormalizedRequirements) (float64, string) {
	if req.Traffic.ReadWriteRatio >= 10.0 {
		return 0.95, "Read-to-write ratio >= 10:1; cache-friendly query workload"
	}
	if req.Traffic.ReadWriteRatio >= 5.0 {
		return 0.80, "Read operations strongly dominate writes (ratio >= 5:1)"
	}
	if req.Domain == model.DomainContentMedia {
		return 0.75, "Content/Media catalog typical reads exceed writes"
	}
	return 0.30, "Read/write volume is balanced or write-skewed"
}

func evaluateWriteHeavy(req *model.NormalizedRequirements) (float64, string) {
	if req.Domain == model.DomainIoTTelemetry {
		return 0.95, "IoT / Telemetry ingestion has persistent high-frequency writes"
	}
	if req.Traffic.ReadWriteRatio <= 0.5 && req.Traffic.ReadWriteRatio > 0.0 {
		return 0.90, "Write-to-read ratio >= 2:1 indicates write-heavy stream"
	}
	if req.IsWriteHeavy() {
		return 0.75, "Writes meet or exceed read traffic"
	}
	return 0.15, "Standard read-skewed traffic distribution"
}

func evaluateLatencySensitive(req *model.NormalizedRequirements) (float64, string) {
	if req.Latency.P99TargetMs <= 50 {
		return 0.95, "Strict sub-50ms P99 latency SLA requires low-overhead paths"
	}
	if req.Latency.P99TargetMs <= 100 {
		return 0.85, "Sub-100ms P99 latency requires cached reads and asynchronous side-effects"
	}
	if req.Domain == model.DomainRealtimeChat {
		return 0.80, "Real-time communication requires low-latency packet delivery"
	}
	return 0.25, "Standard acceptable web API response budget"
}

func evaluateBackgroundIntensive(req *model.NormalizedRequirements) (float64, string) {
	if len(req.Invariants.AsyncOperations) > 0 {
		return 0.85, fmt.Sprintf("Explicit asynchronous operations requested: %s", strings.Join(req.Invariants.AsyncOperations, ", "))
	}
	if req.Domain == model.DomainAsyncPipeline || req.Domain == model.DomainAnalytics {
		return 0.90, "Pipeline or analytics workload dominated by async batch execution"
	}
	if req.Domain == model.DomainECommerce || req.Domain == model.DomainFinTech {
		return 0.70, "Commerce and financial workflows require async outbox dispatchers and reconciliations"
	}
	return 0.20, "Workload is predominantly synchronous request-response"
}

func evaluateConsistencySensitive(req *model.NormalizedRequirements) (float64, string) {
	if req.Invariants.PreventDuplicates {
		return 0.95, "Explicit invariant: duplicate prevention required on authoritative attendance/registration records"
	}
	if req.Consistency == model.ConsistencyStrong || req.Consistency == model.ConsistencyLinearizable {
		return 0.95, "Strong or linearizable consistency explicitly mandated"
	}
	if req.Domain == model.DomainFinTech || req.Data.HasHighContention {
		return 0.90, "Financial transactions or contention risk require immediate consistency"
	}
	return 0.30, "Eventual consistency is sufficient for this workload"
}

func evaluateBurstyTraffic(req *model.NormalizedRequirements) (float64, string) {
	if req.Traffic.IsBursty || req.Traffic.PeakArrivalWindow != "" {
		windowInfo := ""
		if req.Traffic.PeakArrivalWindow != "" {
			windowInfo = fmt.Sprintf(" (%s)", req.Traffic.PeakArrivalWindow)
		}
		return 0.90, fmt.Sprintf("Traffic spikes and concentrated arrival windows explicitly documented%s", windowInfo)
	}
	// Do not declare burstiness based on assumed defaults (e.g. 10 vs 30 RPS default ratio)
	if req.Traffic.TrafficSource != "ASSUMED_DEFAULT" && req.Traffic.PeakRPS >= req.Traffic.ExpectedRPS*3.0 && req.Traffic.PeakRPS > 50 {
		return 0.75, fmt.Sprintf("Peak throughput (%0.1f RPS) exceeds baseline (%0.1f RPS) significantly based on user-stated figures", req.Traffic.PeakRPS, req.Traffic.ExpectedRPS)
	}
	return 0.20, "Traffic pattern is steady or unmeasured; burstiness not indicated"
}

func evaluateHighContention(req *model.NormalizedRequirements) (float64, string) {
	if req.Data.HasHighContention {
		return 0.95, "Shared mutable state (inventory/seats/balances) accessed concurrently"
	}
	if req.Invariants.RequiresIdempotency && req.Invariants.PreventDuplicates {
		return 0.75, "Concurrent check-in/scan operations from multiple devices target shared attendance records"
	}
	return 0.15, "Requests operate on isolated, user-owned entities without contention"
}
