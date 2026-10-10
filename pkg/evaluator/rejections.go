package evaluator

import (
	"fmt"

	"github.com/sdskill/archintel/pkg/model"
)

// RejectionGenerator produces explicit, well-reasoned justifications for discarded architectural options.
type RejectionGenerator struct{}

// NewRejectionGenerator creates a new rejection generator.
func NewRejectionGenerator() *RejectionGenerator {
	return &RejectionGenerator{}
}

// GenerateRejections compares selected decisions against alternative patterns and documents why they were discarded.
func (g *RejectionGenerator) GenerateRejections(req *model.NormalizedRequirements, class *model.ClassificationResult, summary *DecisionSummary) []model.RejectedAlternative {
	var rejections []model.RejectedAlternative

	// 1. Rejection of Microservices if Modular Monolith was chosen
	if summary.RecommendedTopology == model.TopologyModularMonolith {
		rejections = append(rejections, model.RejectedAlternative{
			Pattern: "Microservices Architecture",
			Reason: fmt.Sprintf(
				"Rejected due to operational and cognitive complexity under current team and deployment constraints (%s). Workload domain boundaries and traffic requirements are fully satisfied by a Modular Monolith. Microservices would introduce network serialization latency, cross-service transaction failure modes, and multi-service deployment overhead without justification.",
				req.Constraints.DeploymentTarget,
			),
		})
	}

	// 2. Rejection of Dedicated Message Brokers (Kafka / RabbitMQ) if Postgres SKIP LOCKED was selected
	if summary.RecommendedQueueRules.QueueMechanism == "POSTGRES_SKIP_LOCKED" {
		rejections = append(rejections, model.RejectedAlternative{
			Pattern: "Dedicated Message Broker (Kafka / RabbitMQ / SQS)",
			Reason:  "Rejected to prevent distributed dual-write inconsistency and unnecessary infrastructure maintenance. PostgreSQL SKIP LOCKED enables enqueuing background tasks atomically within the local database transaction, guaranteeing outbox atomicity and bounded asynchronous processing without external broker clusters.",
		})
	}

	// 3. Rejection of Optimistic Concurrency Control if Pessimistic Locking was selected
	if summary.RecommendedDataRules.Locking == model.LockingPessimistic {
		rejections = append(rejections, model.RejectedAlternative{
			Pattern: "Optimistic Concurrency Control (Version tag retries)",
			Reason:  "Rejected due to high write contention on shared records (e.g. inventory or balances). Under burst concurrency, version collisions cause repeated application retry loops, degrading throughput and causing cascading tail-latency spikes.",
		})
	}

	// 4. Rejection of Pessimistic Locking if Optimistic Locking was selected
	if summary.RecommendedDataRules.Locking == model.LockingOptimistic {
		rejections = append(rejections, model.RejectedAlternative{
			Pattern: "Pessimistic Concurrency Control (SELECT FOR UPDATE)",
			Reason:  "Rejected because concurrent write collisions are rare for this workload. Holding exclusive row locks would needlessly increase database lock contention and limit read scalability.",
		})
	}

	// 5. Rejection of Synchronous Direct Webhook Calls in Payment Workflows
	if summary.RecommendedDataRules.RequiresOutboxPattern {
		rejections = append(rejections, model.RejectedAlternative{
			Pattern: "Synchronous External API Dispatch Inside Request Handler",
			Reason:  "Rejected because third-party network timeouts or outages would directly block client requests and risk database connection pool exhaustion. An asynchronous worker polling the Transactional Outbox provides resilient at-least-once delivery with jittered retries.",
		})
	}

	return rejections
}
