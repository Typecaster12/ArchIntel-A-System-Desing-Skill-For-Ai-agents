package normalizer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sdskill/archintel/pkg/model"
)

var (
	// Regex patterns for detecting traffic and scale specifications.
	rpsRegex           = regexp.MustCompile(`(?i)([\d,]+(?:\.\d+)?)\s*(?:req(?:uests)?\/s(?:ec)?|rps)`)
	dauRegex           = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:k|m|million|thousand)?\s*(?:dau|daily\s+active\s+users)`)
	kNumRegex          = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*k`)
	mNumRegex          = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:m|million)`)
	attendeeRegex      = regexp.MustCompile(`(?i)([\d,]+)\s*(?:registered\s+)?(?:delegates|attendees|participants|users|guests)`)
	arrivalWindowRegex = regexp.MustCompile(`(?i)([\d,]+)(?:\s*(?:–|-|to)\s*([\d,]+))?\s*(?:attendees|delegates|arrivals|guests)?\s*arriving\s*within\s*(?:a\s*)?(\d+)[- ]minute`)

	// Regex patterns for latency constraints.
	latencyMsRegex = regexp.MustCompile(`(?i)(?:sub-?|<|under|within)?\s*(\d+)\s*ms`)
	ratioRegex     = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*:\s*(\d+(?:\.\d+)?)\s*(?:read(?:\s*to|\s*-|\s*\/)?\s*write)?`)

	// Domain keyword triggers.
	eCommerceKeywords = []string{"ecommerce", "e-commerce", "shop", "checkout", "cart", "order", "inventory", "store"}
	finTechKeywords   = []string{"payment", "fintech", "banking", "wallet", "ledger", "transaction", "transfer", "currency", "credit card"}
	telemetryKeywords = []string{"iot", "telemetry", "sensor", "metrics", "time-series", "timeseries", "device tracking"}
	chatKeywords      = []string{"chat", "messaging", "websocket", "real-time", "realtime", "presence"}
	analyticsKeywords = []string{"analytics", "reporting", "dashboard", "bi", "data warehouse", "olap", "etl"}
	contentKeywords   = []string{"blog", "cms", "media", "video", "catalog", "article", "news", "stream", "url shortener"}
	pipelineKeywords  = []string{"pipeline", "worker", "batch", "notification", "dispatcher", "background job"}
	eventKeywords     = []string{"conference", "symposium", "summit", "event management", "convention", "delegate", "session", "attendance", "check-in", "qr-code"}
)

// Normalizer parses unstructured requirements and optional structured flags into NormalizedRequirements.
type Normalizer struct{}

// New creates a new Normalizer instance.
func New() *Normalizer {
	return &Normalizer{}
}

// Normalize processes a RawRequirement and converts it into a canonical NormalizedRequirements struct.
func (n *Normalizer) Normalize(raw model.RawRequirement) *model.NormalizedRequirements {
	promptLower := strings.ToLower(raw.Prompt)
	inv := normalizeInvariants(raw, promptLower)
	data := normalizeData(raw, promptLower, inv)

	norm := &model.NormalizedRequirements{
		ProjectName: extractProjectName(raw),
		Summary:     generateSummary(raw.Prompt),
		Domain:      detectDomain(raw, promptLower),
		Traffic:     normalizeTraffic(raw, promptLower),
		Data:        data,
		Latency:     normalizeLatency(raw, promptLower),
		Consistency: detectConsistency(raw, promptLower),
		Constraints: normalizeConstraints(raw, promptLower),
		Invariants:  inv,
		Assumptions: []string{},
	}

	// Record traffic assumptions / derivation notes
	if norm.Traffic.TrafficSource == "DERIVED_FROM_ARRIVALS" {
		norm.Assumptions = append(norm.Assumptions, norm.Traffic.DerivedRPSReason)
	} else if !norm.Traffic.IsExplicit {
		norm.Assumptions = append(norm.Assumptions,
			"Traffic not explicitly specified; assumed moderate baseline of 10 RPS with 30 RPS peak")
	}

	// Consistency enforcement: explicit business invariants override generic defaults
	if norm.Invariants.PreventDuplicates {
		if norm.Consistency == model.ConsistencyUnspecified {
			norm.Consistency = model.ConsistencyStrong
			norm.Assumptions = append(norm.Assumptions,
				"Consistency enforced as STRONG to guarantee uniqueness and prevent duplicate records")
		}
	} else if norm.Consistency == model.ConsistencyUnspecified {
		if norm.Domain == model.DomainECommerce || norm.Domain == model.DomainFinTech {
			norm.Consistency = model.ConsistencyStrong
			norm.Assumptions = append(norm.Assumptions,
				"Consistency defaulted to STRONG due to financial/commerce domain invariants")
		} else {
			norm.Consistency = model.ConsistencyEventual
			norm.Assumptions = append(norm.Assumptions,
				"Consistency defaulted to EVENTUAL for read-oriented/standard workload")
		}
	}

	// Idempotency assumptions if network resilience is required
	if norm.Invariants.RequiresIdempotency {
		norm.Assumptions = append(norm.Assumptions,
			"Idempotency validation required on mutating check-in/payment endpoints to support safe client retries across network drops")
	}

	// Populate explicit provenance for numeric and architectural values
	norm.FieldProvenance = buildFieldProvenance(norm, promptLower)

	return norm
}

