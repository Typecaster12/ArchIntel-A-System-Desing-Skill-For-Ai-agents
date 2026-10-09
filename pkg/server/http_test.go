package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sdskill/archintel/pkg/engine"
	"github.com/sdskill/archintel/pkg/model"
)

func TestHTTPHealthCheck(t *testing.T) {
	eng := engine.NewEngine(engine.EngineOptions{})
	srv := NewServer(eng, Config{Addr: ":0"})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode json body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestHTTPRecommendArchitecture(t *testing.T) {
	eng := engine.NewEngine(engine.EngineOptions{})
	srv := NewServer(eng, Config{Addr: ":0"})

	payload := model.RawRequirement{
		Prompt: "Build a high throughput payment processing gateway with PCI compliance",
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recommend", bytes.NewReader(payloadBytes))
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp engine.RecommendationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Contract == nil {
		t.Fatalf("expected contract in response")
	}
	if resp.Contract.Topology.Type != model.TopologyModularMonolith {
		t.Errorf("expected TopologyModularMonolith, got %v", resp.Contract.Topology.Type)
	}
}
