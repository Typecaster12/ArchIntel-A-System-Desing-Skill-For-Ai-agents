package e2e

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/sdskill/archintel/pkg/engine"
	"github.com/sdskill/archintel/pkg/model"
)

func loadFixture(t *testing.T, path string) model.RawRequirement {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var raw model.RawRequirement
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse fixture json %s: %v", path, err)
	}

	return raw
}

func TestE2EEcommerceCheckout(t *testing.T) {
	eng := engine.NewEngine(engine.EngineOptions{})
	ctx := context.Background()

	raw := loadFixture(t, "../fixtures/ecommerce_checkout.json")
	resp, err := eng.RecommendArchitecture(ctx, raw)
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if contract.Topology.Type != model.TopologyModularMonolith {
		t.Errorf("expected Modular Monolith for 300 RPS e-commerce, got %v", contract.Topology.Type)
	}
	if contract.Data.Locking != model.LockingPessimistic {
		t.Errorf("expected Pessimistic Locking for flash sales, got %v", contract.Data.Locking)
	}
	if !contract.Data.RequiresIdempotencyKey {
		t.Errorf("expected RequiresIdempotencyKey to be true")
	}
	if !contract.Data.RequiresOutboxPattern {
		t.Errorf("expected RequiresOutboxPattern to be true")
	}
	if contract.Queue.QueueMechanism != "POSTGRES_SKIP_LOCKED" {
		t.Errorf("expected POSTGRES_SKIP_LOCKED for 300 RPS, got %v", contract.Queue.QueueMechanism)
	}

	// Verify rejected alternatives
	foundMicroRejection := false
	foundBrokerRejection := false
	for _, r := range contract.RejectedAlternatives {
		if r.Pattern == "Microservices Architecture" {
			foundMicroRejection = true
		}
		if r.Pattern == "Dedicated Message Broker (Kafka / RabbitMQ / SQS)" {
			foundBrokerRejection = true
		}
	}

	if !foundMicroRejection {
		t.Errorf("expected Microservices Architecture to be explicitly rejected")
	}
	if !foundBrokerRejection {
		t.Errorf("expected Dedicated Message Broker to be explicitly rejected")
	}
}

func TestE2EURLShortener(t *testing.T) {
	eng := engine.NewEngine(engine.EngineOptions{})
	ctx := context.Background()

	raw := loadFixture(t, "../fixtures/url_shortener.json")
	resp, err := eng.RecommendArchitecture(ctx, raw)
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if contract.Topology.Type != model.TopologyModularMonolith {
		t.Errorf("expected Modular Monolith, got %v", contract.Topology.Type)
	}

	// Verify Cache-Aside is among selected patterns
	foundCacheAside := false
	for _, p := range resp.DecisionSummary.SelectedPatterns {
		if p.PatternID == "PAT-CACHE-ASIDE" {
			foundCacheAside = true
		}
	}
	if !foundCacheAside {
		t.Errorf("expected PAT-CACHE-ASIDE to be selected for 20:1 read-heavy URL shortener")
	}
}

func TestE2ENotificationDispatcher(t *testing.T) {
	eng := engine.NewEngine(engine.EngineOptions{})
	ctx := context.Background()

	raw := loadFixture(t, "../fixtures/notification_dispatcher.json")
	resp, err := eng.RecommendArchitecture(ctx, raw)
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if !contract.Queue.Enabled {
		t.Errorf("expected Queue to be enabled for notification dispatcher")
	}
	if contract.Queue.MaxWorkerConcurrency <= 0 {
		t.Errorf("expected bounded MaxWorkerConcurrency > 0, got %d", contract.Queue.MaxWorkerConcurrency)
	}
	if contract.Queue.DeadLetterHandling == "" {
		t.Errorf("expected DeadLetterHandling to be configured")
	}
}

func TestE2ECRUDAdminPanel(t *testing.T) {
	eng := engine.NewEngine(engine.EngineOptions{})
	ctx := context.Background()

	raw := loadFixture(t, "../fixtures/crud_admin.json")
	resp, err := eng.RecommendArchitecture(ctx, raw)
	if err != nil {
		t.Fatalf("RecommendArchitecture failed: %v", err)
	}

	contract := resp.Contract
	if contract.Topology.Type != model.TopologyModularMonolith {
		t.Errorf("expected Modular Monolith for internal admin panel, got %v", contract.Topology.Type)
	}
	if contract.Queue.Enabled {
		t.Errorf("expected Queue to be disabled for simple internal CRUD panel")
	}
	if contract.Data.Locking != model.LockingNone {
		t.Errorf("expected LockingNone for low-scale admin panel, got %v", contract.Data.Locking)
	}
}