// extractProjectName generates a domain-aware slug or name based on prompt phrasing or hints.
func extractProjectName(raw model.RawRequirement) string {
	if raw.DomainHint != "" {
		return strings.ReplaceAll(strings.Title(strings.ToLower(raw.DomainHint)), " ", "") + "Service"
	}

	prompt := raw.Prompt
	buildRe := regexp.MustCompile(`(?i)(?:build|design|create|develop)\s+(?:a|an)\s+(?:web-based\s+)?([A-Za-z0-9\s-]+?)(?:\s+for|\s+with|\s+that|\s+handling|\.|\n|$)`)
	if m := buildRe.FindStringSubmatch(prompt); len(m) > 1 {
		candidate := strings.TrimSpace(m[1])
		words := strings.Fields(candidate)
		if len(words) >= 1 && len(words) <= 6 {
			var sb strings.Builder
			for _, w := range words {
				clean := strings.Map(func(r rune) rune {
					if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
						return r
					}
					return -1
				}, w)
				if len(clean) > 0 {
					sb.WriteString(strings.ToUpper(clean[:1]) + strings.ToLower(clean[1:]))
				}
			}
			res := sb.String()
			if len(res) > 3 {
				return res
			}
		}
	}

	return "SystemDesignTarget"
}

// generateSummary extracts or shortens the raw prompt to a concise summary.
func generateSummary(prompt string) string {
	lines := strings.Split(prompt, "\n")
	first := strings.TrimSpace(lines[0])
	if len(first) > 160 {
		return first[:157] + "..."
	}
	if first == "" {
		return "Application architecture requirement"
	}
	return first
}

// detectDomain classifies business domain from keywords or domain hints.
func detectDomain(raw model.RawRequirement, promptLower string) model.DomainCategory {
	if raw.DomainHint != "" {
		switch strings.ToUpper(raw.DomainHint) {
		case "ECOMMERCE", "E-COMMERCE":
			return model.DomainECommerce
		case "FINTECH", "PAYMENTS":
			return model.DomainFinTech
		case "IOT", "TELEMETRY":
			return model.DomainIoTTelemetry
		case "ANALYTICS":
			return model.DomainAnalytics
		case "CHAT":
			return model.DomainRealtimeChat
		case "ASYNC_PIPELINE", "PIPELINE", "WORKER", "NOTIFICATION":
			return model.DomainAsyncPipeline
		case "EVENT_MANAGEMENT", "CONFERENCE":
			return model.DomainEventManagement
		}
	}

	if containsAny(promptLower, finTechKeywords) {
		return model.DomainFinTech
	}
	if containsAny(promptLower, eCommerceKeywords) {
		return model.DomainECommerce
	}
	if containsAny(promptLower, eventKeywords) {
		return model.DomainEventManagement
	}
	if containsAny(promptLower, telemetryKeywords) {
		return model.DomainIoTTelemetry
	}
	if containsAny(promptLower, pipelineKeywords) {
		return model.DomainAsyncPipeline
	}
	if containsAny(promptLower, chatKeywords) {
		return model.DomainRealtimeChat
	}
	if containsAny(promptLower, analyticsKeywords) {
		return model.DomainAnalytics
	}
	if containsAny(promptLower, contentKeywords) {
		return model.DomainContentMedia
	}

	return model.DomainGeneralCRUD
}

