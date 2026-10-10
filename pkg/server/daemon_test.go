package server

import (
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/sdskill/archintel/pkg/engine"
)

func TestLiveServerDaemon(t *testing.T) {
	if os.Getenv("START_SERVER_DAEMON") != "1" {
		t.Skip("skipping daemon test unless START_SERVER_DAEMON=1")
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	eng := engine.NewEngine(engine.EngineOptions{})
	srv := NewServer(eng, Config{Addr: ":" + port})

	log.Printf("[archintel] Starting AI System Design Intelligence Layer on :%s", port)
	log.Printf("[archintel] Server ready to accept architectural queries on :%s", port)

	if err := srv.Start(); err != nil && err != http.ErrServerClosed {
		t.Fatalf("server error: %v", err)
	}
}
