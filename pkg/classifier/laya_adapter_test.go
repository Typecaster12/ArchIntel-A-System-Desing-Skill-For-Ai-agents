package classifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sdskill/archintel/pkg/model"
)

func TestLayaAdapterUnconfiguredFallback(t *testing.T) {
	adapter := NewLayaAdapter(LayaConfig{
		BaseURL: "", // unconfigured
	})

	req := &model.NormalizedRequirements{
		ProjectName: "UnconfiguredApp",
		Domain:      model.DomainGeneralCRUD,
	}

	res, err := adapter.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.FallbackApplied {
		t.Errorf("expected FallbackApplied to be true")
	}
	if res.Source != model.ClassifierSourceRuleBased {
		t.Errorf("expected source RuleBased, got %v", res.Source)
	}
}

func TestLayaAdapterOfflineFallback(t *testing.T) {
	// Point to unreachable localhost port
	adapter := NewLayaAdapter(LayaConfig{
		BaseURL: "http://127.0.0.1:54321",
		Timeout: 50 * time.Millisecond,
	})

	req := &model.NormalizedRequirements{
		ProjectName: "OfflineApp",
		Domain:      model.DomainFinTech,
		Data:        model.DataProfile{RequiresACID: true},
	}

	res, err := adapter.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.FallbackApplied {
		t.Errorf("expected FallbackApplied to be true when endpoint is offline")
	}
	if !res.HasTrait(model.TraitTransactionHeavy) {
		t.Errorf("expected fallback to detect TraitTransactionHeavy for FinTech")
	}
}

func TestLayaAdapterSuccessAndMalformed(t *testing.T) {
	// 1. Success case
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/classify" {
			http.NotFound(w, r)
			return
		}

		resp := LayaClassificationResponse{
			ModelVersion: "laya-v1-modernbert-checkpoint",
			LatencyMs:    32,
			Probabilities: map[string]float64{
				string(model.TraitTransactionHeavy):     0.96,
				string(model.TraitConsistencySensitive): 0.91,
				string(model.TraitReadHeavy):            0.20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	adapter := NewLayaAdapter(LayaConfig{
		BaseURL: server.URL,
		Timeout: 500 * time.Millisecond,
	})

	req := &model.NormalizedRequirements{
		ProjectName: "LiveLayaApp",
		Domain:      model.DomainFinTech,
	}

	res, err := adapter.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.FallbackApplied {
		t.Errorf("expected FallbackApplied to be false on live response")
	}
	if res.Source != model.ClassifierSourceLayaModel {
		t.Errorf("expected Source LayaModel, got %v", res.Source)
	}
	if !res.HasTrait(model.TraitTransactionHeavy) {
		t.Errorf("expected TraitTransactionHeavy to be active")
	}
	if res.InferenceLatencyMs != 32 {
		t.Errorf("expected LatencyMs 32, got %v", res.InferenceLatencyMs)
	}

	// 2. Malformed JSON case
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-valid-json"))
	}))
	defer badServer.Close()

	badAdapter := NewLayaAdapter(LayaConfig{
		BaseURL: badServer.URL,
		Timeout: 200 * time.Millisecond,
	})

	badRes, err := badAdapter.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on malformed response: %v", err)
	}
	if !badRes.FallbackApplied {
		t.Errorf("expected FallbackApplied to be true on malformed JSON")
	}
}