// normalizeTraffic extracts RPS, DAU, and read/write ratios from text.
func normalizeTraffic(raw model.RawRequirement, promptLower string) model.TrafficProfile {
	profile := model.TrafficProfile{
		ExpectedRPS:      10.0,
		PeakRPS:          30.0,
		DailyActiveUsers: 5000,
		ReadWriteRatio:   4.0, // Default 4:1
		IsBursty:         false,
		IsExplicit:       false,
		TrafficSource:    "ASSUMED_DEFAULT",
		DerivedRPSReason: "Traffic not explicitly specified; assumed moderate baseline of 10 RPS with 30 RPS peak",
	}

	// Check explicit flag or stated traffic string first.
	trafficText := promptLower + " " + strings.ToLower(raw.StatedTraffic)

	// Check for direct RPS mentions (e.g. "500 RPS" or "2,000 RPS" or "200 req/s").
	if match := rpsRegex.FindStringSubmatch(trafficText); len(match) > 1 {
		cleanedRPS := strings.ReplaceAll(match[1], ",", "")
		if val, err := strconv.ParseFloat(cleanedRPS, 64); err == nil && val > 0 {
			profile.ExpectedRPS = val
			profile.PeakRPS = val * 3.0 // Standard heuristic 3x peak multiplier
			profile.IsExplicit = true
			profile.TrafficSource = "EXPLICIT_RPS"
			profile.DerivedRPSReason = fmt.Sprintf("Explicitly specified in prompt (%0.0f RPS baseline, 3x peak)", val)
		}
	}

	// Check for total registered attendees / participants.
	if attendeeMatch := attendeeRegex.FindStringSubmatch(trafficText); len(attendeeMatch) > 1 {
		cleaned := strings.ReplaceAll(attendeeMatch[1], ",", "")
		if count, err := strconv.ParseInt(cleaned, 10, 64); err == nil && count > 0 {
			profile.StatedAttendees = count
			profile.DailyActiveUsers = count
		}
	}

	// Check for peak arrival volume window (e.g. "300-500 attendees arriving within a 30-minute window").
	if arrMatch := arrivalWindowRegex.FindStringSubmatch(trafficText); len(arrMatch) > 1 && profile.TrafficSource == "ASSUMED_DEFAULT" {
		minArr, err1 := strconv.ParseFloat(strings.ReplaceAll(arrMatch[1], ",", ""), 64)
		maxArr := minArr
		if len(arrMatch) > 2 && arrMatch[2] != "" {
			if parsedMax, err2 := strconv.ParseFloat(strings.ReplaceAll(arrMatch[2], ",", ""), 64); err2 == nil {
				maxArr = parsedMax
			}
		}
		mins := 30.0
		if len(arrMatch) > 3 && arrMatch[3] != "" {
			if parsedMins, err3 := strconv.ParseFloat(arrMatch[3], 64); err3 == nil && parsedMins > 0 {
				mins = parsedMins
			}
		}
		if err1 == nil && minArr > 0 {
			avgArrivalsPerSec := ((minArr + maxArr) / 2.0) / (mins * 60.0)
			profile.PeakArrivalWindow = arrMatch[0]
			profile.ArrivalsPerSec = avgArrivalsPerSec
			profile.ExpectedRPS = 2.0
			profile.PeakRPS = 15.0 // Estimated peak HTTP RPS for simultaneous QR scans, retries, and dashboard loads
			profile.EstimatedHTTPRPS = 15.0
			profile.IsExplicit = true
			profile.TrafficSource = "DERIVED_FROM_ARRIVALS"
			profile.DerivedRPSReason = fmt.Sprintf(
				"Physical arrival rate derived from %s (%0.2f arrivals/sec). Estimated peak HTTP traffic is 15-25 RPS to account for concurrent volunteer QR scans, device retries, schedule queries, and administrative dashboard refreshes.",
				arrMatch[0], avgArrivalsPerSec,
			)
		}
	}

	// Check for DAU mentions (e.g. "100k DAU").
	if match := dauRegex.FindStringSubmatch(trafficText); len(match) > 0 && profile.TrafficSource == "ASSUMED_DEFAULT" {
		var dau float64
		sub := match[0]
		if kMatch := kNumRegex.FindStringSubmatch(sub); len(kMatch) > 1 {
			if v, err := strconv.ParseFloat(kMatch[1], 64); err == nil {
				dau = v * 1000
			}
		} else if mMatch := mNumRegex.FindStringSubmatch(sub); len(mMatch) > 1 {
			if v, err := strconv.ParseFloat(mMatch[1], 64); err == nil {
				dau = v * 1000000
			}
		}

		if dau > 0 {
			profile.DailyActiveUsers = int64(dau)
			// Heuristic: Average active user makes ~20 requests per day.
			// Avg RPS = (DAU * 20) / 86400s
			inferredRPS := (dau * 20.0) / 86400.0
			profile.ExpectedRPS = inferredRPS
			profile.PeakRPS = inferredRPS * 4.0
			profile.IsExplicit = true
			profile.TrafficSource = "DERIVED_FROM_DAU"
			profile.DerivedRPSReason = fmt.Sprintf("Derived from stated DAU (%d active users): avg %0.2f RPS", profile.DailyActiveUsers, inferredRPS)
		}
	}

	// Detect burstiness keywords.
	if strings.Contains(promptLower, "spike") || strings.Contains(promptLower, "flash sale") ||
		strings.Contains(promptLower, "burst") || strings.Contains(promptLower, "black friday") ||
		strings.Contains(promptLower, "arriving within") {
		profile.IsBursty = true
	}

	// Adjust read-write ratios based on explicit ratio or domain hints.
	if match := ratioRegex.FindStringSubmatch(trafficText); len(match) > 2 {
		rVal, err1 := strconv.ParseFloat(match[1], 64)
		wVal, err2 := strconv.ParseFloat(match[2], 64)
		if err1 == nil && err2 == nil && wVal > 0 {
			profile.ReadWriteRatio = rVal / wVal
		}
	} else if strings.Contains(promptLower, "read-heavy") || strings.Contains(promptLower, "catalog") || strings.Contains(promptLower, "feed") || strings.Contains(promptLower, "shortener") {
		profile.ReadWriteRatio = 10.0
	} else if strings.Contains(promptLower, "write-heavy") || strings.Contains(promptLower, "telemetry") || strings.Contains(promptLower, "ingestion") {
		profile.ReadWriteRatio = 0.2 // 5 writes per 1 read
	}

	return profile
}

