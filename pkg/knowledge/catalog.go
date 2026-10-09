package knowledge

import "github.com/sdskill/archintel/pkg/model"

// Catalog is the in-memory repository of architectural patterns.
type Catalog struct {
	patterns map[string]ArchitecturalPattern
}

// NewCatalog initializes the default curated catalog of architectural patterns.
func NewCatalog() *Catalog {
	c := &Catalog{
		patterns: make(map[string]ArchitecturalPattern),
	}
	c.seed()
	return c
}

// seed populates the catalog with production-grade architectural patterns.
func (c *Catalog) seed() {
	list := []ArchitecturalPattern{
		{
			ID:       "PAT-MODULAR-MONOLITH",
			Name:     "Modular Monolith",
			Category: CategoryTopology,
			Summary:  "Single deployable binary with strict internal domain package boundaries and shared relational DB",
			Prerequisites: []string{
				"Clean package structure with defined interfaces",
				"Single database instance with logical table schemas",
			},
			Benefits: []string{
				"Zero distributed network latency between modules",
				"ACID transactions across internal modules without distributed consensus",
				"Radically simpler operations, deployment, and local debugging",
			},
			Drawbacks: []string{
				"All modules must be deployed and scaled together",
				"Requires engineering discipline to avoid cyclic dependencies",
			},
			InappropriateWhen: "Independent teams require separate CI/CD pipelines or conflicting technology runtimes",
			MatchedTraits: []model.WorkloadTrait{
				model.TraitTransactionHeavy,
				model.TraitReadHeavy,
				model.TraitConsistencySensitive,
			},
		},
		{
			ID:       "PAT-POSTGRES-SKIP-LOCKED-QUEUE",
			Name:     "PostgreSQL SKIP LOCKED Job Queue",
			Category: CategoryMessaging,
			Summary:  "Durable worker queue using relational database rows with SELECT ... FOR UPDATE SKIP LOCKED",
			Prerequisites: []string{
				"PostgreSQL 9.5+ or MySQL 8.0+",
				"Bounded worker concurrency to prevent database connection saturation",
			},
			Benefits: []string{
				"No extra infrastructure (zero Kafka/RabbitMQ maintenance)",
				"Jobs can be enqueued inside the same ACID transaction as business data",
				"Trivial visibility and debugging via standard SQL queries",
			},
			Drawbacks: []string{
				"Not suitable for > 2,000 job enqueues per second due to WAL write pressure",
				"Requires scheduled vacuuming or cleanup of processed jobs",
			},
			InappropriateWhen: "Sustained throughput exceeds 2,000 jobs/sec or fan-out pub/sub to dozens of consumers is required",
			MatchedTraits: []model.WorkloadTrait{
				model.TraitBackgroundIntensive,
				model.TraitTransactionHeavy,
			},
		},
		{
			ID:       "PAT-TRANSACTIONAL-OUTBOX",
			Name:     "Transactional Outbox Pattern",
			Category: CategoryMessaging,
			Summary:  "Atomically writes business records and outgoing events to the same DB transaction before dispatch",
			Prerequisites: []string{
				"Relational database supporting transactions",
				"Asynchronous background worker polling or tailing the outbox table",
			},
			Benefits: []string{
				"Eliminates dual-write bugs where database succeeds but message broker publish fails",
				"Guarantees at-least-once message delivery to external systems",
			},
			Drawbacks: []string{
				"Downstream consumers must be idempotent to handle duplicate deliveries",
				"Adds slight delay between DB commit and event dispatch",
			},
			InappropriateWhen: "Real-time synchronous response is required without async decoupling",
			MatchedTraits: []model.WorkloadTrait{
				model.TraitTransactionHeavy,
				model.TraitConsistencySensitive,
				model.TraitBackgroundIntensive,
			},
		},
		{
			ID:       "PAT-CACHE-ASIDE",
			Name:     "Cache-Aside (Lazy Loading)",
			Category: CategoryPerformance,
			Summary:  "Application reads from cache; on miss, loads from database and populates cache with TTL",
			Prerequisites: []string{
				"Fast in-memory cache (Redis, Memcached, or Go sync.Map/Ristretto)",
				"Explicit TTL on all cached keys to prevent stale zombie data",
			},
			Benefits: []string{
				"Protects primary database from high read volume",
				"Cache failure degrades gracefully to database queries",
			},
			Drawbacks: []string{
				"Risk of cache penetration and thundering herd on popular keys without single-flight protection",
				"Potential temporary eventual consistency until TTL expires",
			},
			InappropriateWhen: "Data is write-heavy (Write > Read) or requires strict linearizable consistency",
			MatchedTraits: []model.WorkloadTrait{
				model.TraitReadHeavy,
				model.TraitLatencySensitive,
			},
		},
		{
			ID:       "PAT-OPTIMISTIC-LOCKING",
			Name:     "Optimistic Concurrency Control",
			Category: CategoryData,
			Summary:  "Validates row version (UPDATE ... WHERE id = $1 AND version = $2) to detect concurrent modifications",
			Prerequisites: []string{
				"Version or updated_at column on target entities",
				"Application retry loop on conflict error",
			},
			Benefits: []string{
				"Zero lock contention on the database engine during reads",
				"High throughput when concurrent updates to the same row are rare",
			},
			Drawbacks: []string{
				"Under high write contention, excessive retry loops waste CPU and degrade latency",
			},
			InappropriateWhen: "High contention scenarios (e.g. flash sales selling 1 item to 1,000 concurrent buyers)",
			MatchedTraits: []model.WorkloadTrait{
				model.TraitConsistencySensitive,
				model.TraitReadHeavy,
			},
		},
		{
			ID:       "PAT-PESSIMISTIC-LOCKING",
			Name:     "Pessimistic Concurrency Control (SELECT FOR UPDATE)",
			Category: CategoryData,
			Summary:  "Acquires exclusive database row locks during transaction to guarantee serialized updates",
			Prerequisites: []string{
				"Short database transactions to avoid holding row locks",
				"Deterministic lock ordering across queries to prevent deadlocks",
			},
			Benefits: []string{
				"Zero retry loops: requests queue deterministically on the row lock",
				"Guarantees no inventory overselling or double-spending under burst contention",
			},
			Drawbacks: []string{
				"Can cause database lock wait timeouts if transactions hold locks too long",
			},
			InappropriateWhen: "Low contention workloads or distributed microservices without central DB",
			MatchedTraits: []model.WorkloadTrait{
				model.TraitTransactionHeavy,
				model.TraitHighContention,
				model.TraitBurstyTraffic,
			},
		},
	}

	for _, p := range list {
		c.patterns[p.ID] = p
	}
}

