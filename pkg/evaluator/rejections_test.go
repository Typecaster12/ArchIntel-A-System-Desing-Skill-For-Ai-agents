package evaluator

import (
	"testing"

	"github.com/sdskill/archintel/pkg/model"
)

func TestRejectionGeneratorEcommerce(t *testing.T) {
	gen := NewRejectionGenerator()

	req := &model.NormalizedRequirements{
		ProjectName: "ShopCore",
		Traffic: model.TrafficProfile{
			ExpectedRPS: 50,
			PeakRPS:     150,
		},
	}

	class := &model.ClassificationResult{}

	summary := &DecisionSummary{
		RecommendedTopology: model.TopologyModularMonolith,
		RecommendedDataRules: model.DataRules{
			Locking:               model.LockingPessimistic,
			RequiresOutboxPattern: true,
		},
		RecommendedQueueRules: model.QueueRules{
			QueueMechanism: "POSTGRES_SKIP_LOCKED",
		},
	}

	rejections := gen.GenerateRejections(req, class, summary)
	if len(rejections) < 3 {
		t.Fatalf("expected at least 3 rejected alternatives, got %d", len(rejections))
	}

	foundMicro := false
	foundBroker := false
	foundOptimistic := false

	for _, r := range rejections {
		if r.Pattern == "Microservices Architecture" {
			foundMicro = true
		}
		if r.Pattern == "Dedicated Message Broker (Kafka / RabbitMQ / SQS)" {
			foundBroker = true
		}
		if r.Pattern == "Optimistic Concurrency Control (Version tag retries)" {
			foundOptimistic = true
		}
	}

	if !foundMicro {
		t.Errorf("expected Microservices Architecture to be rejected")
	}
	if !foundBroker {
		t.Errorf("expected Dedicated Message Broker to be rejected")
	}
	if !foundOptimistic {
		t.Errorf("expected Optimistic Concurrency Control to be rejected")
	}
}