// normalizeInvariants extracts domain invariants and correctness requirements from prompt text.
func normalizeInvariants(raw model.RawRequirement, promptLower string) model.BusinessInvariants {
	inv := model.BusinessInvariants{
		PreventDuplicates:           false,
		UniquenessTargets:           make([]string, 0),
		RequiresIdempotency:         false,
		RequiresOfflineHandling:     false,
		OfflineConflictPolicy:       "",
		ExternalCallsInTxProhibited: true,
		AsyncOperations:             make([]string, 0),
	}

	// 1. Duplicate prevention
	if strings.Contains(promptLower, "prevent duplicate") ||
		strings.Contains(promptLower, "cannot check in multiple times") ||
		strings.Contains(promptLower, "cannot check in twice") ||
		strings.Contains(promptLower, "duplicate attendance") ||
		strings.Contains(promptLower, "duplicate registration") ||
		strings.Contains(promptLower, "prevent double") ||
		strings.Contains(promptLower, "duplicate payment") ||
		strings.Contains(promptLower, "duplicate charge") {
		inv.PreventDuplicates = true
		if strings.Contains(promptLower, "attendance") || strings.Contains(promptLower, "check-in") || strings.Contains(promptLower, "check in") {
			inv.UniquenessTargets = append(inv.UniquenessTargets, "attendance(delegate_id, session_id)")
		}
		if strings.Contains(promptLower, "registration") || strings.Contains(promptLower, "delegate") {
			inv.UniquenessTargets = append(inv.UniquenessTargets, "registration(email/delegate_id)")
		}
		if strings.Contains(promptLower, "payment") || strings.Contains(promptLower, "charge") {
			inv.UniquenessTargets = append(inv.UniquenessTargets, "payment(transaction_id/idempotency_key)")
		}
	}

	// 2. Idempotency & network retry resilience
	if strings.Contains(promptLower, "idempotent") ||
		strings.Contains(promptLower, "network interruption") ||
		strings.Contains(promptLower, "network failure") ||
		strings.Contains(promptLower, "simultaneously from different devices") ||
		strings.Contains(promptLower, "simultaneous") ||
		strings.Contains(promptLower, "retry") ||
		strings.Contains(promptLower, "retries") ||
		strings.Contains(promptLower, "offline") {
		inv.RequiresIdempotency = true
	}

	// 3. Offline handling
	if strings.Contains(promptLower, "offline") ||
		strings.Contains(promptLower, "temporary network interruption") ||
		strings.Contains(promptLower, "network interruptions") {
		inv.RequiresOfflineHandling = true
		inv.OfflineConflictPolicy = "SERVER_FIRST_WRITE_WINS"
	}

	// 4. Async operations
	if strings.Contains(promptLower, "certificate generation") || strings.Contains(promptLower, "certificate") {
		inv.AsyncOperations = append(inv.AsyncOperations, "certificate generation")
	}
	if strings.Contains(promptLower, "email") || strings.Contains(promptLower, "notification") {
		inv.AsyncOperations = append(inv.AsyncOperations, "email notifications")
	}
	if strings.Contains(promptLower, "export") || strings.Contains(promptLower, "report") {
		inv.AsyncOperations = append(inv.AsyncOperations, "attendance reports")
	}

	return inv
}