// Get returns a pattern by ID.
func (c *Catalog) Get(id string) (*ArchitecturalPattern, bool) {
	p, ok := c.patterns[id]
	if !ok {
		return nil, false
	}
	return &p, true
}

// FindByCategory returns all patterns within a specific category.
func (c *Catalog) FindByCategory(cat PatternCategory) []ArchitecturalPattern {
	var results []ArchitecturalPattern
	for _, p := range c.patterns {
		if p.Category == cat {
			results = append(results, p)
		}
	}
	return results
}

// MatchTraits returns patterns that align with any of the active workload traits.
func (c *Catalog) MatchTraits(traits []model.WorkloadTrait) []ArchitecturalPattern {
	traitSet := make(map[model.WorkloadTrait]bool)
	for _, t := range traits {
		traitSet[t] = true
	}

	var results []ArchitecturalPattern
	for _, p := range c.patterns {
		matched := false
		for _, pt := range p.MatchedTraits {
			if traitSet[pt] {
				matched = true
				break
			}
		}
		if matched {
			results = append(results, p)
		}
	}
	return results
}

// All returns all patterns in the catalog.
func (c *Catalog) All() []ArchitecturalPattern {
	var list []ArchitecturalPattern
	for _, p := range c.patterns {
		list = append(list, p)
	}
	return list
}
