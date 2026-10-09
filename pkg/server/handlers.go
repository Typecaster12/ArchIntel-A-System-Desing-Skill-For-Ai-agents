package server

import (
	"encoding/json"
	"net/http"

	"github.com/sdskill/archintel/pkg/engine"
	"github.com/sdskill/archintel/pkg/model"
	"github.com/sdskill/archintel/pkg/validator"
)

// Handler holds the HTTP route handlers.
type Handler struct {
	engine *engine.Engine
}

// NewHandler creates a new HTTP handler instance.
func NewHandler(eng *engine.Engine) *Handler {
	return &Handler{
		engine: eng,
	}
}

// HealthCheck responds with the operational health of the service.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "archintel",
		"version": "1.0.0",
	})
}

// RecommendArchitecture handles POST /api/v1/recommend.
func (h *Handler) RecommendArchitecture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var raw model.RawRequirement
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if raw.Prompt == "" {
		http.Error(w, "Field 'prompt' is required", http.StatusBadRequest)
		return
	}

	res, err := h.engine.RecommendArchitecture(r.Context(), raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

// ValidateRequest defines the payload for POST /api/v1/validate.
type ValidateRequest struct {
	Contract model.ArchitectureContract     `json:"contract"`
	Proposal validator.ImplementationProposal `json:"proposal"`
}

// ValidateArchitecture handles POST /api/v1/validate.
func (h *Handler) ValidateArchitecture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	report := h.engine.ValidateArchitecture(&req.Contract, req.Proposal)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(report)
}