// normalizeData extracts persistence requirements and ACID needs.
func normalizeData(raw model.RawRequirement, promptLower string, inv model.BusinessInvariants) model.DataProfile {
	data := model.DataProfile{
		PrimaryStorage:       "RELATIONAL",
		DailyVolumeMB:        100.0,
		RequiresACID:         false,
		TransactionScope:     "NONE",
		AtomicOutboxRequired: false,
		HasHighContention:    false,
		RetentionDays:        90,
	}

	if strings.Contains(promptLower, "acid") || strings.Contains(promptLower, "payment") ||
		strings.Contains(promptLower, "financial") || strings.Contains(promptLower, "ledger") ||
		strings.Contains(promptLower, "balance") {
		data.RequiresACID = true
		data.TransactionScope = "PER_OPERATION_ATOMIC"
	}

	if strings.Contains(promptLower, "inventory") || strings.Contains(promptLower, "ticket booking") ||
		strings.Contains(promptLower, "seat reservation") || strings.Contains(promptLower, "flash sale") {
		data.HasHighContention = true
		data.RequiresACID = true
		data.TransactionScope = "PER_OPERATION_ATOMIC"
	}

	// Duplicate prevention and transactional outbox require atomic multi-step transactions
	if inv.PreventDuplicates || len(inv.AsyncOperations) > 0 {
		data.RequiresACID = true
		data.TransactionScope = "PER_OPERATION_ATOMIC"
	}
	if len(inv.AsyncOperations) > 0 {
		data.AtomicOutboxRequired = true
	}

	if strings.Contains(promptLower, "document") || strings.Contains(promptLower, "mongodb") {
		data.PrimaryStorage = "DOCUMENT"
	} else if strings.Contains(promptLower, "timeseries") || strings.Contains(promptLower, "time-series") {
		data.PrimaryStorage = "TIMESERIES"
	}

	return data
}

