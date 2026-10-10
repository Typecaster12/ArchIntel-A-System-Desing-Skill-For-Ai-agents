package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sdskill/archintel/pkg/model"
)

// LayaConfig holds connection and timeout parameters for the LAYA inference service.
type LayaConfig struct {
	// BaseURL is the HTTP endpoint of the LAYA inference server (e.g. "http://localhost:8000").
	BaseURL string

	// Timeout is the maximum duration to wait for LAYA response before falling back.
	Timeout time.Duration

	// FallbackClassifier is invoked when LAYA is offline, unconfigured, or fails.
	FallbackClassifier Classifier
}

// LayaAdapter connects to an external LAYA decision model service over HTTP,
// with transparent fallback to deterministic heuristics when the model is unavailable.
type LayaAdapter struct {
	baseURL    string
	timeout    time.Duration
	fallback   Classifier
	httpClient *http.Client
}

// NewLayaAdapter creates a new LayaAdapter with a fallback classifier.
func NewLayaAdapter(cfg LayaConfig) *LayaAdapter {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 500 * time.Millisecond // Bounded fast timeout
	}

	fallback := cfg.FallbackClassifier
	if fallback == nil {
		fallback = NewRuleBasedClassifier()
	}

	return &LayaAdapter{
		baseURL:  cfg.BaseURL,
		timeout:  timeout,
		fallback: fallback,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// LayaClassificationPayload defines the JSON body sent to the LAYA decision endpoint.
type LayaClassificationPayload struct {
	ProjectName  string  `json:"project_name"`
	Domain       string  `json:"domain"`
	Summary      string  `json:"summary"`
	ExpectedRPS  float64 `json:"expected_rps"`
	IsBursty     bool    `json:"is_bursty"`
	RequiresACID bool    `json:"requires_acid"`
}

// LayaClassificationResponse defines the expected JSON response from the LAYA model.
type LayaClassificationResponse struct {
	ModelVersion  string             `json:"model_version"`
	LatencyMs     int64              `json:"latency_ms"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// Classify attempts LAYA model inference. If the service is unreachable or unconfigured,
// it gracefully and transparently falls back to the heuristic classifier.
func (a *LayaAdapter) Classify(ctx context.Context, req *model.NormalizedRequirements) (*model.ClassificationResult, error) {
	// If no BaseURL is configured, immediately use the deterministic fallback.
	if a.baseURL == "" {
		res, err := a.fallback.Classify(ctx, req)
		if err != nil {
			return nil, err
		}
		res.Source = model.ClassifierSourceRuleBased
		res.FallbackApplied = true
		res.ModelVersion = "deterministic rule-based heuristics (LAYA service unconfigured; running standalone deterministic engine)"
		return res, nil
	}

	payload := LayaClassificationPayload{
		ProjectName:  req.ProjectName,
		Domain:       string(req.Domain),
		Summary:      req.Summary,
		ExpectedRPS:  req.Traffic.ExpectedRPS,
		IsBursty:     req.Traffic.IsBursty,
		RequiresACID: req.Data.RequiresACID,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return a.executeFallback(ctx, req, fmt.Sprintf("payload marshal error: %v", err))
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/classify", bytes.NewReader(bodyBytes))
	if err != nil {
		return a.executeFallback(ctx, req, fmt.Sprintf("request creation error: %v", err))
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		// Connection refused, timeout, or network error: fallback gracefully.
		return a.executeFallback(ctx, req, fmt.Sprintf("laya offline (%v)", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return a.executeFallback(ctx, req, fmt.Sprintf("laya status %d", resp.StatusCode))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return a.executeFallback(ctx, req, "failed to read response body")
	}

	var layaResp LayaClassificationResponse
	if err := json.Unmarshal(respBytes, &layaResp); err != nil {
		return a.executeFallback(ctx, req, "malformed laya json response")
	}

	// Successfully parsed genuine LAYA response.
	scores := make(map[model.WorkloadTrait]model.ClassificationScore)
	var activeTraits []model.WorkloadTrait
	var primaryTrait model.WorkloadTrait
	var maxProb float64

	for traitStr, prob := range layaResp.Probabilities {
		trait := model.WorkloadTrait(traitStr)
		isActive := prob >= DefaultActivationThreshold

		scores[trait] = model.ClassificationScore{
			Trait:     trait,
			Score:     prob,
			IsActive:  isActive,
			Rationale: fmt.Sprintf("Calibrated probability from LAYA model checkpoint (%s)", layaResp.ModelVersion),
		}

		if isActive {
			activeTraits = append(activeTraits, trait)
			if prob > maxProb {
				maxProb = prob
				primaryTrait = trait
			}
		}
	}

	if primaryTrait == "" {
		primaryTrait = model.TraitReadHeavy
	}

	return &model.ClassificationResult{
		Source:             model.ClassifierSourceLayaModel,
		ModelVersion:       layaResp.ModelVersion,
		InferenceLatencyMs: layaResp.LatencyMs,
		Scores:             scores,
		ActiveTraits:       activeTraits,
		PrimaryTrait:       primaryTrait,
		FallbackApplied:    false,
	}, nil
}

func (a *LayaAdapter) executeFallback(ctx context.Context, req *model.NormalizedRequirements, reason string) (*model.ClassificationResult, error) {
	res, err := a.fallback.Classify(ctx, req)
	if err != nil {
		return nil, err
	}
	res.Source = model.ClassifierSourceRuleBased
	res.FallbackApplied = true
	res.ModelVersion = fmt.Sprintf("fallback to deterministic heuristics: %s", reason)
	return res, nil
}
