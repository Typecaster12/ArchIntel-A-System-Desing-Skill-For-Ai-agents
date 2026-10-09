package server

import (
	"context"
	"net/http"
	"time"

	"github.com/sdskill/archintel/pkg/engine"
)

// Server encapsulates the HTTP REST API server.
type Server struct {
	httpServer *http.Server
	handler    *Handler
}

// Config defines configuration options for the HTTP server.
type Config struct {
	Addr string
}

// NewServer creates a new HTTP Server instance.
func NewServer(eng *engine.Engine, cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}

	h := NewHandler(eng)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.HealthCheck)
	mux.HandleFunc("/api/v1/recommend", h.RecommendArchitecture)
	mux.HandleFunc("/api/v1/validate", h.ValidateArchitecture)

	return &Server{
		handler: h,
		httpServer: &http.Server{
			Addr:         cfg.Addr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

// Handler returns the underlying http.Handler for testing with httptest.
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// Start begins listening on the configured address.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully terminates the HTTP server without dropping active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