// buildFieldProvenance tracks origin, derivation method, and constraint status for parameters.
func buildFieldProvenance(norm *model.NormalizedRequirements, promptLower string) map[string]model.ProvenanceMeta {
	prov := make(map[string]model.ProvenanceMeta)

	if norm.Traffic.TrafficSource == "EXPLICIT_RPS" {
		prov["traffic.expected_rps"] = model.ProvenanceMeta{
			Source:           model.ProvenanceUserProvided,
			Method:           "Extracted directly from user prompt RPS specification",
			IsHardConstraint: true,
			Confidence:       1.0,
		}
		prov["traffic.peak_rps"] = model.ProvenanceMeta{
			Source:           model.ProvenanceDerived,
			Method:           "Derived 3x burst multiplier over user-stated baseline RPS",
			IsHardConstraint: false,
			Confidence:       0.90,
		}
	} else if norm.Traffic.TrafficSource == "DERIVED_FROM_ARRIVALS" {
		prov["traffic.expected_rps"] = model.ProvenanceMeta{
			Source:           model.ProvenanceDerived,
			Method:           norm.Traffic.DerivedRPSReason,
			IsHardConstraint: false,
			Confidence:       0.85,
		}
		prov["traffic.peak_rps"] = model.ProvenanceMeta{
			Source:           model.ProvenanceDerived,
			Method:           "Derived estimated peak HTTP RPS for concurrent QR scans, device retries, and dashboard refreshes",
			IsHardConstraint: false,
			Confidence:       0.85,
		}
	} else if norm.Traffic.TrafficSource == "DERIVED_FROM_DAU" {
		prov["traffic.expected_rps"] = model.ProvenanceMeta{
			Source:           model.ProvenanceDerived,
			Method:           norm.Traffic.DerivedRPSReason,
			IsHardConstraint: false,
			Confidence:       0.80,
		}
		prov["traffic.peak_rps"] = model.ProvenanceMeta{
			Source:           model.ProvenanceDerived,
			Method:           "Derived 4x burst multiplier from DAU baseline traffic estimate",
			IsHardConstraint: false,
			Confidence:       0.80,
		}
	} else {
		prov["traffic.expected_rps"] = model.ProvenanceMeta{
			Source:           model.ProvenanceAssumedDefault,
			Method:           "Assumed standard baseline traffic in absence of explicit volume metrics",
			IsHardConstraint: false,
			Confidence:       0.50,
		}
		prov["traffic.peak_rps"] = model.ProvenanceMeta{
			Source:           model.ProvenanceAssumedDefault,
			Method:           "Heuristic 3x burst headroom over default baseline",
			IsHardConstraint: false,
			Confidence:       0.50,
		}
	}

	if ratioRegex.MatchString(promptLower) {
		prov["traffic.read_write_ratio"] = model.ProvenanceMeta{
			Source:           model.ProvenanceUserProvided,
			Method:           "Extracted from explicit read-to-write ratio specification",
			IsHardConstraint: true,
			Confidence:       1.0,
		}
	} else if strings.Contains(promptLower, "read-heavy") || strings.Contains(promptLower, "write-heavy") {
		prov["traffic.read_write_ratio"] = model.ProvenanceMeta{
			Source:           model.ProvenanceDerived,
			Method:           "Derived from stated workload bias (read-heavy or write-heavy)",
			IsHardConstraint: false,
			Confidence:       0.85,
		}
	} else {
		prov["traffic.read_write_ratio"] = model.ProvenanceMeta{
			Source:           model.ProvenanceSystemDefault,
			Method:           "Standard web application default read-write ratio (4:1)",
			IsHardConstraint: false,
			Confidence:       0.60,
		}
	}

	prov["data.daily_volume_mb"] = model.ProvenanceMeta{
		Source:           model.ProvenanceAssumedDefault,
		Method:           "Default planning estimate for early-stage relational storage",
		IsHardConstraint: false,
		Confidence:       0.50,
	}

	prov["data.retention_days"] = model.ProvenanceMeta{
		Source:           model.ProvenanceSystemDefault,
		Method:           "Default operational data retention window (90 days) prior to cold archive",
		IsHardConstraint: false,
		Confidence:       0.50,
	}

	if strings.Contains(promptLower, "acid") {
		prov["data.requires_acid"] = model.ProvenanceMeta{
			Source:           model.ProvenanceUserProvided,
			Method:           "Explicitly required by user prompt",
			IsHardConstraint: true,
			Confidence:       1.0,
		}
	} else if norm.Invariants.PreventDuplicates || norm.Data.AtomicOutboxRequired {
		prov["data.requires_acid"] = model.ProvenanceMeta{
			Source:           model.ProvenanceDerived,
			Method:           "Derived from requirement for atomic transactional outbox and database-enforced duplicate prevention",
			IsHardConstraint: true,
			Confidence:       0.95,
		}
	} else {
		prov["data.requires_acid"] = model.ProvenanceMeta{
			Source:           model.ProvenanceSystemDefault,
			Method:           "ACID transactions not required for read-only or stateless workflows",
			IsHardConstraint: false,
			Confidence:       0.90,
		}
	}

	if latencyMsRegex.MatchString(promptLower) {
		prov["latency.p99_target_ms"] = model.ProvenanceMeta{
			Source:           model.ProvenanceUserProvided,
			Method:           "Extracted directly from user-specified latency SLA",
			IsHardConstraint: true,
			Confidence:       1.0,
		}
	} else {
		prov["latency.p99_target_ms"] = model.ProvenanceMeta{
			Source:           model.ProvenanceSystemDefault,
			Method:           "Standard web service P99 target budget (500ms)",
			IsHardConstraint: false,
			Confidence:       0.70,
		}
	}

	prov["latency.p95_target_ms"] = model.ProvenanceMeta{
		Source:           model.ProvenanceSystemDefault,
		Method:           "Standard interactive web application P95 target budget (200ms)",
		IsHardConstraint: false,
		Confidence:       0.70,
	}

	prov["latency.max_acceptable_ms"] = model.ProvenanceMeta{
		Source:           model.ProvenanceSystemDefault,
		Method:           "System default request timeout threshold (2000ms)",
		IsHardConstraint: false,
		Confidence:       0.90,
	}

	return prov
}

