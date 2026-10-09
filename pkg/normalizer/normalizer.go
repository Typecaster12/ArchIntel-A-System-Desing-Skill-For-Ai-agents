package normalizer

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sdskill/archintel/pkg/model"
)

var (
	// Regex patterns for detecting traffic and scale specifications.
	rpsRegex  = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:req(?:uests)?\/s(?:ec)?|rps)`)
	dauRegex  = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:k|m|million|thousand)?\s*(?:dau|daily\s+active\s+users)`)
	kNumRegex = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*k`)
	mNumRegex = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:m|million)`)

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

	norm := &model.NormalizedRequirements{
		ProjectName: extractProjectName(raw),
		Summary:     generateSummary(raw.Prompt),
		Domain:      detectDomain(raw, promptLower),
		Traffic:     normalizeTraffic(raw, promptLower),
		Data:        normalizeData(raw, promptLower),
		Latency:     normalizeLatency(raw, promptLower),
		Consistency: detectConsistency(raw, promptLower),
		Constraints: normalizeConstraints(raw, promptLower),
		Assumptions: []string{},
	}

	// Record default assumptions when metrics were not explicitly specified.
	if !norm.Traffic.IsExplicit {
		norm.Assumptions = append(norm.Assumptions,
			"Traffic not explicitly specified; assumed moderate baseline of 10 RPS with 30 RPS peak")
	}
	if norm.Consistency == model.ConsistencyUnspecified {
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

	return norm
}

// extractProjectName generates a slug or name based on hints or prompt keywords.
func extractProjectName(raw model.RawRequirement) string {
	if raw.DomainHint != "" {
		return strings.ReplaceAll(strings.Title(strings.ToLower(raw.DomainHint)), " ", "") + "Service"
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
		}
	}

	if containsAny(promptLower, finTechKeywords) {
		return model.DomainFinTech
	}
	if containsAny(promptLower, eCommerceKeywords) {
		return model.DomainECommerce
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
	}

	// Check explicit flag or stated traffic string first.
	trafficText := promptLower + " " + strings.ToLower(raw.StatedTraffic)

	// Check for direct RPS mentions (e.g. "500 RPS" or "200 req/s").
	if match := rpsRegex.FindStringSubmatch(trafficText); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil && val > 0 {
			profile.ExpectedRPS = val
			profile.PeakRPS = val * 3.0 // Standard heuristic 3x peak multiplier
			profile.IsExplicit = true
		}
	}

	// Check for DAU mentions (e.g. "100k DAU").
	if match := dauRegex.FindStringSubmatch(trafficText); len(match) > 0 {
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
			if !profile.IsExplicit {
				profile.ExpectedRPS = inferredRPS
				profile.PeakRPS = inferredRPS * 4.0
				profile.IsExplicit = true
			}
		}
	}

	// Detect burstiness keywords.
	if strings.Contains(promptLower, "spike") || strings.Contains(promptLower, "flash sale") ||
		strings.Contains(promptLower, "burst") || strings.Contains(promptLower, "black friday") {
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

// normalizeData extracts persistence requirements and ACID needs.
func normalizeData(raw model.RawRequirement, promptLower string) model.DataProfile {
	data := model.DataProfile{
		PrimaryStorage:    "RELATIONAL",
		DailyVolumeMB:     100.0,
		RequiresACID:      false,
		HasHighContention: false,
		RetentionDays:     90,
	}

	if strings.Contains(promptLower, "acid") || strings.Contains(promptLower, "payment") ||
		strings.Contains(promptLower, "financial") || strings.Contains(promptLower, "ledger") ||
		strings.Contains(promptLower, "balance") {
		data.RequiresACID = true
	}

	if strings.Contains(promptLower, "inventory") || strings.Contains(promptLower, "ticket booking") ||
		strings.Contains(promptLower, "seat reservation") || strings.Contains(promptLower, "flash sale") {
		data.HasHighContention = true
		data.RequiresACID = true
	}

	if strings.Contains(promptLower, "document") || strings.Contains(promptLower, "mongodb") {
		data.PrimaryStorage = "DOCUMENT"
	} else if strings.Contains(promptLower, "timeseries") || strings.Contains(promptLower, "time-series") {
		data.PrimaryStorage = "TIMESERIES"
	}

	return data
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