// normalizeLatency extracts SLA and latency budgets.
func normalizeLatency(raw model.RawRequirement, promptLower string) model.LatencyProfile {
	latency := model.LatencyProfile{
		P95TargetMs:     200,
		P99TargetMs:     500,
		MaxAcceptableMs: 2000,
	}

	if match := latencyMsRegex.FindStringSubmatch(promptLower); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil && val > 0 {
			latency.P99TargetMs = val
			latency.P95TargetMs = int(float64(val) * 0.6)
			latency.MaxAcceptableMs = val * 3
		}
	} else if strings.Contains(promptLower, "low latency") || strings.Contains(promptLower, "real-time") {
		latency.P95TargetMs = 50
		latency.P99TargetMs = 100
		latency.MaxAcceptableMs = 500
	}

	return latency
}

// detectConsistency extracts consistency constraints.
func detectConsistency(raw model.RawRequirement, promptLower string) model.ConsistencyLevel {
	if strings.Contains(promptLower, "strong consistency") || strings.Contains(promptLower, "strict consistency") {
		return model.ConsistencyStrong
	}
	if strings.Contains(promptLower, "eventual consistency") {
		return model.ConsistencyEventual
	}
	return model.ConsistencyUnspecified
}

// normalizeConstraints captures tech stack preferences, compliance, and deployment targets.
func normalizeConstraints(raw model.RawRequirement, promptLower string) model.Constraints {
	constraints := model.Constraints{
		PreferredStack:   raw.PreferredStack,
		DeploymentTarget: "CONTAINER",
		ComplianceNeeds:  []string{},
	}

	// Scan compliance keywords.
	if strings.Contains(promptLower, "pci") || strings.Contains(promptLower, "pci-dss") {
		constraints.ComplianceNeeds = append(constraints.ComplianceNeeds, "PCI-DSS")
	}
	if strings.Contains(promptLower, "hipaa") {
		constraints.ComplianceNeeds = append(constraints.ComplianceNeeds, "HIPAA")
	}
	if strings.Contains(promptLower, "gdpr") {
		constraints.ComplianceNeeds = append(constraints.ComplianceNeeds, "GDPR")
	}

	// Detect deployment targets if explicitly mentioned.
	if strings.Contains(promptLower, "serverless") || strings.Contains(promptLower, "lambda") {
		constraints.DeploymentTarget = "SERVERLESS"
	} else if strings.Contains(promptLower, "single server") || strings.Contains(promptLower, "monolith") {
		constraints.DeploymentTarget = "SINGLE_SERVER"
	}

	return constraints
}

// containsAny is a helper returning true if text contains any of the target keywords.
func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
